package extract

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"

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
