package extract

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jsoprych/elko-market-mcp/internal/channel"
)

var cikRE = regexp.MustCompile(`<cik>(\d+)</cik>`)

// edgarClient holds shared CIK lookup cache across both EDGAR extractors.
type edgarClient struct {
	cikMu sync.RWMutex
	cikM  map[string]string
}

// RegisterEDGAR registers all SEC EDGAR extractors with shared CIK cache state.
func RegisterEDGAR(r *channel.Runner) {
	ec := &edgarClient{cikM: make(map[string]string)}
	r.RegisterExtractor("edgar_xbrl_financials", ec.extractFinancials)
	r.RegisterExtractor("edgar_company_info", ec.extractCompanyInfo)
	r.RegisterExtractor("edgar_insider_trades", ec.extractInsiderTrades)
}

// ── CIK lookup ────────────────────────────────────────────────────────────────

func (ec *edgarClient) lookupCIK(ctx context.Context, ticker string, ch *channel.Channel) (string, error) {
	ticker = strings.ToUpper(ticker)

	ec.cikMu.RLock()
	if cik, ok := ec.cikM[ticker]; ok {
		ec.cikMu.RUnlock()
		return cik, nil
	}
	ec.cikMu.RUnlock()

	u := fmt.Sprintf(
		"https://www.sec.gov/cgi-bin/browse-edgar?action=getcompany&CIK=%s&type=10-K&dateb=&owner=include&count=1&output=atom",
		url.QueryEscape(ticker),
	)
	body, err := ch.Fetch(ctx, u)
	if err != nil {
		return "", fmt.Errorf("CIK lookup for %s: %w", ticker, err)
	}
	m := cikRE.FindSubmatch(body)
	if m == nil {
		return "", fmt.Errorf("CIK not found for ticker %q (not a US-listed company?)", ticker)
	}
	cik := string(m[1])

	ec.cikMu.Lock()
	ec.cikM[ticker] = cik
	ec.cikMu.Unlock()
	return cik, nil
}

// ── XBRL concept fetch ────────────────────────────────────────────────────────

type secEntry struct {
	End  string  `json:"end"`
	Val  float64 `json:"val"`
	Form string  `json:"form"`
	FP   string  `json:"fp"`
	FY   int     `json:"fy"`
}

func secConcept(ctx context.Context, ch *channel.Channel, cik, concept, form string) ([]secEntry, error) {
	u := fmt.Sprintf(
		"https://data.sec.gov/api/xbrl/companyconcept/CIK%s/us-gaap/%s.json",
		cik, concept,
	)
	body, err := ch.Fetch(ctx, u)
	if err != nil {
		return nil, err
	}

	var r struct {
		Units map[string][]secEntry `json:"units"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("parse concept %s: %w", concept, err)
	}

	entries, ok := r.Units["USD"]
	if !ok {
		for _, v := range r.Units {
			entries = v
			break
		}
	}

	seen := map[string]bool{}
	var out []secEntry
	for _, e := range entries {
		if e.Form != form {
			continue
		}
		if form == "10-K" && e.FP != "FY" {
			continue
		}
		key := e.End + fmt.Sprint(e.Val)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].End < out[j].End })
	return out, nil
}

// ── Extractors ────────────────────────────────────────────────────────────────

type finArgs struct {
	Symbol    string `json:"symbol"`
	Statement string `json:"statement"`
	Frequency string `json:"frequency"`
}

func (ec *edgarClient) extractFinancials(ctx context.Context, args json.RawMessage, ch *channel.Channel) (string, error) {
	var a finArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return "", err
	}
	if a.Symbol == "" {
		return "", fmt.Errorf("symbol is required")
	}
	a.Symbol = strings.ToUpper(a.Symbol)
	if a.Statement == "" {
		a.Statement = "income"
	}
	if a.Frequency == "" {
		a.Frequency = "annual"
	}

	form := "10-K"
	if a.Frequency == "quarterly" {
		form = "10-Q"
	}

	cik, err := ec.lookupCIK(ctx, a.Symbol, ch)
	if err != nil {
		return "", err
	}

	type conceptDef struct {
		label   string
		tags    []string
		divisor float64
		suffix  string
	}
	type stmtDef struct {
		title    string
		concepts []conceptDef
	}

	stmts := map[string]stmtDef{
		"income": {
			title: "Income Statement",
			concepts: []conceptDef{
				{label: "Revenue", tags: []string{
					"RevenueFromContractWithCustomerExcludingAssessedTax",
					"Revenues", "SalesRevenueNet",
					"RevenueFromContractWithCustomerIncludingAssessedTax",
				}, divisor: 1e9, suffix: "B"},
				{label: "Gross Profit", tags: []string{"GrossProfit"}, divisor: 1e9, suffix: "B"},
				{label: "Operating Income", tags: []string{"OperatingIncomeLoss"}, divisor: 1e9, suffix: "B"},
				{label: "Net Income", tags: []string{"NetIncomeLoss"}, divisor: 1e9, suffix: "B"},
				{label: "EPS (Basic)", tags: []string{"EarningsPerShareBasic"}, divisor: 1, suffix: ""},
				{label: "EPS (Diluted)", tags: []string{"EarningsPerShareDiluted"}, divisor: 1, suffix: ""},
			},
		},
		"balance": {
			title: "Balance Sheet",
			concepts: []conceptDef{
				{label: "Total Assets", tags: []string{"Assets"}, divisor: 1e9, suffix: "B"},
				{label: "Current Assets", tags: []string{"AssetsCurrent"}, divisor: 1e9, suffix: "B"},
				{label: "Cash & Equivalents", tags: []string{
					"CashAndCashEquivalentsAtCarryingValue",
					"CashCashEquivalentsAndShortTermInvestments",
				}, divisor: 1e9, suffix: "B"},
				{label: "Total Liabilities", tags: []string{"Liabilities"}, divisor: 1e9, suffix: "B"},
				{label: "Current Liabilities", tags: []string{"LiabilitiesCurrent"}, divisor: 1e9, suffix: "B"},
				{label: "Long-Term Debt", tags: []string{
					"LongTermDebt", "LongTermDebtNoncurrent",
				}, divisor: 1e9, suffix: "B"},
				{label: "Stockholders' Equity", tags: []string{
					"StockholdersEquity", "StockholdersEquityAttributableToParent",
				}, divisor: 1e9, suffix: "B"},
			},
		},
		"cashflow": {
			title: "Cash Flow Statement",
			concepts: []conceptDef{
				{label: "Operating Cash Flow", tags: []string{"NetCashProvidedByUsedInOperatingActivities"}, divisor: 1e9, suffix: "B"},
				{label: "Investing Cash Flow", tags: []string{"NetCashProvidedByUsedInInvestingActivities"}, divisor: 1e9, suffix: "B"},
				{label: "Financing Cash Flow", tags: []string{"NetCashProvidedByUsedInFinancingActivities"}, divisor: 1e9, suffix: "B"},
				{label: "CapEx", tags: []string{
					"PaymentsToAcquirePropertyPlantAndEquipment",
					"CapitalExpendituresIncurredButNotYetPaid",
				}, divisor: 1e9, suffix: "B"},
			},
		},
	}

	def, ok := stmts[a.Statement]
	if !ok {
		return "", fmt.Errorf("unknown statement %q — use: income, balance, cashflow", a.Statement)
	}

	type row struct {
		label  string
		byDate map[string]float64
		div    float64
		suffix string
	}
	allDates := map[string]bool{}
	rows := make([]row, 0, len(def.concepts))

	for _, c := range def.concepts {
		r := row{label: c.label, byDate: map[string]float64{}, div: c.divisor, suffix: c.suffix}
		for _, tag := range c.tags {
			entries, err := secConcept(ctx, ch, cik, tag, form)
			if err != nil || len(entries) == 0 {
				continue
			}
			if len(entries) > 8 {
				entries = entries[len(entries)-8:]
			}
			for _, e := range entries {
				r.byDate[e.End] = e.Val
				allDates[e.End] = true
			}
			break
		}
		rows = append(rows, r)
	}

	dates := make([]string, 0, len(allDates))
	for d := range allDates {
		dates = append(dates, d)
	}
	sort.Strings(dates)
	if len(dates) > 5 {
		dates = dates[len(dates)-5:]
	}

	if len(dates) == 0 {
		return fmt.Sprintf("No SEC EDGAR XBRL data for %s. Company may not be US-listed or may not file XBRL.", a.Symbol), nil
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "# %s %s (%s) — SEC EDGAR XBRL\n\n", a.Symbol, def.title, a.Frequency)
	fmt.Fprintf(&sb, "%-30s", "")
	for _, d := range dates {
		fmt.Fprintf(&sb, "  %12s", d)
	}
	sb.WriteByte('\n')
	sb.WriteString(strings.Repeat("-", 30+14*len(dates)) + "\n")

	for _, r := range rows {
		if len(r.byDate) == 0 {
			continue
		}
		fmt.Fprintf(&sb, "%-30s", r.label)
		for _, d := range dates {
			v, ok := r.byDate[d]
			if !ok {
				fmt.Fprintf(&sb, "  %12s", "—")
				continue
			}
			if r.div == 1 {
				fmt.Fprintf(&sb, "  %12.2f", v)
			} else {
				fmt.Fprintf(&sb, "  %11.2f%s", v/r.div, r.suffix)
			}
		}
		sb.WriteByte('\n')
	}
	sb.WriteString("\nValues in USD billions (B) unless noted. Source: SEC EDGAR XBRL.\n")
	return sb.String(), nil
}

type infoArgs struct {
	Symbol string `json:"symbol"`
}

func (ec *edgarClient) extractCompanyInfo(ctx context.Context, args json.RawMessage, ch *channel.Channel) (string, error) {
	var a infoArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return "", err
	}
	if a.Symbol == "" {
		return "", fmt.Errorf("symbol is required")
	}
	a.Symbol = strings.ToUpper(a.Symbol)

	cik, err := ec.lookupCIK(ctx, a.Symbol, ch)
	if err != nil {
		return "", err
	}

	submURL := fmt.Sprintf("https://data.sec.gov/submissions/CIK%s.json", cik)
	body, err := ch.Fetch(ctx, submURL)
	if err != nil {
		return "", err
	}

	var s struct {
		Name                 string `json:"name"`
		SIC                  string `json:"sic"`
		SICDescription       string `json:"sicDescription"`
		StateOfIncorporation string `json:"stateOfIncorporation"`
		FiscalYearEnd        string `json:"fiscalYearEnd"`
		Category             string `json:"category"`
	}
	if err := json.Unmarshal(body, &s); err != nil {
		return "", fmt.Errorf("parse submissions: %w", err)
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "# %s — SEC EDGAR Company Info\n\n", a.Symbol)
	fmt.Fprintf(&sb, "%-28s %s\n", "Name:", s.Name)
	fmt.Fprintf(&sb, "%-28s %s\n", "CIK:", cik)
	if s.SICDescription != "" {
		fmt.Fprintf(&sb, "%-28s %s (SIC %s)\n", "Industry:", s.SICDescription, s.SIC)
	}
	if s.StateOfIncorporation != "" {
		fmt.Fprintf(&sb, "%-28s %s\n", "Incorporated:", s.StateOfIncorporation)
	}
	if len(s.FiscalYearEnd) == 4 {
		months := []string{"", "Jan", "Feb", "Mar", "Apr", "May", "Jun",
			"Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}
		mo := int(s.FiscalYearEnd[0]-'0')*10 + int(s.FiscalYearEnd[1]-'0')
		day := s.FiscalYearEnd[2:]
		if mo >= 1 && mo <= 12 {
			fmt.Fprintf(&sb, "%-28s %s %s\n", "Fiscal Year End:", months[mo], day)
		}
	}
	if s.Category != "" {
		fmt.Fprintf(&sb, "%-28s %s\n", "Filer Category:", s.Category)
	}
	return sb.String(), nil
}

// ── Form 4 insider trades ──────────────────────────────────────────────────────

// padCIK zero-pads a CIK to 10 digits as required by data.sec.gov/submissions.
func padCIK(cik string) string {
	for len(cik) < 10 {
		cik = "0" + cik
	}
	return cik
}

// form4Doc models the subset of Form 4 XML we care about.
type form4Doc struct {
	Issuer struct {
		Name   string `xml:"issuerName"`
		Symbol string `xml:"issuerTradingSymbol"`
	} `xml:"issuer"`
	Owner struct {
		ID struct {
			Name string `xml:"rptOwnerName"`
		} `xml:"reportingOwnerId"`
		Rel struct {
			IsDirector string `xml:"isDirector"`
			IsOfficer  string `xml:"isOfficer"`
			Is10Pct    string `xml:"isTenPercentOwner"`
			Title      string `xml:"officerTitle"`
		} `xml:"reportingOwnerRelationship"`
	} `xml:"reportingOwner"`
	NonDerivatives struct {
		Txns []struct {
			Date    string `xml:"transactionDate>value"`
			Code    string `xml:"transactionCoding>transactionCode"`
			Shares  string `xml:"transactionAmounts>transactionShares>value"`
			Price   string `xml:"transactionAmounts>transactionPricePerShare>value"`
			AcqDisp string `xml:"transactionAmounts>transactionAcquiredDisposedCode>value"`
			After   string `xml:"postTransactionAmounts>sharesOwnedFollowingTransaction>value"`
		} `xml:"nonDerivativeTransaction"`
	} `xml:"nonDerivativeTable"`
}

type insiderTrade struct {
	Date    string
	Name    string
	Role    string
	Code    string
	Label   string
	Shares  float64
	Price   float64
	Value   float64
	After   float64
}

var txnCodeLabel = map[string]string{
	"P": "Buy",
	"S": "Sell",
	"A": "Award",
	"M": "Opt.Exercise",
	"F": "Tax Withhold",
	"G": "Gift",
	"D": "Return",
	"J": "Other",
}

type insiderArgs struct {
	Symbol string `json:"symbol"`
	Months int    `json:"months"`
	Types  string `json:"types"`
	Limit  int    `json:"limit"`
}

func (ec *edgarClient) extractInsiderTrades(ctx context.Context, args json.RawMessage, ch *channel.Channel) (string, error) {
	var a insiderArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return "", err
	}
	if a.Symbol == "" {
		return "", fmt.Errorf("symbol is required")
	}
	a.Symbol = strings.ToUpper(a.Symbol)
	if a.Months <= 0 {
		a.Months = 12
	}
	if a.Months > 36 {
		a.Months = 36
	}
	if a.Limit <= 0 {
		a.Limit = 30
	}
	if a.Limit > 100 {
		a.Limit = 100
	}
	tradesOnly := a.Types != "all"

	cik, err := ec.lookupCIK(ctx, a.Symbol, ch)
	if err != nil {
		return "", err
	}

	// Fetch company name from submissions (one call, also confirms CIK is valid).
	submURL := fmt.Sprintf("https://data.sec.gov/submissions/CIK%s.json", padCIK(cik))
	submBody, err := ch.Fetch(ctx, submURL)
	if err != nil {
		return "", fmt.Errorf("fetch submissions: %w", err)
	}
	var subm struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(submBody, &subm); err != nil {
		return "", fmt.Errorf("parse submissions: %w", err)
	}

	// Use EDGAR EFTS full-text search to find Form 4 filings where this
	// company's CIK appears as the issuer. Form 4s are filed under the
	// insider's own CIK, not the company's, so the submissions endpoint
	// doesn't list them. EFTS finds all docs containing the padded CIK.
	cutoff := time.Now().AddDate(0, -a.Months, 0).Format("2006-01-02")
	eftsURL := fmt.Sprintf(
		"https://efts.sec.gov/LATEST/search-index?q=%%22%s%%22&forms=4&dateRange=custom&startdt=%s&size=50",
		padCIK(cik), cutoff,
	)
	eftsBody, err := ch.Fetch(ctx, eftsURL)
	if err != nil {
		return "", fmt.Errorf("EDGAR search: %w", err)
	}

	var efts struct {
		Hits struct {
			Hits []struct {
				ID     string `json:"_id"` // "adsh:filename.xml"
				Source struct {
					FileDate string `json:"file_date"`
				} `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := json.Unmarshal(eftsBody, &efts); err != nil {
		return "", fmt.Errorf("parse EDGAR search: %w", err)
	}

	// Build filing list: split _id into accession + document filename.
	type filing struct {
		accession string // with dashes: "0000320193-24-000126"
		doc       string // filename: "wk-form4_xxx.xml"
		date      string
	}
	var filings []filing
	for _, h := range efts.Hits.Hits {
		parts := strings.SplitN(h.ID, ":", 2)
		if len(parts) != 2 {
			continue
		}
		filings = append(filings, filing{
			accession: parts[0],
			doc:       parts[1],
			date:      h.Source.FileDate,
		})
	}

	if len(filings) == 0 {
		return fmt.Sprintf("No Form 4 filings found for %s in the last %d months.", a.Symbol, a.Months), nil
	}

	// Fetch and parse each Form 4 XML.
	var trades []insiderTrade
	for _, f := range filings {
		accNoDash := strings.ReplaceAll(f.accession, "-", "")
		// Archive path uses the filer's CIK (first 10 digits of accession number,
		// stripped of leading zeros) — not necessarily the issuer's CIK.
		filerCIK := strings.TrimLeft(strings.Split(f.accession, "-")[0], "0")
		if filerCIK == "" {
			filerCIK = "0"
		}
		xmlURL := fmt.Sprintf(
			"https://www.sec.gov/Archives/edgar/data/%s/%s/%s",
			filerCIK, accNoDash, f.doc,
		)
		xmlBody, err := ch.Fetch(ctx, xmlURL)
		if err != nil {
			continue // skip unparseable filings
		}
		var doc form4Doc
		if err := xml.Unmarshal(xmlBody, &doc); err != nil {
			continue
		}

		// Determine insider role label.
		rel := doc.Owner.Rel
		role := rel.Title
		if role == "" {
			switch {
			case rel.IsDirector == "1":
				role = "Director"
			case rel.Is10Pct == "1":
				role = ">10% Owner"
			default:
				role = "Insider"
			}
		}
		name := doc.Owner.ID.Name

		for _, txn := range doc.NonDerivatives.Txns {
			code := strings.ToUpper(strings.TrimSpace(txn.Code))
			if tradesOnly && code != "P" && code != "S" {
				continue
			}
			label, ok := txnCodeLabel[code]
			if !ok {
				label = code
			}
			// Skip transactions older than the cutoff window.
			if txn.Date != "" && txn.Date < cutoff {
				continue
			}
			shares, _ := strconv.ParseFloat(txn.Shares, 64)
			price, _ := strconv.ParseFloat(txn.Price, 64)
			after, _ := strconv.ParseFloat(txn.After, 64)
			if shares == 0 {
				continue
			}
			trades = append(trades, insiderTrade{
				Date:   txn.Date,
				Name:   name,
				Role:   role,
				Code:   code,
				Label:  label,
				Shares: shares,
				Price:  price,
				Value:  shares * price,
				After:  after,
			})
		}
	}

	if len(trades) == 0 {
		typeDesc := "open-market trades"
		if !tradesOnly {
			typeDesc = "transactions"
		}
		return fmt.Sprintf("No insider %s found for %s in the last %d months.", typeDesc, a.Symbol, a.Months), nil
	}

	// Sort newest first, cap at limit.
	sort.Slice(trades, func(i, j int) bool { return trades[i].Date > trades[j].Date })
	if len(trades) > a.Limit {
		trades = trades[:a.Limit]
	}

	// Build summary counts.
	var buyShares, sellShares float64
	buyCount, sellCount := 0, 0
	for _, t := range trades {
		if t.Code == "P" {
			buyShares += t.Shares
			buyCount++
		} else if t.Code == "S" {
			sellShares += t.Shares
			sellCount++
		}
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "# %s (%s) — Insider Transactions (%d months)\n\n",
		subm.Name, a.Symbol, a.Months)

	fmt.Fprintf(&sb, "%-12s  %-28s  %-18s  %-13s  %-10s  %-10s  %-14s  %s\n",
		"Date", "Insider", "Role", "Type",
		"Shares", "Price", "Value", "Owned After")
	sb.WriteString(strings.Repeat("-", 120) + "\n")

	for _, t := range trades {
		role := t.Role
		if len(role) > 18 {
			role = role[:15] + "..."
		}
		name := t.Name
		if len(name) > 28 {
			name = name[:25] + "..."
		}
		priceStr := "—"
		if t.Price > 0 {
			priceStr = fmt.Sprintf("$%.2f", t.Price)
		}
		valueStr := "—"
		if t.Value > 0 {
			valueStr = formatLarge(t.Value)
		}
		afterStr := "—"
		if t.After > 0 {
			afterStr = formatLarge(t.After)
		}
		fmt.Fprintf(&sb, "%-12s  %-28s  %-18s  %-13s  %10s  %10s  %14s  %s\n",
			t.Date, name, role, t.Label,
			formatLarge(t.Shares), priceStr, valueStr, afterStr)
	}

	sb.WriteByte('\n')
	if buyCount > 0 || sellCount > 0 {
		fmt.Fprintf(&sb, "Summary (%d months): ", a.Months)
		if buyCount > 0 {
			fmt.Fprintf(&sb, "%d buy(s) +%s shares", buyCount, formatLarge(buyShares))
		}
		if buyCount > 0 && sellCount > 0 {
			sb.WriteString("  |  ")
		}
		if sellCount > 0 {
			fmt.Fprintf(&sb, "%d sell(s) -%s shares", sellCount, formatLarge(sellShares))
		}
		net := buyShares - sellShares
		sign := "+"
		if net < 0 {
			sign = ""
		}
		fmt.Fprintf(&sb, "  |  Net: %s%s shares\n", sign, formatLarge(net))
	}
	sb.WriteString("Source: SEC EDGAR Form 4 (data.sec.gov)\n")
	return sb.String(), nil
}
