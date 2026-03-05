# Adding a New Channel

A "channel" is elko's name for a data source tool. Each channel consists of two things:

1. **A JSON spec file** — declares the tool's name, description, input schema, HTTP config, and extractor reference
2. **A Go extractor function** — fetches and parses the API response, returns formatted text

Once those two pieces are in place, elko automatically makes the tool available via MCP, REST, and CLI — no additional wiring needed.

---

## Table of Contents

1. [How Channels Work](#how-channels-work)
2. [The Two Files You Need](#the-two-files-you-need)
3. [Worked Example: CoinGecko Market Chart](#worked-example-coingecko-market-chart)
   - [Step 1: Write the JSON spec](#step-1-write-the-json-spec)
   - [Step 2: Write the Go extractor](#step-2-write-the-go-extractor)
   - [Step 3: Register the extractor](#step-3-register-the-extractor)
   - [Step 4: Test it](#step-4-test-it)
4. [JSON Spec Reference](#json-spec-reference)
5. [Extractor Function Reference](#extractor-function-reference)
6. [Tips and Patterns](#tips-and-patterns)

---

## How Channels Work

When elko starts, it:

1. Walks `channels/**/*.json` (embedded in the binary) and loads every `Spec`
2. For each spec, looks up the named extractor from the extractor registry
3. Wraps the two together into a `Channel` with a `Fetch` closure
4. Registers the channel as a `registry.Tool` with name, schema, and handler

The `Fetch` closure handles everything transport-related: cache lookup, HTTP GET, header injection, cache write. The extractor only has to call `ch.Fetch(ctx, url)` and parse the result.

```
JSON spec      ──────────────────────────────────► Spec struct (name, schema, request config)
                                                          │
Go extractor  ──► registered as ExtractorFunc            │
                                                          │
runner.Register ─────────────────────────────────► Channel{Spec, Fetch closure}
                                                          │
registry.Register ───────────────────────────────► Tool{Name, Schema, Handler}
                                                          │
MCP / REST / CLI ◄───────────────────────────────── tool invocation
```

---

## The Two Files You Need

| File | Location | Purpose |
|------|----------|---------|
| `channels/<source>/<name>.json` | Embedded in binary | Tool metadata, input schema, HTTP config |
| `internal/channel/extract/<source>.go` | Go source | Fetch, parse, format the API response |

Plus a one-line registration call in `internal/channel/extract/all.go`.

---

## Worked Example: CoinGecko Market Chart

We'll add a new tool `coingecko_market_chart` that fetches historical price/volume data for any cryptocurrency from the [CoinGecko public API](https://www.coingecko.com/en/api) — no API key required.

**API endpoint:**
```
GET https://api.coingecko.com/api/v3/coins/{id}/market_chart
    ?vs_currency=usd
    &days=30
```

**Response shape:**
```json
{
  "prices":       [[timestamp_ms, price], ...],
  "market_caps":  [[timestamp_ms, market_cap], ...],
  "total_volumes": [[timestamp_ms, volume], ...]
}
```

---

### Step 1: Write the JSON spec

Create `channels/coingecko/market_chart.json`:

```json
{
  "name": "coingecko_market_chart",
  "description": "Historical price, market cap, and volume from CoinGecko. Free, no key required.",
  "source": "coingecko",
  "category": "crypto",

  "schema": {
    "type": "object",
    "properties": {
      "id": {
        "type": "string",
        "description": "CoinGecko coin ID (e.g. bitcoin, ethereum, solana)",
        "examples": ["bitcoin", "ethereum", "solana", "dogecoin", "chainlink"]
      },
      "days": {
        "type": "string",
        "description": "Number of days of history (1, 7, 30, 90, 365, max)",
        "enum": ["1", "7", "30", "90", "365", "max"],
        "default": "30"
      },
      "currency": {
        "type": "string",
        "description": "vs currency (usd, eur, btc)",
        "enum": ["usd", "eur", "btc"],
        "default": "usd"
      }
    },
    "required": ["id"]
  },

  "result_format": "csv",

  "chart": {
    "type": "line",
    "x": "Date",
    "y": "Price"
  },

  "request": {
    "base_url": "https://api.coingecko.com/api/v3/coins",
    "headers": {
      "Accept": "application/json"
    },
    "ttl": "1h"
  },

  "response": {
    "extractor": "coingecko_market_chart"
  }
}
```

**What each field does:**

| Field | Effect |
|-------|--------|
| `name` | Tool identifier. Used as CLI arg (`elko call coingecko_market_chart`), REST path (`/v1/call/coingecko_market_chart`), MCP tool name |
| `source` | Groups the tool in the web sidebar. Any lowercase string — `"coingecko"` creates a new "coingecko" section |
| `category` | Sub-grouping. `"crypto"` appears under the source |
| `schema` | JSON Schema for input args. Drives MCP tool schemas, web form generation, and arg validation |
| `result_format: "csv"` | Tells the web renderer to parse the output as a CSV table |
| `chart` | Tells the web UI to render a line chart using `Date` as x-axis and `Price` as y-axis |
| `request.base_url` | Base URL — the extractor appends the path and query string |
| `request.ttl` | Cache TTL. `"1h"` means live prices cached for 1 hour |
| `response.extractor` | Name to look up in the extractor registry at startup |

---

### Step 2: Write the Go extractor

Create `internal/channel/extract/coingecko.go`:

```go
package extract

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jsoprych/elko-market-mcp/internal/channel"
)

// RegisterCoinGecko registers the CoinGecko extractors.
func RegisterCoinGecko(r *channel.Runner) {
	r.RegisterExtractor("coingecko_market_chart", extractCoinGeckoMarketChart)
}

type cgMarketChartArgs struct {
	ID       string `json:"id"`
	Days     string `json:"days"`
	Currency string `json:"currency"`
}

type cgMarketChartResponse struct {
	Prices      [][2]float64 `json:"prices"`
	MarketCaps  [][2]float64 `json:"market_caps"`
	TotalVolumes [][2]float64 `json:"total_volumes"`
}

func extractCoinGeckoMarketChart(ctx context.Context, args json.RawMessage, ch *channel.Channel) (string, error) {
	// 1. Parse arguments.
	var a cgMarketChartArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return "", err
	}
	if a.ID == "" {
		return "", fmt.Errorf("id is required")
	}
	if a.Days == "" {
		a.Days = "30"
	}
	if a.Currency == "" {
		a.Currency = "usd"
	}

	// 2. Build the URL.
	apiURL := fmt.Sprintf(
		"%s/%s/market_chart?vs_currency=%s&days=%s",
		ch.Spec.Request.BaseURL, a.ID, a.Currency, a.Days,
	)

	// 3. Fetch (handles cache + HTTP + headers — all centralized in runner).
	body, err := ch.Fetch(ctx, apiURL)
	if err != nil {
		return "", fmt.Errorf("coingecko: %w", err)
	}

	// 4. Parse the response.
	var resp cgMarketChartResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("coingecko: parse error: %w", err)
	}
	if len(resp.Prices) == 0 {
		return fmt.Sprintf("No data for %s.", a.ID), nil
	}

	// 5. Format as CSV.
	var sb strings.Builder
	fmt.Fprintf(&sb, "# %s — %s days, vs %s\n\n", a.ID, a.Days, strings.ToUpper(a.Currency))
	sb.WriteString("Timestamp,Date,Price,MarketCap,Volume\n")

	for i, p := range resp.Prices {
		ts := int64(p[0]) / 1000 // milliseconds → seconds
		date := time.Unix(ts, 0).UTC().Format("2006-01-02")
		price := p[1]

		var mc, vol float64
		if i < len(resp.MarketCaps) {
			mc = resp.MarketCaps[i][1]
		}
		if i < len(resp.TotalVolumes) {
			vol = resp.TotalVolumes[i][1]
		}

		fmt.Fprintf(&sb, "%d,%s,%.4f,%.0f,%.0f\n", ts, date, price, mc, vol)
	}

	return sb.String(), nil
}
```

**The extractor's responsibilities:**
1. Unmarshal `args` into a typed struct
2. Apply defaults for optional fields
3. Build the full URL from `ch.Spec.Request.BaseURL` + query params
4. Call `ch.Fetch(ctx, url)` — this handles all caching and HTTP
5. Unmarshal the response body
6. Format and return the result as a string

**What the extractor does NOT do:**
- Make HTTP requests directly (that's `ch.Fetch`)
- Manage headers or auth tokens (declared in the JSON spec)
- Handle cache (handled by `ch.Fetch`)

---

### Step 3: Register the extractor

Add one line to `internal/channel/extract/all.go`:

```go
func RegisterAll(r *channel.Runner) {
	RegisterYahoo(r)
	RegisterEDGAR(r)
	RegisterTreasury(r)
	RegisterBLS(r)
	RegisterFDIC(r)
	RegisterWorldBank(r)
	RegisterCoinGecko(r)  // ← add this line
}
```

That's it. elko will discover the `channels/coingecko/market_chart.json` spec at startup, match it to the `"coingecko_market_chart"` extractor, and register the tool.

---

### Step 4: Test it

```bash
# Rebuild
go build -o elko ./cmd/elko

# Verify it's registered
./elko catalogue | grep coingecko

# CLI test
./elko call coingecko_market_chart id=bitcoin days=30

# With specific currency
./elko call coingecko_market_chart id=ethereum days=7 currency=usd

# REST test
./elko serve --port 8080 &
curl -s -XPOST localhost:8080/v1/call/coingecko_market_chart \
  -H 'Content-Type: application/json' \
  -d '{"id":"solana","days":"90"}'

# Pipe to CSV file
./elko call coingecko_market_chart id=bitcoin days=365 > btc_365d.csv
```

In the web dashboard (`http://localhost:8080`), the new tool will appear in the sidebar under **coingecko → crypto → coingecko_market_chart**. Because `result_format: "csv"` and a `chart` block are declared, the result will render as a table with a line chart above it.

---

## JSON Spec Reference

### Top-level fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | yes | Unique tool identifier. Must match `response.extractor` name in the extractor registry |
| `description` | string | yes | Human-readable description shown in MCP tool list and dashboard sidebar |
| `source` | string | yes | Source group tag. Determines sidebar grouping. Use lowercase snake_case |
| `category` | string | yes | Sub-category tag within the source |
| `schema` | JSON Schema | yes | Input argument schema. See below |
| `result_format` | string | yes | Output hint for the web renderer: `"csv"`, `"table"`, `"kv"`, `"sections"` |
| `chart` | object | no | If present, the web UI renders a chart. Fields: `type` (`"line"`\|`"bar"`), `x` (column name), `y` (column name) |
| `request` | object | yes | HTTP config |
| `response` | object | yes | Names the extractor function |

### `request` fields

| Field | Type | Description |
|-------|------|-------------|
| `base_url` | string | Base URL. The extractor appends paths and query strings |
| `headers` | `{string: string}` | Static HTTP headers always sent |
| `env_headers` | `{string: string}` | Maps header name → env var name. Header is set if the env var is non-empty. Used for `SEC_USER_AGENT` |
| `ttl` | string | Cache duration. Go duration string: `"5m"`, `"1h"`, `"24h"`. Default: `"1h"` |

### `schema` / JSON Schema extensions

Standard JSON Schema (`type`, `properties`, `required`, `enum`, `minimum`, `maximum`, `default`, `description`). Additionally:

| Extension | Web form effect |
|-----------|----------------|
| `"format": "date"` | Renders as `<input type="date">` with a calendar picker |
| `"examples": [...]` | Renders as `<datalist>` autocomplete attached to the input |
| `"enum": [...]` | Renders as `<select>` dropdown |
| `"default": "..."` | Pre-fills the field |

### `result_format` values

| Value | Web rendering | Typical use |
|-------|---------------|------------|
| `"csv"` | HTML table parsed from CSV | Price history, dividend events |
| `"table"` | HTML table parsed from fixed-width text | Financial statements, yield curves |
| `"kv"` | Key-value pairs | Stock quotes, company info |
| `"sections"` | Multiple labeled key-value groups | Bank financials |

---

## Extractor Function Reference

```go
type ExtractorFunc func(
    ctx  context.Context,
    args json.RawMessage,
    ch   *channel.Channel,
) (string, error)
```

### `ch.Fetch(ctx, url)`

```go
body, err := ch.Fetch(ctx, url)
```

Returns `([]byte, error)`. Handles:
- Cache key: `"<source>:<url>"`
- Cache lookup (L1 memory, optional L2 SQLite)
- HTTP GET with merged headers (static from spec + env-var from spec)
- Cache write on success with the spec's TTL
- Error message includes source name and HTTP status

### `ch.Spec`

The parsed JSON spec. Useful fields:
- `ch.Spec.Request.BaseURL` — the base URL declared in the JSON
- `ch.Spec.Source` — source tag
- `ch.Spec.Name` — tool name

### Output format conventions

| result_format | Expected output |
|---------------|----------------|
| `"csv"` | Header line + data rows: `"Col1,Col2,Col3\nval,val,val\n..."` |
| `"table"` | Fixed-width aligned columns, header, separator line |
| `"kv"` | `"Key:   Value\n"` pairs |
| `"sections"` | Multiple `"# Section\n\nKey: Value\n"` blocks |

The web renderer uses `result_format` to decide how to parse the text. Mismatches (e.g. returning CSV when `result_format` is `"kv"`) will cause incorrect web rendering but won't break CLI or MCP output.

---

## Tips and Patterns

### Use `ch.Spec.Request.BaseURL` for the base

Don't hardcode the API URL in Go — put it in the JSON spec's `base_url`. The extractor appends paths and query parameters:

```go
url := fmt.Sprintf("%s/%s/endpoint?param=%s", ch.Spec.Request.BaseURL, id, val)
```

### Apply defaults for optional fields

The MCP/web UI may omit optional args. Apply sensible defaults after unmarshalling:

```go
if a.Days == "" { a.Days = "30" }
if a.Currency == "" { a.Currency = "usd" }
```

### Headers with env-var injection

For APIs that require an auth token or identifying header, use `env_headers` in the JSON spec:

```json
"request": {
  "base_url": "https://api.example.com",
  "env_headers": {
    "X-API-Key": "EXAMPLE_API_KEY"
  }
}
```

The user sets `EXAMPLE_API_KEY=mytoken` in their environment. The header is injected automatically by the runner — no Go code needed.

### For env-var-only auth headers, document it

Add a note to the tool's `description` field in the JSON:

```json
"description": "... Requires EXAMPLE_API_KEY environment variable."
```

### Pagination

For APIs that paginate, make multiple `ch.Fetch` calls with different page parameters and concatenate results:

```go
var rows []string
for page := 1; page <= maxPages; page++ {
    url := fmt.Sprintf("%s?page=%d&...", ch.Spec.Request.BaseURL, page)
    body, err := ch.Fetch(ctx, url)
    // parse, append to rows
    if len(parsed) < pageSize { break }  // last page
}
```

Each URL has its own cache entry, so paginated results are individually cached.

### Multiple tools from one source file

A single `Register*` function can register multiple extractors, and a source directory can have multiple JSON files:

```
channels/myapi/
  prices.json          → "extractor": "myapi_prices"
  metrics.json         → "extractor": "myapi_metrics"

internal/channel/extract/myapi.go
  RegisterMyAPI(r) {
      r.RegisterExtractor("myapi_prices",  extractMyAPIPrices)
      r.RegisterExtractor("myapi_metrics", extractMyAPIMetrics)
  }
```

### Shared state within a source (e.g. ID lookup cache)

If your source needs a shared lookup cache (like EDGAR's CIK lookup), use a package-level variable with a `sync.RWMutex`:

```go
var (
    idCacheMu sync.RWMutex
    idCache   = map[string]string{}
)

func lookupID(ctx context.Context, symbol string, ch *channel.Channel) (string, error) {
    idCacheMu.RLock()
    if id, ok := idCache[symbol]; ok {
        idCacheMu.RUnlock()
        return id, nil
    }
    idCacheMu.RUnlock()

    // fetch ID from API...

    idCacheMu.Lock()
    idCache[symbol] = id
    idCacheMu.Unlock()
    return id, nil
}
```

### Naming conventions

| Thing | Convention | Example |
|-------|-----------|---------|
| Tool name | `<source>_<description>` | `coingecko_market_chart` |
| Extractor name | Same as tool name | `"coingecko_market_chart"` |
| Source tag | lowercase | `"coingecko"` |
| Category tag | lowercase | `"crypto"` |
| JSON spec file | `<tool_name_without_source>.json` | `market_chart.json` |
| Go file | `<source>.go` | `coingecko.go` |
| Register function | `Register<Source>` | `RegisterCoinGecko` |
