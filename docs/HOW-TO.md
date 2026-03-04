# elko-market-mcp: How To Use It

Three ways to call tools: **ask Claude** (MCP), **CLI one-shot**, or **REST API**.

---

## 1. Ask Claude (MCP — you're doing this right now)

The server is already wired into this Claude Code session. Just ask naturally.
Claude calls the tool, gets real live data, and reasons over it.

```
"What's the current NVDA quote?"
"Show me AAPL's income statement for the last 3 years"
"Get 5-year price history for BTC-USD"
"What are current US Treasury yields?"
"Search for Wells Fargo in the FDIC database"
```

Claude picks the right tool and arguments automatically.

---

## 2. CLI One-Shot

```bash
cd /home/john/CODE/PROJECTS/LATEST/2026/MARKETMCP/elko-market-mcp
./elko call <tool_name> key=value key=value ...
```

---

## 3. REST API

```bash
./elko serve --port 8080 &

curl -s -XPOST localhost:8080/v1/call/<tool_name> \
  -H 'Content-Type: application/json' \
  -d '{"key": "value"}'
```

---

## 4. Web Dashboard

Start the server and open a browser — a dynamic UI is served at the root:

```bash
./elko serve --port 8080
# open http://localhost:8080
```

The dashboard is entirely generated from the JSON Schema of each tool:
- **Sidebar** — collapsible tree: source → category → tool
- **Form** — generated per-tool from `schema.properties`: text inputs, `<select>` for enum fields, checkboxes for booleans, number inputs with min/max, datalist autocomplete where `examples` are defined
- **Result panel** — rendered as HTML tables (`csv`, `table`, `kv`, `sections` formats) or `<pre>` fallback
- **Toolbar** — back/forward history (up to 50 results), copy-to-clipboard button
- **URL state** — `?tool=name&arg=val` is bookmarkable and auto-runs on load when all required args are present
- **Row overflow** — tables > 100 rows collapse with a "Show all N rows" toggle

No static HTML beyond the 5-line index shell. All structure is derived from the catalogue at `/v1/catalogue`.

---

## Tool Reference & Example Calls

---

### `yahoo_history` — OHLCV Price History

**Arguments:**
| Arg | Required | Example | Notes |
|-----|----------|---------|-------|
| `symbol` | ✅ | `AAPL` | Any ticker. Crypto: `BTC-USD`, `ETH-USD` |
| `period` | — | `1mo` | `1d 5d 1mo 3mo 6mo 1y 2y 5y 10y ytd max` |
| `from` | — | `2024-01-01` | YYYY-MM-DD. Overrides period. |
| `to` | — | `2024-12-31` | YYYY-MM-DD. Defaults to today. |
| `interval` | — | `1d` | `1m 5m 15m 30m 1h 1d 1wk 1mo` |

**CLI examples:**
```bash
# Last month of daily bars
./elko call yahoo_history symbol=AAPL period=1mo

# 5 years of weekly bars
./elko call yahoo_history symbol=NVDA period=5y interval=1wk

# Specific date range
./elko call yahoo_history symbol=SPY from=2024-01-01 to=2024-12-31

# Bitcoin hourly
./elko call yahoo_history symbol=BTC-USD period=5d interval=1h
```

**REST:**
```bash
curl -s -XPOST localhost:8080/v1/call/yahoo_history \
  -H 'Content-Type: application/json' \
  -d '{"symbol":"AAPL","period":"1mo","interval":"1d"}'
```

**Output:** CSV — `Timestamp,Date,Open,High,Low,Close,AdjClose,Volume`

---

### `yahoo_quote` — Live Quote + Metadata

**Arguments:**
| Arg | Required | Example |
|-----|----------|---------|
| `symbol` | ✅ | `TSLA` |

**CLI examples:**
```bash
./elko call yahoo_quote symbol=NVDA
./elko call yahoo_quote symbol=BRK-B
./elko call yahoo_quote symbol=GLD       # Gold ETF
./elko call yahoo_quote symbol=EURUSD=X  # Forex
```

**Output:** Price, day/52W range, volume, market cap, exchange, currency.

---

### `yahoo_dividends` — Dividend History

**Arguments:**
| Arg | Required | Example |
|-----|----------|---------|
| `symbol` | ✅ | `KO` |
| `from` | — | `2020-01-01` |
| `to` | — | `2024-12-31` |

**CLI examples:**
```bash
./elko call yahoo_dividends symbol=KO
./elko call yahoo_dividends symbol=JNJ from=2020-01-01
```

**Output:** CSV — `Date,Amount`

---

### `edgar_financials` — SEC XBRL Financial Statements

**Arguments:**
| Arg | Required | Example | Notes |
|-----|----------|---------|-------|
| `symbol` | ✅ | `MSFT` | US-listed only |
| `statement` | — | `income` | `income` `balance` `cashflow` |
| `frequency` | — | `annual` | `annual` (10-K) or `quarterly` (10-Q) |

**CLI examples:**
```bash
# Annual income statement
./elko call edgar_financials symbol=AAPL

# Quarterly balance sheet
./elko call edgar_financials symbol=MSFT statement=balance frequency=quarterly

# Cash flow statement
./elko call edgar_financials symbol=AMZN statement=cashflow

# Last 5 years annual
./elko call edgar_financials symbol=GOOGL statement=income frequency=annual
```

**Output:** Table — Revenue, Gross Profit, Operating Income, Net Income, EPS (in $B).

---

### `edgar_company_info` — SEC Company Metadata

**Arguments:**
| Arg | Required | Example |
|-----|----------|---------|
| `symbol` | ✅ | `JPM` |

**CLI examples:**
```bash
./elko call edgar_company_info symbol=TSLA
./elko call edgar_company_info symbol=JPM
./elko call edgar_company_info symbol=WMT
```

**Output:** Name, CIK, SIC industry code, state of incorporation, fiscal year end, filer category.

---

### `treasury_yields` — US Treasury Yield Curve

**Arguments:**
| Arg | Required | Example | Notes |
|-----|----------|---------|-------|
| `from` | — | `2024-01-01` | Default: 30 days ago |
| `to` | — | `2024-12-31` | Default: today |
| `latest` | — | `true` | Return only the most recent date |

**CLI examples:**
```bash
# Most recent yield curve snapshot
./elko call treasury_yields latest=true

# Last 30 days (default)
./elko call treasury_yields

# Specific period
./elko call treasury_yields from=2024-01-01 to=2024-03-31
```

**Output:** Table — Date, Security (1mo through 30y), Rate (%).

---

### `bls_series` — Bureau of Labor Statistics

**Arguments:**
| Arg | Required | Example | Notes |
|-----|----------|---------|-------|
| `series_id` | ✅ | `CUUR0000SA0` | BLS series ID |
| `start_year` | — | `2020` | Default: 5 years ago |
| `end_year` | — | `2024` | Default: current year |

**Well-known series IDs:**
| Series ID | What It Is |
|-----------|-----------|
| `CUUR0000SA0` | CPI-U All Items (not seasonally adjusted) |
| `CUSR0000SA0` | CPI-U All Items (seasonally adjusted) |
| `LNS14000000` | Unemployment Rate (SA) |
| `CES0000000001` | Total Nonfarm Payrolls |
| `WPUFD49104` | PPI Final Demand |

**CLI examples:**
```bash
# CPI last 5 years
./elko call bls_series series_id=CUUR0000SA0

# Unemployment rate 2018–2024
./elko call bls_series series_id=LNS14000000 start_year=2018 end_year=2024

# Nonfarm payrolls
./elko call bls_series series_id=CES0000000001 start_year=2020
```

**Output:** Table — Year, Period (M01–M12), Value.

---

### `worldbank_indicator` — World Bank Macro Data

**Arguments:**
| Arg | Required | Example | Notes |
|-----|----------|---------|-------|
| `country` | ✅ | `US` | ISO 2/3-letter code or `all` |
| `indicator` | ✅ | `NY.GDP.MKTP.CD` | World Bank indicator code |
| `from_year` | — | `2010` | Default: 10 years ago |
| `to_year` | — | `2024` | Default: current year |

**Common indicators:**
| Code | Meaning |
|------|---------|
| `NY.GDP.MKTP.CD` | GDP (current US$) |
| `NY.GDP.MKTP.KD.ZG` | GDP growth (annual %) |
| `FP.CPI.TOTL.ZG` | Inflation, consumer prices (annual %) |
| `SL.UEM.TOTL.ZS` | Unemployment rate |
| `SP.POP.TOTL` | Population, total |
| `GC.DOD.TOTL.GD.ZS` | Government debt (% of GDP) |

**CLI examples:**
```bash
# US GDP
./elko call worldbank_indicator country=US indicator=NY.GDP.MKTP.CD

# China vs US GDP growth (run twice with different country)
./elko call worldbank_indicator country=CN indicator=NY.GDP.MKTP.KD.ZG from_year=2000

# Global inflation
./elko call worldbank_indicator country=all indicator=FP.CPI.TOTL.ZG to_year=2023

# EU unemployment
./elko call worldbank_indicator country=EU indicator=SL.UEM.TOTL.ZS
```

**Output:** Table — Year, Value.

---

### `fdic_bank_search` — Find FDIC-Insured Banks

**Arguments:**
| Arg | Required | Example | Notes |
|-----|----------|---------|-------|
| `name` | — | `Wells Fargo` | Partial match |
| `state` | — | `NY` | Two-letter state code |
| `limit` | — | `10` | Max 50 |

**CLI examples:**
```bash
# Search by name
./elko call fdic_bank_search name="Wells Fargo"

# Banks in Texas
./elko call fdic_bank_search state=TX limit=20

# Community banks in Vermont
./elko call fdic_bank_search state=VT limit=50

# Find a specific bank
./elko call fdic_bank_search name="Silicon Valley Bank"
```

**Output:** Table — Cert#, Name, City/State, Active, Assets ($M).
The **Cert** number is what you pass to `fdic_bank_financials`.

---

### `fdic_bank_financials` — Bank Financial Data

**Arguments:**
| Arg | Required | Example | Notes |
|-----|----------|---------|-------|
| `cert` | ✅ | `57803` | FDIC certificate number from `fdic_bank_search` |
| `year` | — | `2023` | Filter to a specific year |

**CLI examples:**
```bash
# First: find the cert number
./elko call fdic_bank_search name="JPMorgan Chase"
# Note the cert number from output, then:
./elko call fdic_bank_financials cert=628

# Wells Fargo
./elko call fdic_bank_financials cert=3511

# Specific year
./elko call fdic_bank_financials cert=628 year=2022
```

**Output:** Assets, Deposits, Net Loans, Equity, Net Income, ROA, ROE, Tier 1 Capital per period.

---

## Interesting Workflows

### "Earnings season" deep dive
```bash
./elko call yahoo_quote         symbol=AAPL
./elko call edgar_financials    symbol=AAPL statement=income
./elko call edgar_financials    symbol=AAPL statement=cashflow
./elko call edgar_company_info  symbol=AAPL
```

### Macro context for a trade
```bash
./elko call treasury_yields     latest=true
./elko call bls_series          series_id=CUUR0000SA0        # CPI
./elko call bls_series          series_id=LNS14000000        # Unemployment
./elko call worldbank_indicator country=US indicator=NY.GDP.MKTP.KD.ZG
```

### Banking sector check
```bash
./elko call fdic_bank_search    name="Bank of America"
./elko call fdic_bank_financials cert=3510                   # BofA cert
./elko call yahoo_quote         symbol=BAC
./elko call edgar_financials    symbol=BAC statement=balance
```

### Crypto + macro
```bash
./elko call yahoo_history symbol=BTC-USD period=1y interval=1wk
./elko call bls_series    series_id=CUUR0000SA0 start_year=2021
./elko call treasury_yields from=2021-01-01
```

---

## Ask Claude — example prompts

Since the MCP server is live in this session, just paste any of these:

```
"Get the current quote for NVDA and tell me if it's near its 52-week high or low"

"Pull AAPL's last 5 annual income statements and calculate the revenue CAGR"

"Show me the current Treasury yield curve and tell me if it's inverted"

"Search for community banks in Vermont and show me the financials for the largest one"

"Get BTC-USD weekly price history for the last 2 years"

"Compare US vs China GDP growth over the last decade"

"What's the unemployment trend since 2020? Use BLS data."

"Pull MSFT's balance sheet and calculate their debt-to-equity ratio"
```

---

## Quick Reference

```bash
# See all 10 tools
./elko catalogue

# Get help
./elko --help
./elko call --help

# Enable SQLite cache (survives restarts)
./elko --db ~/.elko-cache.db call yahoo_history symbol=AAPL period=1y

# Enable specific sources only
./elko --sources yahoo,edgar call yahoo_quote symbol=TSLA

# Start REST server + web dashboard
./elko serve --port 8080
# open http://localhost:8080

# Health check
curl localhost:8080/health

# Full catalogue via REST
curl localhost:8080/v1/catalogue | jq '.tools[].name'
```
