package extract

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jsoprych/elko-market-mcp/internal/channel"
)

// RegisterTreasury registers the US Treasury yields extractor.
func RegisterTreasury(r *channel.Runner) {
	r.RegisterExtractor("treasury_yields", extractTreasuryYields)
}

type yieldsArgs struct {
	From   string `json:"from"`
	To     string `json:"to"`
	Latest bool   `json:"latest"`
}

func extractTreasuryYields(ctx context.Context, args json.RawMessage, ch *channel.Channel) (string, error) {
	var a yieldsArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return "", err
	}

	if a.From == "" {
		a.From = time.Now().AddDate(0, 0, -30).Format("2006-01-02")
	}
	if a.To == "" {
		a.To = time.Now().Format("2006-01-02")
	}

	curveURL := fmt.Sprintf(
		"https://api.fiscaldata.treasury.gov/services/api/v1/accounting/od/avg_interest_rates?filter=record_date:gte:%s,record_date:lte:%s&sort=-record_date&page[size]=30&fields=record_date,security_desc,avg_interest_rate_amt",
		a.From, a.To,
	)

	body, err := ch.Fetch(ctx, curveURL)
	if err != nil {
		return "", err
	}

	var resp struct {
		Data []map[string]string `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("parse treasury response: %w", err)
	}

	if len(resp.Data) == 0 {
		return "No Treasury yield data available for the requested period.", nil
	}

	type rateRow struct {
		date string
		desc string
		rate string
	}
	var rows []rateRow
	for _, d := range resp.Data {
		rows = append(rows, rateRow{
			date: d["record_date"],
			desc: d["security_desc"],
			rate: d["avg_interest_rate_amt"],
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].date != rows[j].date {
			return rows[i].date > rows[j].date
		}
		return rows[i].desc < rows[j].desc
	})

	if a.Latest && len(rows) > 0 {
		latestDate := rows[0].date
		var filtered []rateRow
		for _, r := range rows {
			if r.date == latestDate {
				filtered = append(filtered, r)
			}
		}
		rows = filtered
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "# US Treasury Average Interest Rates\n\n")
	fmt.Fprintf(&sb, "%-12s  %-40s  %s\n", "Date", "Security", "Rate (%)")
	sb.WriteString(strings.Repeat("-", 62) + "\n")
	for _, r := range rows {
		fmt.Fprintf(&sb, "%-12s  %-40s  %s\n", r.date, r.desc, r.rate)
	}
	sb.WriteString("\nSource: fiscaldata.treasury.gov\n")
	return sb.String(), nil
}
