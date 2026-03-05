# elko-market-mcp

**Free financial market data for AI agents, CLIs, and REST clients.**

elko is a Go binary that exposes 10 financial data tools — equity quotes, OHLCV history, SEC filings, Treasury yields, BLS economic series, FDIC banking data, and World Bank macro indicators — through three simultaneous interfaces:

- **MCP** — plug into Claude, Cursor, or any MCP-compatible client for natural language data access
- **REST API + Web Dashboard** — a local server with a zero-dependency browser UI, SVG charts, and data export
- **CLI** — one-shot tool invocation from the terminal

All tools are free, require no API keys (except a contact email for SEC EDGAR), and cover a full macro + micro data stack.

---

## Data Sources

| Source | Tools | What You Get |
|--------|-------|-------------|
| **Yahoo Finance** | `yahoo_quote`, `yahoo_history`, `yahoo_dividends` | Live quotes, OHLCV bars (1m→1mo), dividend events — equities, ETFs, crypto, forex |
| **SEC EDGAR** | `edgar_financials`, `edgar_company_info` | Annual/quarterly income, balance sheet, cash flow (XBRL); company metadata |
| **US Treasury** | `treasury_yields` | Full yield curve (1mo→30y), any date range |
| **Bureau of Labor Statistics** | `bls_series` | CPI, PPI, unemployment rate, nonfarm payrolls — any BLS series ID |
| **FDIC** | `fdic_bank_search`, `fdic_bank_financials` | Find banks by name/state; get assets, deposits, equity, ROA, ROE, capital ratios |
| **World Bank** | `worldbank_indicator` | GDP, inflation, unemployment, population, debt — any country or global |

---

## Quick Start

### Build

```bash
git clone https://github.com/jsoprych/elko-market-mcp
cd elko-market-mcp
go build -o elko ./cmd/elko
```

### Set SEC user agent (required for EDGAR tools)

```bash
export SEC_USER_AGENT="MyApp me@example.com"
```

### Run as MCP server (Claude Code / Claude Desktop)

Add to your `.mcp.json`:

```json
{
  "mcpServers": {
    "elko-market": {
      "type": "stdio",
      "command": "/path/to/elko",
      "args": ["mcp"],
      "env": { "SEC_USER_AGENT": "MyApp me@example.com" }
    }
  }
}
```

Then ask Claude anything:

```
"What's the NVDA quote right now?"
"Show me AAPL's last 5 annual income statements"
"Is the yield curve inverted?"
"Find Wells Fargo in the FDIC database and pull their financials"
"Compare US vs China GDP growth over the last decade"
```

### Run as CLI

```bash
./elko call yahoo_quote symbol=NVDA
./elko call edgar_financials symbol=AAPL statement=income
./elko call treasury_yields latest=true
./elko call bls_series series_id=LNS14000000 start_year=2020
```

### Run as REST API + Web Dashboard

```bash
./elko serve --port 8080
# open http://localhost:8080
```

```bash
curl -s -XPOST localhost:8080/v1/call/yahoo_quote \
  -H 'Content-Type: application/json' \
  -d '{"symbol":"AAPL"}'
```

---

## Web Dashboard

The `serve` command starts a combined REST API and browser UI at `http://localhost:8080`.

```
┌─────────────────────────────────────────────────────────────────┐
│  elko                                         [◀] [▶] [↓ txt ▾]│
├──────────────────┬──────────────────────────────────────────────┤
│ ▼ yahoo          │  yahoo_history                               │
│   ○ quote        │                                              │
│   ○ history      │  symbol  [AAPL        ]                     │
│   ○ dividends    │  period  [1y          ]                      │
│ ▼ edgar          │  interval[1d          ]                      │
│   ○ financials   │                                              │
│   ○ company_info │  [ Run ]                                     │
│ ▶ treasury       │                                              │
│ ▶ bls            │  ╔═══════════════════════════════════╗      │
│ ▶ fdic           │  ║  Close Price — AAPL (1d)          ║      │
│ ▶ worldbank      │  ║  ▲                                 ║      │
│                  │  ║    ╱╲    ╱╲  ╱╲                  ║      │
│                  │  ║   ╱  ╲╱  ╲╱  ╲╱                  ║      │
│                  │  ║  ╱                                ║      │
│                  │  ╚═══════════════════════════════════╝      │
└──────────────────┴──────────────────────────────────────────────┘
```

**Features:**

- **Auto-generated forms** from JSON Schema — no static HTML
- **Pure-SVG charts** — line and bar charts with no external dependencies
- **Result history** — back/forward navigation (up to 50 results)
- **URL state** — `?tool=name&arg=val` is bookmarkable; auto-runs on load
- **Export** — download or copy results as TXT, CSV, or JSON
- **Table overflow** — rows > 100 collapse with a "Show all N rows" toggle

---

## All 10 Tools

```
yahoo_quote           Live stock/ETF/crypto/forex quote + metadata
yahoo_history         OHLCV price history (1m to 1mo intervals)
yahoo_dividends       Dividend event history

edgar_financials      Income, balance sheet, or cash flow (annual/quarterly)
edgar_company_info    CIK, SIC industry, state, fiscal year, filer category

treasury_yields       US Treasury yield curve (1mo → 30y)

bls_series            Any BLS economic series (CPI, unemployment, payrolls…)

fdic_bank_search      Find FDIC-insured banks by name or state
fdic_bank_financials  Bank assets, deposits, equity, ROA, ROE, capital ratios

worldbank_indicator   GDP, inflation, trade, debt, population — any country
```

```bash
# See all tools with full schemas
./elko catalogue
```

---

## Optional Features

### SQLite cache (persists across restarts)

```bash
./elko serve --port 8080 --db ~/.elko-cache.db
./elko --db ~/.elko-cache.db call yahoo_history symbol=AAPL period=1y
```

### Source filtering

```bash
# Only enable Yahoo Finance and SEC EDGAR
./elko serve --sources yahoo,edgar
./elko --sources bls,treasury call treasury_yields latest=true
```

### Docker

```bash
docker compose up
# Dashboard at http://localhost:8080
```

```yaml
# docker-compose.yml excerpt
services:
  elko:
    build: .
    ports: ["8080:8080"]
    volumes: ["elko-data:/data"]
    environment:
      SEC_USER_AGENT: "MyApp me@example.com"
```

---

## Architecture

```
┌─────────┐  ┌─────────┐  ┌─────────┐
│   MCP   │  │  REST   │  │   CLI   │    Three interfaces,
│  stdio  │  │   API   │  │  call   │    one registry
└────┬────┘  └────┬────┘  └────┬────┘
     │             │             │
     └─────────────┼─────────────┘
                   ▼
            ┌─────────────┐
            │  Registry   │   Unified tool catalogue
            │  (10 tools) │   name · schema · handler
            └──────┬──────┘
                   ▼
            ┌─────────────┐
            │   Runner    │   HTTP client · TTL cache
            │  + Channel  │   header merging · fetch
            └──────┬──────┘
                   ▼
         ┌─────────────────────┐
         │  Cache (L1 memory   │   Optional L2 SQLite
         │  + L2 SQLite file)  │   with TTL control
         └──────────┬──────────┘
                    ▼
    ┌──────────────────────────────┐
    │  External APIs               │
    │  Yahoo · EDGAR · Treasury    │
    │  BLS · FDIC · World Bank     │
    └──────────────────────────────┘
```

Each tool is declared in a **channel JSON spec** (name, schema, HTTP config, extractor). Go code handles only source-specific parsing — HTTP, caching, output formatting, and registration are centralized.

---

## Documentation

| Document | Description |
|----------|-------------|
| [docs/INDEX.md](docs/INDEX.md) | Full documentation index and table of contents |
| [docs/QUICKSTART.md](docs/QUICKSTART.md) | Build, install, and first run for all interfaces |
| [docs/TOOLS.md](docs/TOOLS.md) | Complete tool reference with arguments and examples |
| [docs/MCP-SETUP.md](docs/MCP-SETUP.md) | MCP configuration for Claude Code, Claude Desktop, Cursor |
| [docs/REST-API.md](docs/REST-API.md) | REST API endpoint reference |
| [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) | Channel pipeline, registry, cache, extractor design |
| [docs/CHANNELS.md](docs/CHANNELS.md) | How to add new data source channels |
| [docs/DOCKER.md](docs/DOCKER.md) | Docker and docker-compose deployment |
| [docs/HOW-TO.md](docs/HOW-TO.md) | Usage examples and workflow cookbook |
| [docs/PIPELINE-DESIGN.md](docs/PIPELINE-DESIGN.md) | Phase 2 SQLite-native architecture proposal |

---

## Requirements

- Go 1.21+ (for build)
- No API keys required
- `SEC_USER_AGENT` environment variable (your app name + contact email) — required by SEC policy for EDGAR tools
- Network access to Yahoo Finance, SEC, Treasury, BLS, FDIC, World Bank APIs

---

## License

Apache 2.0 — see [LICENSE](LICENSE) and [NOTICE](NOTICE).

Copyright 2026 John Soprych / Elko.AI

## Contact

Open an issue or discussion at [github.com/jsoprych/elko-market-mcp](https://github.com/jsoprych/elko-market-mcp)

---

## Author

**John Soprych** — Chief Scientist, Elko.AI
*with assistance from "TheDarkCodeFactory" AI Workflow* :)
