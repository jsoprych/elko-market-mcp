package extract

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jsoprych/elko-market-mcp/internal/channel"
)

// Common World Bank indicator codes with labels.
var wbIndicators = map[string]string{
	"NY.GDP.MKTP.CD":    "GDP (current US$)",
	"NY.GDP.PCAP.CD":    "GDP per capita (current US$)",
	"NY.GDP.MKTP.KD.ZG": "GDP growth (annual %)",
	"FP.CPI.TOTL.ZG":   "Inflation, consumer prices (annual %)",
	"SL.UEM.TOTL.ZS":   "Unemployment, total (% of labor force)",
	"NE.TRD.GNFS.ZS":   "Trade (% of GDP)",
	"GC.DOD.TOTL.GD.ZS": "Central government debt (% of GDP)",
	"SP.POP.TOTL":       "Population, total",
}

// RegisterWorldBank registers the World Bank indicator extractor.
func RegisterWorldBank(r *channel.Runner) {
	r.RegisterExtractor("worldbank_indicator", extractWorldBankIndicator)
}

type indicatorArgs struct {
	Country   string `json:"country"`
	Indicator string `json:"indicator"`
	FromYear  int    `json:"from_year"`
	ToYear    int    `json:"to_year"`
}

func extractWorldBankIndicator(ctx context.Context, args json.RawMessage, ch *channel.Channel) (string, error) {
	var a indicatorArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return "", err
	}
	if a.Country == "" || a.Indicator == "" {
		return "", fmt.Errorf("country and indicator are required")
	}
	now := time.Now()
	if a.FromYear == 0 {
		a.FromYear = now.Year() - 10
	}
	if a.ToYear == 0 {
		a.ToYear = now.Year()
	}

	apiURL := fmt.Sprintf(
		"https://api.worldbank.org/v2/country/%s/indicator/%s?format=json&date=%d:%d&per_page=100",
		a.Country, a.Indicator, a.FromYear, a.ToYear,
	)

	body, err := ch.Fetch(ctx, apiURL)
	if err != nil {
		return "", err
	}

	// World Bank API returns a 2-element JSON array: [metadata, data]
	var wrapper []json.RawMessage
	if err := json.Unmarshal(body, &wrapper); err != nil {
		return "", fmt.Errorf("parse WB response: %w", err)
	}
	if len(wrapper) < 2 {
		return "Unexpected World Bank API response format.", nil
	}

	var data []struct {
		Date  string   `json:"date"`
		Value *float64 `json:"value"`
		Country struct {
			ID    string `json:"id"`
			Value string `json:"value"`
		} `json:"country"`
		Indicator struct {
			ID    string `json:"id"`
			Value string `json:"value"`
		} `json:"indicator"`
	}
	if err := json.Unmarshal(wrapper[1], &data); err != nil || len(data) == 0 {
		return fmt.Sprintf("No World Bank data for indicator %s, country %s.", a.Indicator, a.Country), nil
	}

	for i, j := 0, len(data)-1; i < j; i, j = i+1, j-1 {
		data[i], data[j] = data[j], data[i]
	}

	label := wbIndicators[a.Indicator]
	if label == "" {
		label = a.Indicator
	}
	countryName := data[0].Country.Value

	var sb strings.Builder
	fmt.Fprintf(&sb, "# World Bank: %s — %s\n\n", label, countryName)
	fmt.Fprintf(&sb, "%-8s  %s\n", "Year", "Value")
	sb.WriteString(strings.Repeat("-", 30) + "\n")
	for _, d := range data {
		if d.Value == nil {
			fmt.Fprintf(&sb, "%-8s  —\n", d.Date)
		} else {
			fmt.Fprintf(&sb, "%-8s  %.4g\n", d.Date, *d.Value)
		}
	}
	fmt.Fprintf(&sb, "\nSource: api.worldbank.org | Indicator: %s\n", a.Indicator)
	return sb.String(), nil
}
