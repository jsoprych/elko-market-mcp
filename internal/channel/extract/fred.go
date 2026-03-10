package extract

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jsoprych/elko-market-mcp/internal/channel"
)

// RegisterFRED registers Federal Reserve Economic Data extractors.
func RegisterFRED(r *channel.Runner) {
	r.RegisterExtractor("fred_series", extractFREDSeries)
	r.RegisterExtractor("fred_search", extractFREDSearch)
}

// fredAPIKey returns the FRED API key from the environment, or empty string
// for anonymous access (rate-limited to 120 req/min).
func fredAPIKey() string {
	return os.Getenv("FRED_API_KEY")
}

// fredURL builds a FRED API URL, appending api_key when available.
func fredURL(base, path string, params map[string]string) string {
	u := base + path + "?file_type=json"
	for k, v := range params {
		u += "&" + k + "=" + v
	}
	if key := fredAPIKey(); key != "" {
		u += "&api_key=" + key
	}
	return u
}

// ── fred_series ───────────────────────────────────────────────────────────────

type fredSeriesArgs struct {
	SeriesID  string `json:"series_id"`
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
	Limit     int    `json:"limit"`
}

type fredObsResp struct {
	SeriesID         string `json:"series_id"`
	Title            string `json:"title"`
	Units            string `json:"units"`
	Frequency        string `json:"frequency"`
	ObservationStart string `json:"observation_start"`
	ObservationEnd   string `json:"observation_end"`
	Observations     []struct {
		Date  string `json:"date"`
		Value string `json:"value"`
	} `json:"observations"`
}

func extractFREDSeries(ctx context.Context, args json.RawMessage, ch *channel.Channel) (string, error) {
	var a fredSeriesArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return "", err
	}
	if fredAPIKey() == "" {
		return "", fmt.Errorf("FRED_API_KEY is not set — register for a free key at https://fred.stlouisfed.org/docs/api/api_key.html")
	}
	if a.SeriesID == "" {
		return "", fmt.Errorf("series_id is required — use fred_search to find series IDs")
	}
	a.SeriesID = strings.ToUpper(strings.TrimSpace(a.SeriesID))

	now := time.Now()
	if a.StartDate == "" {
		a.StartDate = now.AddDate(-5, 0, 0).Format("2006-01-02")
	}
	if a.EndDate == "" {
		a.EndDate = now.Format("2006-01-02")
	}
	limit := a.Limit
	if limit <= 0 {
		limit = 120
	}
	if limit > 1000 {
		limit = 1000
	}

	// Fetch series metadata and observations in parallel via two URLs; for
	// simplicity use a single observations endpoint (it includes units/freq
	// in a separate call). We do two fetches: metadata then observations.
	metaURL := fredURL(ch.Spec.Request.BaseURL, "/series", map[string]string{
		"series_id": a.SeriesID,
	})
	metaBody, err := ch.Fetch(ctx, metaURL)
	if err != nil {
		return "", fmt.Errorf("FRED metadata: %w", err)
	}
	var metaResp struct {
		Seriess []struct {
			ID        string `json:"id"`
			Title     string `json:"title"`
			Units     string `json:"units"`
			Frequency string `json:"frequency_short"`
			Notes     string `json:"notes"`
		} `json:"seriess"`
	}
	if err := json.Unmarshal(metaBody, &metaResp); err != nil {
		return "", fmt.Errorf("parse FRED metadata: %w", err)
	}
	title, units, freq := a.SeriesID, "", ""
	if len(metaResp.Seriess) > 0 {
		s := metaResp.Seriess[0]
		if s.Title != "" {
			title = s.Title
		}
		units = s.Units
		freq = s.Frequency
	}

	obsURL := fredURL(ch.Spec.Request.BaseURL, "/series/observations", map[string]string{
		"series_id":         a.SeriesID,
		"observation_start": a.StartDate,
		"observation_end":   a.EndDate,
		"sort_order":        "asc",
		"limit":             fmt.Sprintf("%d", limit),
	})
	obsBody, err := ch.Fetch(ctx, obsURL)
	if err != nil {
		return "", fmt.Errorf("FRED observations: %w", err)
	}
	var obsResp fredObsResp
	if err := json.Unmarshal(obsBody, &obsResp); err != nil {
		return "", fmt.Errorf("parse FRED observations: %w", err)
	}
	if len(obsResp.Observations) == 0 {
		return fmt.Sprintf("No FRED data found for %s in range %s – %s.", a.SeriesID, a.StartDate, a.EndDate), nil
	}

	// Filter out missing values (FRED uses "." for N/A).
	type row struct{ date, value string }
	var rows []row
	for _, o := range obsResp.Observations {
		if o.Value != "." {
			rows = append(rows, row{o.Date, o.Value})
		}
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "# FRED: %s (%s)\n", title, a.SeriesID)
	if units != "" {
		fmt.Fprintf(&sb, "Units: %s", units)
		if freq != "" {
			fmt.Fprintf(&sb, " | Frequency: %s", freq)
		}
		sb.WriteByte('\n')
	}
	sb.WriteByte('\n')
	fmt.Fprintf(&sb, "%-12s  %s\n", "Date", "Value")
	sb.WriteString(strings.Repeat("-", 28) + "\n")
	for _, r := range rows {
		fmt.Fprintf(&sb, "%-12s  %s\n", r.date, r.value)
	}
	fmt.Fprintf(&sb, "\n%d observations (%s to %s)\n", len(rows), rows[0].date, rows[len(rows)-1].date)
	return sb.String(), nil
}

// ── fred_search ───────────────────────────────────────────────────────────────

type fredSearchArgs struct {
	Query string `json:"query"`
	Limit int    `json:"limit"`
}

func extractFREDSearch(ctx context.Context, args json.RawMessage, ch *channel.Channel) (string, error) {
	var a fredSearchArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return "", err
	}
	if fredAPIKey() == "" {
		return "", fmt.Errorf("FRED_API_KEY is not set — register for a free key at https://fred.stlouisfed.org/docs/api/api_key.html")
	}
	if a.Query == "" {
		return "", fmt.Errorf("query is required")
	}
	limit := a.Limit
	if limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}

	searchURL := fredURL(ch.Spec.Request.BaseURL, "/series/search", map[string]string{
		"search_text":     strings.ReplaceAll(a.Query, " ", "+"),
		"limit":           fmt.Sprintf("%d", limit),
		"sort_order":      "desc",
		"order_by":        "popularity",
	})
	body, err := ch.Fetch(ctx, searchURL)
	if err != nil {
		return "", fmt.Errorf("FRED search: %w", err)
	}

	var resp struct {
		Seriess []struct {
			ID            string `json:"id"`
			Title         string `json:"title"`
			Units         string `json:"units"`
			Frequency     string `json:"frequency_short"`
			SeasonAdj     string `json:"seasonal_adjustment_short"`
			LastUpdated   string `json:"last_updated"`
			Popularity    int    `json:"popularity"`
		} `json:"seriess"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("parse FRED search: %w", err)
	}
	if len(resp.Seriess) == 0 {
		return fmt.Sprintf("No FRED series found matching %q.", a.Query), nil
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "# FRED Series Search: %q\n\n", a.Query)
	fmt.Fprintf(&sb, "%-20s  %-8s  %-6s  %-4s  %s\n", "Series ID", "Freq", "Adj", "Pop", "Title")
	sb.WriteString(strings.Repeat("-", 90) + "\n")
	for _, s := range resp.Seriess {
		title := s.Title
		if len(title) > 55 {
			title = title[:52] + "..."
		}
		fmt.Fprintf(&sb, "%-20s  %-8s  %-6s  %-4d  %s\n",
			s.ID, s.Frequency, s.SeasonAdj, s.Popularity, title)
	}
	sb.WriteString("\nUse series_id values above with fred_series.\n")
	return sb.String(), nil
}
