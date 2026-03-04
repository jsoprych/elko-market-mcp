package extract

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/jsoprych/elko-market-mcp/internal/channel"
)

// RegisterFDIC registers all FDIC bank data extractors.
func RegisterFDIC(r *channel.Runner) {
	r.RegisterExtractor("fdic_bank_search", extractFDICBankSearch)
	r.RegisterExtractor("fdic_bank_financials", extractFDICBankFinancials)
}

type fdicSearchArgs struct {
	Name  string `json:"name"`
	State string `json:"state"`
	Limit int    `json:"limit"`
}

func extractFDICBankSearch(ctx context.Context, args json.RawMessage, ch *channel.Channel) (string, error) {
	var a fdicSearchArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return "", err
	}
	if a.Limit <= 0 || a.Limit > 50 {
		a.Limit = 10
	}

	params := url.Values{}
	params.Set("fields", "cert,name,city,stname,active,asset,repdte")
	params.Set("limit", fmt.Sprint(a.Limit))
	params.Set("sort_by", "asset")
	params.Set("sort_order", "DESC")
	if a.Name != "" {
		params.Set("filters", fmt.Sprintf("name:%s*", url.QueryEscape(a.Name)))
	}
	if a.State != "" {
		filter := params.Get("filters")
		stFilter := fmt.Sprintf("stname:%s", strings.ToUpper(a.State))
		if filter != "" {
			params.Set("filters", filter+" AND "+stFilter)
		} else {
			params.Set("filters", stFilter)
		}
	}

	apiURL := "https://banks.data.fdic.gov/api/institutions?" + params.Encode()
	body, err := ch.Fetch(ctx, apiURL)
	if err != nil {
		return "", err
	}

	var resp struct {
		Data []struct {
			Data struct {
				Cert   interface{} `json:"cert"`
				Name   string      `json:"name"`
				City   string      `json:"city"`
				State  string      `json:"stname"`
				Active interface{} `json:"active"`
				Asset  interface{} `json:"asset"`
				RepDte string      `json:"repdte"`
			} `json:"data"`
		} `json:"data"`
		Meta struct {
			Total int `json:"total"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("parse FDIC response: %w", err)
	}

	if len(resp.Data) == 0 {
		return "No FDIC institutions found matching your query.", nil
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "# FDIC Bank Search Results (%d total)\n\n", resp.Meta.Total)
	fmt.Fprintf(&sb, "%-10s  %-40s  %-20s  %-6s  %s\n", "Cert", "Name", "City, State", "Active", "Assets ($M)")
	sb.WriteString(strings.Repeat("-", 90) + "\n")
	for _, row := range resp.Data {
		d := row.Data
		location := fmt.Sprintf("%s, %s", d.City, d.State)
		active := fmt.Sprint(d.Active)
		if active == "1" {
			active = "Yes"
		} else {
			active = "No"
		}
		fmt.Fprintf(&sb, "%-10v  %-40s  %-20s  %-6s  %v\n",
			d.Cert, d.Name, location, active, d.Asset)
	}
	sb.WriteString("\nSource: banks.data.fdic.gov\n")
	return sb.String(), nil
}

type fdicFinArgs struct {
	Cert string `json:"cert"`
	Year int    `json:"year"`
}

func extractFDICBankFinancials(ctx context.Context, args json.RawMessage, ch *channel.Channel) (string, error) {
	var a fdicFinArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return "", err
	}
	if a.Cert == "" {
		return "", fmt.Errorf("cert is required")
	}

	fields := strings.Join([]string{
		"cert", "repdte", "asset", "dep", "lnlsnet", "eq", "netinc",
		"roa", "roe", "intinc", "nonii", "nonix", "lnlsdepr",
		"tier1capital", "rbct1tier1", "rbcrwaj",
	}, ",")

	params := url.Values{}
	params.Set("filters", fmt.Sprintf("cert:%s", a.Cert))
	params.Set("fields", fields)
	params.Set("sort_by", "repdte")
	params.Set("sort_order", "DESC")
	params.Set("limit", "8")

	if a.Year > 0 {
		filter := params.Get("filters")
		params.Set("filters", filter+fmt.Sprintf(" AND repdte:[%d0101 TO %d1231]", a.Year, a.Year))
	}

	apiURL := "https://banks.data.fdic.gov/api/financials?" + params.Encode()
	body, err := ch.Fetch(ctx, apiURL)
	if err != nil {
		return "", err
	}

	var resp struct {
		Data []struct {
			Data map[string]interface{} `json:"data"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("parse FDIC financials: %w", err)
	}

	if len(resp.Data) == 0 {
		return fmt.Sprintf("No FDIC financial data found for cert %s.", a.Cert), nil
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "# FDIC Financials — Cert %s\n\n", a.Cert)

	labels := map[string]string{
		"repdte":      "Report Date",
		"asset":       "Total Assets ($M)",
		"dep":         "Total Deposits ($M)",
		"lnlsnet":     "Net Loans ($M)",
		"eq":          "Equity ($M)",
		"netinc":      "Net Income ($M)",
		"roa":         "ROA (%)",
		"roe":         "ROE (%)",
		"intinc":      "Interest Income ($M)",
		"nonii":       "Non-Interest Income ($M)",
		"nonix":       "Non-Interest Expense ($M)",
		"tier1capital": "Tier 1 Capital ($M)",
	}
	order := []string{"repdte", "asset", "dep", "lnlsnet", "eq", "netinc",
		"roa", "roe", "intinc", "nonii", "nonix", "tier1capital"}

	for _, row := range resp.Data {
		d := row.Data
		repdte := fmt.Sprint(d["repdte"])
		fmt.Fprintf(&sb, "\n## Period: %s\n", repdte)
		for _, k := range order {
			if v, ok := d[k]; ok {
				label := labels[k]
				fmt.Fprintf(&sb, "  %-28s %v\n", label+":", v)
			}
		}
	}
	sb.WriteString("\nSource: banks.data.fdic.gov (values in $M unless noted)\n")
	return sb.String(), nil
}
