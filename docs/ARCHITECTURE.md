# Architecture

elko is a Go binary that exposes 10 financial data tools through three simultaneous interfaces (MCP, REST, CLI) via a single shared tool registry. Every tool is defined as a JSON channel spec; Go code handles only source-specific HTTP response parsing.

---

## Table of Contents

1. [Three-Interface Design](#three-interface-design)
2. [Channel Pipeline](#channel-pipeline)
3. [Registry](#registry)
4. [Channel Spec Format](#channel-spec-format)
5. [Runner and Fetch Closure](#runner-and-fetch-closure)
6. [Cache System](#cache-system)
7. [Extractor Pattern](#extractor-pattern)
8. [Web Frontend](#web-frontend)
9. [Data Flow Diagram](#data-flow-diagram)
10. [File Map](#file-map)

---

## Three-Interface Design

```
┌─────────────────────────────────────────────────────────────────┐
│                     Three Interfaces                            │
│                                                                 │
│   ┌──────────┐        ┌──────────┐        ┌──────────┐        │
│   │   MCP    │        │   REST   │        │   CLI    │        │
│   │  stdio   │        │   API    │        │  call    │        │
│   │ (AI use) │        │(+web UI) │        │(scripts) │        │
│   └────┬─────┘        └────┬─────┘        └────┬─────┘        │
│        │                   │                   │               │
│        └───────────────────┼───────────────────┘               │
│                            │                                    │
│                    ┌───────▼──────┐                            │
│                    │   Registry   │   One shared catalogue      │
│                    │  (10 tools)  │   thread-safe, RWMutex      │
│                    └──────────────┘                            │
└─────────────────────────────────────────────────────────────────┘
```

All three interfaces share a single `*registry.Registry` instance built at startup. This means:
- Adding a tool makes it available everywhere simultaneously
- No interface-specific registration code
- The web dashboard's sidebar is generated from the same catalogue as MCP's `tools/list`

### Startup sequence

```
main.go
  │
  ├── NewRunner(cache)              # HTTP client + cache wrapper
  ├── extract.RegisterAll(runner)  # Register per-source extractors
  ├── LoadFS(channels.FS)          # Load embedded JSON specs
  ├── runner.Register(registry, specs)  # Build registry from specs + extractors
  │
  ├── [if "mcp"]    mcp.Serve(registry, os.Stdin, os.Stdout)
  ├── [if "serve"]  api.Serve(registry, port, ...)
  └── [if "call"]   registry.Call(toolName, args) → print
```

---

## Channel Pipeline

Each tool invocation follows this path:

```
Caller (MCP/REST/CLI)
        │
        ▼
   Registry.Call(name, args)
        │
        ▼
   ExtractorFunc(ctx, args, channel)          ← source-specific Go code
        │
        ├── parse args (json.Unmarshal)
        ├── build URL
        │
        ▼
   channel.Fetch(ctx, url)                    ← centralized in runner.go
        │
        ├── cache.Get(key)  ──hit──► return cached bytes
        │        │
        │       miss
        │        │
        ▼
   http.Client.Get(url)                       ← single shared HTTP client
        │
        ▼
   cache.Set(key, body, TTL)
        │
        ▼
   return []byte to ExtractorFunc
        │
        ▼
   json.Unmarshal into source struct
   filter / transform
   fmt.Fprintf → formatted string
        │
        ▼
   return (string, error) to Registry
        │
        ▼
   caller receives formatted text
```

**What is centralized (runner.go):**
- Single `*http.Client` for all sources
- Cache get/set with TTL
- Header merging (static headers from spec + dynamic from env vars)
- Cache key namespacing (`source:url`)

**What is per-source (extract/*.go):**
- URL construction from tool arguments
- JSON unmarshalling into source-specific structs
- Data filtering and transformation
- Output formatting (CSV, table, key-value)

---

## Registry

`internal/registry/registry.go`

The registry is a thread-safe catalogue of all available tools.

```go
type Tool struct {
    Name        string
    Description string
    Source      string
    Category    string
    Schema      json.RawMessage   // JSON Schema for input args
    ResultFormat string           // "csv" | "table" | "kv" | "sections"
    Chart       *ChartSpec        // optional: type, x column, y column
    Handler     HandlerFunc       // func(ctx, args) (string, error)
}

type Registry struct {
    mu    sync.RWMutex
    tools map[string]Tool
}
```

The registry exposes:
- `Register(tool Tool)` — add a tool
- `Call(name string, args json.RawMessage) (string, error)` — invoke
- `List() []Tool` — enumerate all tools (for MCP `tools/list`, REST catalogue, CLI `catalogue`)

---

## Channel Spec Format

Each tool is defined in a JSON file embedded in the binary via Go's `embed.FS`. All specs live in `channels/<source>/<name>.json`.

```json
{
  "name": "yahoo_history",
  "description": "OHLCV price history from Yahoo Finance.",
  "source": "yahoo",
  "category": "equity",

  "schema": {
    "type": "object",
    "properties": {
      "symbol":   { "type": "string", "description": "Ticker symbol" },
      "period":   { "type": "string", "enum": ["1d","5d","1mo","3mo","6mo","1y","2y","5y","10y","ytd","max"], "default": "1mo" },
      "from":     { "type": "string", "description": "Start date YYYY-MM-DD", "format": "date" },
      "to":       { "type": "string", "description": "End date YYYY-MM-DD",   "format": "date" },
      "interval": { "type": "string", "enum": ["1m","5m","15m","30m","1h","1d","1wk","1mo"], "default": "1d" }
    },
    "required": ["symbol"]
  },

  "result_format": "csv",

  "chart": {
    "type": "line",
    "x": "Date",
    "y": "Close"
  },

  "request": {
    "base_url": "https://query1.finance.yahoo.com/v8/finance/chart",
    "headers": {
      "User-Agent": "Mozilla/5.0 (compatible; elko-market-mcp/1.0)"
    },
    "ttl": "1h"
  },

  "response": {
    "extractor": "yahoo_chart_ohlcv"
  }
}
```

### Spec fields

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Unique tool identifier. Used as CLI arg, REST path, MCP tool name |
| `description` | string | Human-readable description. Shown in MCP `tools/list`, dashboard sidebar |
| `source` | string | Source tag: `yahoo`, `edgar`, `treasury`, `bls`, `fdic`, `worldbank` |
| `category` | string | Category tag: `equity`, `macro`, `rates`, `banking` |
| `schema` | JSON Schema | Input argument schema. Used for MCP tool schemas, web form generation, arg validation |
| `result_format` | string | Output hint: `csv`, `table`, `kv`, `sections`. Used by web renderer |
| `chart` | object | Optional chart spec: `type` (`line`\|`bar`), `x` (column name), `y` (column name) |
| `request.base_url` | string | Base URL for the API endpoint |
| `request.headers` | object | Static headers always sent |
| `request.env_headers` | object | Headers injected from environment variables (e.g. `SEC_USER_AGENT`) |
| `request.ttl` | string | Cache TTL: `"5m"`, `"1h"`, `"24h"` |
| `response.extractor` | string | Name of the registered `ExtractorFunc` that handles this tool's response |

### Schema field extensions

The JSON Schema properties support these additional fields used by the web form builder:

| Extension | Effect |
|-----------|--------|
| `"format": "date"` | Renders as `<input type="date">` |
| `"examples": [...]` | Renders as `<datalist>` autocomplete |
| `"enum": [...]` | Renders as `<select>` |
| `"default": "..."` | Pre-fills the form field |
| `"minimum"` / `"maximum"` | Sets `min`/`max` on number inputs |

---

## Runner and Fetch Closure

`internal/channel/runner.go`

The Runner is constructed once at startup. It owns the shared HTTP client and cache, and creates a `Fetch` closure for each channel.

```go
type Runner struct {
    http       *http.Client
    cache      *cache.Cache
    extractors map[string]ExtractorFunc
}
```

**`makeFetch(spec Spec) FetchFunc`** returns a closure that:
1. Builds cache key: `spec.Source + ":" + url`
2. Checks `cache.Get(key)` — returns if hit
3. Makes `http.Get(url)` with merged headers (static spec headers + env-var headers)
4. On success: `cache.Set(key, body, spec.Request.TTL)`
5. Returns raw bytes

**`Register(registry, specs)`** iterates all specs, looks up the named extractor, and registers a `registry.HandlerFunc` wrapper that:
1. Calls the `ExtractorFunc` with `(ctx, args, channel)`
2. The extractor calls `channel.Fetch(ctx, url)` internally
3. Returns formatted string result

---

## Cache System

`internal/cache/cache.go`

Two-layer cache:

```
L1: in-memory (always active)
└── map[string]cacheEntry{data []byte, expiry time.Time}

L2: SQLite file (optional, --db flag)
└── table: cache(key TEXT PK, data BLOB, expiry INTEGER)
```

On a cache miss in L1, the system checks L2 before hitting the network. Writes go to both layers.

**TTL values by source:**

| Source | TTL | Rationale |
|--------|-----|-----------|
| Yahoo Finance quotes | 1h | Stale intraday is acceptable |
| Yahoo Finance history | 1h | Daily bars don't change |
| Yahoo Finance dividends | 24h | Rare updates |
| SEC EDGAR | 24h | Filings update infrequently |
| Treasury | 24h | Published daily at end-of-day |
| BLS | 24h | Monthly releases |
| FDIC | 24h | Quarterly call reports |
| World Bank | 24h | Annual data |

---

## Extractor Pattern

`internal/channel/extract/`

Each source has its own file containing one or more `ExtractorFunc` implementations:

```go
type ExtractorFunc func(ctx context.Context, args json.RawMessage, ch *channel.Channel) (string, error)
```

The extractor is responsible for:
1. Parsing `args` into a source-specific struct
2. Building the full API URL from `ch.Spec.Request.BaseURL` + args
3. Calling `ch.Fetch(ctx, url)` to get cached or fresh bytes
4. Unmarshalling the bytes into a source-specific Go struct
5. Filtering, transforming, and formatting the data
6. Returning a formatted string

**Example (simplified):**

```go
// internal/channel/extract/yahoo.go

func extractYahooQuote(ctx context.Context, args json.RawMessage, ch *channel.Channel) (string, error) {
    var a struct {
        Symbol string `json:"symbol"`
    }
    json.Unmarshal(args, &a)

    url := ch.Spec.Request.BaseURL + "/" + a.Symbol + "?range=1d&interval=1d"
    body, err := ch.Fetch(ctx, url)
    if err != nil { return "", err }

    var resp yahooChartResponse
    json.Unmarshal(body, &resp)

    // format as key-value output
    meta := resp.Chart.Result[0].Meta
    var sb strings.Builder
    fmt.Fprintf(&sb, "Name:  %s\n", meta.LongName)
    fmt.Fprintf(&sb, "Price: %.2f\n", meta.RegularMarketPrice)
    // ...
    return sb.String(), nil
}
```

**Registration:**

```go
// internal/channel/extract/all.go

func RegisterAll(r *channel.Runner) {
    RegisterYahoo(r)
    RegisterEdgar(r)
    RegisterTreasury(r)
    RegisterBLS(r)
    RegisterFDIC(r)
    RegisterWorldBank(r)
}
```

Each `Register*` function calls `r.RegisterExtractor(name, fn)` for each tool it owns.

---

## Web Frontend

`web/src/` — pure JavaScript, no npm, no build step. Embedded in the binary via `embed.FS`.

```
web/
├── index.html           5-line shell: loads style.css + app.js
└── src/
    ├── app.js           Layout, result history (50 entries), URL state sync
    ├── sidebar.js       Source → Category → Tool collapsible tree
    ├── form-builder.js  JSON Schema → HTML form (inputs, selects, datalist, date)
    ├── chart.js         Pure-SVG line and bar charts (no d3, no chart.js)
    ├── renderer.js      CSV/table/KV/sections → HTML table / pre block
    ├── export.js        TXT/CSV/JSON converters + clipboard + file download
    ├── runner.js        POST /v1/call/<tool>, error display
    └── style.css        Layout: sidebar + main panel, toolbar, table styling
```

### Data flow in the browser

```
page load
  │
  ├── fetch /v1/catalogue
  │     └── build sidebar tree (source → category → tool)
  │
  ├── read URL params (?tool=&arg=...)
  │     ├── select tool in sidebar
  │     ├── populate form fields
  │     └── if all required args present → auto-run
  │
user selects tool
  │
  ├── form-builder builds form from spec.schema
  │
user submits form
  │
  ├── runner.js POST /v1/call/<tool> {args}
  ├── push result to history (max 50)
  │
  ├── renderer.js renders result:
  │     ├── csv → HTML table (>100 rows collapse)
  │     ├── table → HTML table
  │     ├── kv → definition list
  │     ├── sections → grouped key-value
  │     └── fallback → <pre>
  │
  ├── if spec.chart → chart.js renders SVG below table
  │
  └── URL updated: ?tool=name&arg1=val1&arg2=val2
```

### SVG chart rendering

`chart.js` generates SVG directly — no external dependencies. Supports:
- **Line charts** — for time-series (price history, yield curves, BLS series)
- **Bar charts** — for categorical data
- Auto-scaled axes with gridlines and labels
- Parses result text (CSV or fixed-width table) to extract x/y columns declared in the channel spec's `chart` block

---

## Data Flow Diagram

Full end-to-end flow for a `yahoo_history` tool call:

```
User (Claude/browser/curl/terminal)
  │
  │  "Get AAPL 1-year daily history"
  ▼
Interface Layer
  ├── MCP:  {"method":"tools/call","params":{"name":"yahoo_history","arguments":{"symbol":"AAPL","period":"1y"}}}
  ├── REST: POST /v1/call/yahoo_history  {"symbol":"AAPL","period":"1y"}
  └── CLI:  ./elko call yahoo_history symbol=AAPL period=1y
  │
  ▼
registry.Call("yahoo_history", {"symbol":"AAPL","period":"1y"})
  │
  ▼
ExtractorFunc: extractYahooOHLCV(ctx, args, ch)
  │
  ├── parse args → symbol="AAPL", period="1y", interval="1d"
  ├── build URL: https://query1.finance.yahoo.com/v8/finance/chart/AAPL?range=1y&interval=1d
  │
  ▼
ch.Fetch(ctx, url)  [runner.makeFetch closure]
  │
  ├── key = "yahoo:https://query1.finance.yahoo.com/..."
  ├── L1 cache miss
  ├── L2 cache miss (or not configured)
  │
  ▼
http.Client.Get(url)
  Headers: User-Agent: Mozilla/5.0 (compatible; elko-market-mcp/1.0)
  │
  ▼
Yahoo Finance API → 200 OK, JSON body (OHLCV arrays)
  │
  ▼
cache.Set(key, body, TTL=1h)
  │
  ▼
extractYahooOHLCV receives []byte
  ├── json.Unmarshal into yahooChartResponse struct
  ├── filter NaN values, zip timestamps with OHLCV arrays
  ├── fmt.Fprintf CSV: Timestamp,Date,Open,High,Low,Close,AdjClose,Volume
  │
  ▼
return (csvString, nil) to registry
  │
  ▼
Interface returns formatted text to caller
  ├── MCP:  content[0].text = csvString
  ├── REST: {"result": csvString}
  └── CLI:  print to stdout
```

---

## File Map

```
cmd/elko/main.go              CLI root: cobra commands, source filtering, startup sequence

internal/
  registry/registry.go        Tool catalogue: Register, Call, List, RWMutex
  mcp/server.go               JSON-RPC 2.0 stdio: initialize, tools/list, tools/call, ping
  api/server.go               Chi router: /health, /v1/catalogue, /v1/call/{tool}, /v1/sources
  cache/cache.go              Two-layer cache: in-memory + optional SQLite file
  channel/
    spec.go                   Spec struct (name, schema, request, response, chart), LoadFS(), ParseTTL()
    runner.go                 Runner struct, makeFetch closure, RegisterExtractor, Register
    extract/
      all.go                  RegisterAll() → calls all Register* functions
      yahoo.go                3 extractors: quote, OHLCV history, dividends
      edgar.go                2 extractors: financials (income/balance/cashflow), company_info; CIK cache
      treasury.go             1 extractor: yield curve
      bls.go                  1 extractor: time series
      fdic.go                 2 extractors: institution search, bank financials
      worldbank.go            1 extractor: macro indicator

channels/                     Embedded JSON specs (10 files across 6 source directories)
web/                          Embedded frontend (index.html + 7 JS/CSS files)
```
