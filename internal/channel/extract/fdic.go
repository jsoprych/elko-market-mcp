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

// fieldCase returns name in the case required by the channel's "field_case" option.
// JSON defines the case; this function enforces it uniformly for request params
// and response field lookups.
func fieldCase(ch *channel.Channel, name string) string {
	if ch.Spec.Options["field_case"] == "upper" {
		return strings.ToUpper(name)
	}
	return strings.ToLower(name)
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

	fc := func(name string) string { return fieldCase(ch, name) }

	fields := strings.Join([]string{
		fc("cert"), fc("name"), fc("city"), fc("stname"), fc("active"), fc("asset"), fc("repdte"),
	}, ",")

	params := url.Values{}
	params.Set("fields", fields)
	params.Set("limit", fmt.Sprint(a.Limit))
	params.Set("sort_by", fc("name"))
	params.Set("sort_order", "ASC")

	if a.Name != "" {
		// Replace spaces with wildcards for multi-word Lucene field queries.
		nameFilter := strings.ReplaceAll(strings.TrimSpace(a.Name), " ", "*")
		params.Set("filters", fmt.Sprintf("%s:%s*", fc("name"), nameFilter))
	}
	if a.State != "" {
		filter := params.Get("filters")
		stFilter := fmt.Sprintf("%s:%s", fc("stname"), strings.ToUpper(a.State))
		if filter != "" {
			params.Set("filters", filter+" AND "+stFilter)
		} else {
			params.Set("filters", stFilter)
		}
	}

	apiURL := ch.Spec.Request.BaseURL + "/institutions?" + params.Encode()
	body, err := ch.Fetch(ctx, apiURL)
	if err != nil {
		return "", err
	}

	var resp struct {
		Data []struct {
			Data map[string]interface{} `json:"data"`
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
		cert := fmt.Sprint(d[fc("cert")])
		name := fmt.Sprint(d[fc("name")])
		city := fmt.Sprint(d[fc("city")])
		state := fmt.Sprint(d[fc("stname")])
		asset := fmt.Sprint(d[fc("asset")])
		active := fmt.Sprint(d[fc("active")])
		if active == "1" {
			active = "Yes"
		} else {
			active = "No"
		}
		location := fmt.Sprintf("%s, %s", city, state)
		fmt.Fprintf(&sb, "%-10s  %-40s  %-20s  %-6s  %s\n", cert, name, location, active, asset)
	}
	fmt.Fprintf(&sb, "\nSource: %s\n", ch.Spec.Request.BaseURL)
	return sb.String(), nil
}

type fdicFinArgs struct {
	Cert json.Number `json:"cert"` // accepts "3009" or 3009 from CLI numeric coercion
	Year int         `json:"year"`
}

func extractFDICBankFinancials(ctx context.Context, args json.RawMessage, ch *channel.Channel) (string, error) {
	var a fdicFinArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return "", err
	}
	cert := a.Cert.String()
	if cert == "" || cert == "0" {
		return "", fmt.Errorf("cert is required")
	}

	fc := func(name string) string { return fieldCase(ch, name) }

	fieldList := []string{
		"cert", "repdte", "asset", "dep", "lnlsnet", "eq", "netinc",
		"roa", "roe", "intinc", "nonii", "nonix", "lnlsdepr",
		"tier1capital", "rbct1tier1", "rbcrwaj",
	}
	fields := make([]string, len(fieldList))
	for i, f := range fieldList {
		fields[i] = fc(f)
	}

	params := url.Values{}
	params.Set("filters", fmt.Sprintf("%s:%s", fc("cert"), cert))
	params.Set("fields", strings.Join(fields, ","))
	params.Set("sort_by", fc("repdte"))
	params.Set("sort_order", "DESC")
	params.Set("limit", "8")

	if a.Year > 0 {
		filter := params.Get("filters")
		params.Set("filters", filter+fmt.Sprintf(" AND %s:[%d0101 TO %d1231]", fc("repdte"), a.Year, a.Year))
	}

	apiURL := ch.Spec.Request.BaseURL + "/financials?" + params.Encode()
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
		return fmt.Sprintf("No FDIC financial data found for cert %s.", cert), nil
	}

	// Labels keyed by canonical lowercase name; fc() applied at lookup time.
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
	order := []string{
		"repdte", "asset", "dep", "lnlsnet", "eq", "netinc",
		"roa", "roe", "intinc", "nonii", "nonix", "tier1capital",
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "# FDIC Financials — Cert %s\n\n", cert)
	for _, row := range resp.Data {
		d := row.Data
		fmt.Fprintf(&sb, "\n## Period: %v\n", d[fc("repdte")])
		for _, k := range order {
			if v, ok := d[fc(k)]; ok {
				fmt.Fprintf(&sb, "  %-28s %v\n", labels[k]+":", v)
			}
		}
	}
	fmt.Fprintf(&sb, "\nSource: %s (values in $M unless noted)\n", ch.Spec.Request.BaseURL)
	return sb.String(), nil
}
