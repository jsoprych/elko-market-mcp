package extract

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/jsoprych/elko-market-mcp/internal/channel"
)

// RegisterYahoo registers all Yahoo Finance extractors.
func RegisterYahoo(r *channel.Runner) {
	r.RegisterExtractor("yahoo_chart_ohlcv", extractYahooOHLCV)
	r.RegisterExtractor("yahoo_chart_quote", extractYahooQuote)
	r.RegisterExtractor("yahoo_chart_dividends", extractYahooDividends)
}

// ── Chart API response types ──────────────────────────────────────────────────

type chartResp struct {
	Chart struct {
		Result []struct {
			Meta struct {
				Symbol             string  `json:"symbol"`
				LongName           string  `json:"longName"`
				ShortName          string  `json:"shortName"`
				Currency           string  `json:"currency"`
				ExchangeName       string  `json:"exchangeName"`
				RegularMarketPrice float64 `json:"regularMarketPrice"`
				PreviousClose      float64 `json:"previousClose"`
				RegularMarketOpen  float64 `json:"regularMarketOpen"`
				DayHigh            float64 `json:"regularMarketDayHigh"`
				DayLow             float64 `json:"regularMarketDayLow"`
				FiftyTwoWeekHigh   float64 `json:"fiftyTwoWeekHigh"`
				FiftyTwoWeekLow    float64 `json:"fiftyTwoWeekLow"`
				RegularMarketVol   int64   `json:"regularMarketVolume"`
				MarketCap          float64 `json:"marketCap"`
			} `json:"meta"`
			Timestamp  []int64 `json:"timestamp"`
			Indicators struct {
				Quote []struct {
					Open   []float64 `json:"open"`
					High   []float64 `json:"high"`
					Low    []float64 `json:"low"`
					Close  []float64 `json:"close"`
					Volume []float64 `json:"volume"`
				} `json:"quote"`
				AdjClose []struct {
					AdjClose []float64 `json:"adjclose"`
				} `json:"adjclose"`
			} `json:"indicators"`
			Events struct {
				Dividends map[string]struct {
					Amount float64 `json:"amount"`
					Date   int64   `json:"date"`
				} `json:"dividends"`
			} `json:"events"`
		} `json:"result"`
		Error *struct {
			Code        string `json:"code"`
			Description string `json:"description"`
		} `json:"error"`
	} `json:"chart"`
}

func fetchChart(ctx context.Context, ch *channel.Channel, sym, period, interval, from, to, events string) (*chartResp, error) {
	url := ch.Spec.Request.BaseURL + "/" + sym
	params := "?interval=" + interval + "&events=" + events

	if from != "" {
		t, err := time.Parse("2006-01-02", from)
		if err != nil {
			return nil, fmt.Errorf("invalid from date: %w", err)
		}
		params += "&period1=" + strconv.FormatInt(t.Unix(), 10)
		if to != "" {
			t2, err := time.Parse("2006-01-02", to)
			if err != nil {
				return nil, fmt.Errorf("invalid to date: %w", err)
			}
			params += "&period2=" + strconv.FormatInt(t2.Unix(), 10)
		} else {
			params += "&period2=" + strconv.FormatInt(time.Now().Unix(), 10)
		}
	} else {
		if period == "" {
			period = "1mo"
		}
		params += "&range=" + period
	}

	body, err := ch.Fetch(ctx, url+params)
	if err != nil {
		return nil, err
	}
	var r chartResp
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("parse chart: %w", err)
	}
	if r.Chart.Error != nil {
		return nil, fmt.Errorf("yahoo error %s: %s", r.Chart.Error.Code, r.Chart.Error.Description)
	}
	if len(r.Chart.Result) == 0 {
		return nil, fmt.Errorf("no data for %s", sym)
	}
	return &r, nil
}

// ── Extractors ────────────────────────────────────────────────────────────────

type histArgs struct {
	Symbol   string `json:"symbol"`
	Period   string `json:"period"`
	From     string `json:"from"`
	To       string `json:"to"`
	Interval string `json:"interval"`
}

func extractYahooOHLCV(ctx context.Context, args json.RawMessage, ch *channel.Channel) (string, error) {
	var a histArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return "", err
	}
	if a.Symbol == "" {
		return "", fmt.Errorf("symbol is required")
	}
	a.Symbol = strings.ToUpper(a.Symbol)
	if a.Interval == "" {
		a.Interval = "1d"
	}

	r, err := fetchChart(ctx, ch, a.Symbol, a.Period, a.Interval, a.From, a.To, "div")
	if err != nil {
		return "", err
	}

	res := r.Chart.Result[0]
	q := res.Indicators.Quote
	if len(q) == 0 || len(res.Timestamp) == 0 {
		return fmt.Sprintf("No OHLCV data for %s", a.Symbol), nil
	}
	quote := q[0]

	var adjClose []float64
	if len(res.Indicators.AdjClose) > 0 {
		adjClose = res.Indicators.AdjClose[0].AdjClose
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "# %s Price History (%s)\n\n", a.Symbol, a.Interval)
	sb.WriteString("Timestamp,Date,Open,High,Low,Close,AdjClose,Volume\n")

	for i, ts := range res.Timestamp {
		if i >= len(quote.Close) {
			break
		}
		c := quote.Close[i]
		if math.IsNaN(c) || c == 0 {
			continue
		}
		date := time.Unix(ts, 0).UTC().Format("2006-01-02")
		o := safeF(quote.Open, i)
		h := safeF(quote.High, i)
		l := safeF(quote.Low, i)
		vol := safeI(quote.Volume, i)
		ac := safeF(adjClose, i)
		fmt.Fprintf(&sb, "%d,%s,%.4f,%.4f,%.4f,%.4f,%.4f,%d\n",
			ts, date, o, h, l, c, ac, vol)
	}
	return sb.String(), nil
}

type quoteArgs struct {
	Symbol string `json:"symbol"`
}

func extractYahooQuote(ctx context.Context, args json.RawMessage, ch *channel.Channel) (string, error) {
	var a quoteArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return "", err
	}
	if a.Symbol == "" {
		return "", fmt.Errorf("symbol is required")
	}
	a.Symbol = strings.ToUpper(a.Symbol)

	r, err := fetchChart(ctx, ch, a.Symbol, "1d", "1d", "", "", "")
	if err != nil {
		return "", err
	}
	m := r.Chart.Result[0].Meta

	name := m.LongName
	if name == "" {
		name = m.ShortName
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "# %s — %s\n\n", a.Symbol, name)
	fmt.Fprintf(&sb, "%-28s %s\n", "Exchange:", m.ExchangeName)
	fmt.Fprintf(&sb, "%-28s %s\n", "Currency:", m.Currency)
	fmt.Fprintf(&sb, "%-28s %.2f\n", "Price:", m.RegularMarketPrice)
	fmt.Fprintf(&sb, "%-28s %.2f\n", "Previous Close:", m.PreviousClose)
	fmt.Fprintf(&sb, "%-28s %.2f\n", "Open:", m.RegularMarketOpen)
	fmt.Fprintf(&sb, "%-28s %.2f – %.2f\n", "Day Range:", m.DayLow, m.DayHigh)
	fmt.Fprintf(&sb, "%-28s %.2f – %.2f\n", "52W Range:", m.FiftyTwoWeekLow, m.FiftyTwoWeekHigh)
	fmt.Fprintf(&sb, "%-28s %s\n", "Volume:", formatLarge(float64(m.RegularMarketVol)))
	if m.MarketCap > 0 {
		fmt.Fprintf(&sb, "%-28s %s\n", "Market Cap:", formatLarge(m.MarketCap))
	}
	return sb.String(), nil
}

type divArgs struct {
	Symbol string `json:"symbol"`
	From   string `json:"from"`
	To     string `json:"to"`
}

func extractYahooDividends(ctx context.Context, args json.RawMessage, ch *channel.Channel) (string, error) {
	var a divArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return "", err
	}
	if a.Symbol == "" {
		return "", fmt.Errorf("symbol is required")
	}
	a.Symbol = strings.ToUpper(a.Symbol)

	r, err := fetchChart(ctx, ch, a.Symbol, "max", "1mo", a.From, a.To, "dividends")
	if err != nil {
		return "", err
	}

	divs := r.Chart.Result[0].Events.Dividends
	if len(divs) == 0 {
		return fmt.Sprintf("No dividend data found for %s.", a.Symbol), nil
	}

	type divRow struct {
		date   string
		amount float64
	}
	rows := make([]divRow, 0, len(divs))
	for _, d := range divs {
		rows = append(rows, divRow{
			date:   time.Unix(d.Date, 0).UTC().Format("2006-01-02"),
			amount: d.Amount,
		})
	}
	for i := 0; i < len(rows); i++ {
		for j := i + 1; j < len(rows); j++ {
			if rows[j].date < rows[i].date {
				rows[i], rows[j] = rows[j], rows[i]
			}
		}
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "# %s Dividends\n\n", a.Symbol)
	sb.WriteString("Date,Amount\n")
	for _, row := range rows {
		fmt.Fprintf(&sb, "%s,%.4f\n", row.date, row.amount)
	}
	return sb.String(), nil
}

// ── Utilities ─────────────────────────────────────────────────────────────────

func safeF(s []float64, i int) float64 {
	if i < len(s) {
		return s[i]
	}
	return 0
}

func safeI(s []float64, i int) int64 {
	if i < len(s) {
		return int64(s[i])
	}
	return 0
}

func formatLarge(v float64) string {
	switch {
	case v >= 1e12:
		return fmt.Sprintf("%.2fT", v/1e12)
	case v >= 1e9:
		return fmt.Sprintf("%.2fB", v/1e9)
	case v >= 1e6:
		return fmt.Sprintf("%.2fM", v/1e6)
	default:
		return fmt.Sprintf("%.0f", v)
	}
}
