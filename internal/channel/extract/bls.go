package extract

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jsoprych/elko-market-mcp/internal/channel"
)

// Well-known BLS series IDs with labels.
var blsWellKnown = map[string]string{
	"CUUR0000SA0":   "CPI-U All Items (NSA)",
	"CUSR0000SA0":   "CPI-U All Items (SA)",
	"LNS14000000":   "Unemployment Rate (SA)",
	"CES0000000001": "Total Nonfarm Payrolls",
	"WPUFD49104":    "PPI Final Demand",
}

// RegisterBLS registers the Bureau of Labor Statistics series extractor.
func RegisterBLS(r *channel.Runner) {
	r.RegisterExtractor("bls_series", extractBLSSeries)
}

type seriesArgs struct {
	SeriesID  string `json:"series_id"`
	StartYear string `json:"start_year"`
	EndYear   string `json:"end_year"`
}

func extractBLSSeries(ctx context.Context, args json.RawMessage, ch *channel.Channel) (string, error) {
	var a seriesArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return "", err
	}
	if a.SeriesID == "" {
		return "", fmt.Errorf("series_id is required")
	}
	now := time.Now()
	if a.StartYear == "" {
		a.StartYear = fmt.Sprintf("%d", now.Year()-5)
	}
	if a.EndYear == "" {
		a.EndYear = fmt.Sprintf("%d", now.Year())
	}

	apiURL := fmt.Sprintf(
		"https://api.bls.gov/publicAPI/v1/timeseries/data/%s?startyear=%s&endyear=%s",
		a.SeriesID, a.StartYear, a.EndYear,
	)

	body, err := ch.Fetch(ctx, apiURL)
	if err != nil {
		return "", err
	}

	var resp struct {
		Status  string   `json:"status"`
		Message []string `json:"message"`
		Results struct {
			Series []struct {
				SeriesID string `json:"seriesID"`
				Data     []struct {
					Year   string `json:"year"`
					Period string `json:"period"`
					Value  string `json:"value"`
				} `json:"data"`
			} `json:"series"`
		} `json:"Results"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("parse BLS response: %w", err)
	}
	if resp.Status != "REQUEST_SUCCEEDED" {
		msg := strings.Join(resp.Message, "; ")
		return "", fmt.Errorf("BLS API error: %s", msg)
	}
	if len(resp.Results.Series) == 0 || len(resp.Results.Series[0].Data) == 0 {
		return fmt.Sprintf("No BLS data found for series %s.", a.SeriesID), nil
	}

	label := blsWellKnown[a.SeriesID]
	if label == "" {
		label = a.SeriesID
	}

	series := resp.Results.Series[0]
	data := series.Data
	for i, j := 0, len(data)-1; i < j; i, j = i+1, j-1 {
		data[i], data[j] = data[j], data[i]
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "# BLS: %s\n\n", label)
	fmt.Fprintf(&sb, "%-8s  %-8s  %s\n", "Year", "Period", "Value")
	sb.WriteString(strings.Repeat("-", 30) + "\n")
	for _, d := range data {
		fmt.Fprintf(&sb, "%-8s  %-8s  %s\n", d.Year, d.Period, d.Value)
	}
	sb.WriteString("\nSource: api.bls.gov (v1, unauthenticated)\n")
	return sb.String(), nil
}
