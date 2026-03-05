# Tool Reference

Complete reference for all 10 elko tools. Each tool is callable via MCP, CLI, and REST.

**CLI syntax:** `./elko call <tool> key=value ...`
**REST syntax:** `POST /v1/call/<tool>` with JSON body
**MCP:** ask your AI assistant naturally — it selects the tool and arguments

---

## Table of Contents

- [Yahoo Finance](#yahoo-finance)
  - [yahoo_quote](#yahoo_quote)
  - [yahoo_history](#yahoo_history)
  - [yahoo_dividends](#yahoo_dividends)
- [SEC EDGAR](#sec-edgar)
  - [edgar_financials](#edgar_financials)
  - [edgar_company_info](#edgar_company_info)
- [US Treasury](#us-treasury)
  - [treasury_yields](#treasury_yields)
- [Bureau of Labor Statistics](#bureau-of-labor-statistics)
  - [bls_series](#bls_series)
- [FDIC](#fdic)
  - [fdic_bank_search](#fdic_bank_search)
  - [fdic_bank_financials](#fdic_bank_financials)
- [World Bank](#world-bank)
  - [worldbank_indicator](#worldbank_indicator)
- [Interesting Workflows](#interesting-workflows)

---

## Yahoo Finance

No authentication required. Covers equities, ETFs, mutual funds, crypto (e.g. `BTC-USD`), forex (e.g. `EURUSD=X`), and futures.

---

### `yahoo_quote`

Live quote plus key metadata for any Yahoo Finance-listed symbol.

**Output format:** key-value sections

**Arguments:**

| Argument | Type | Required | Description |
|----------|------|----------|-------------|
| `symbol` | string | **yes** | Ticker symbol |

**CLI examples:**

```bash
./elko call yahoo_quote symbol=AAPL
./elko call yahoo_quote symbol=NVDA
./elko call yahoo_quote symbol=BRK-B        # Berkshire B shares
./elko call yahoo_quote symbol=BTC-USD      # Bitcoin
./elko call yahoo_quote symbol=EURUSD=X     # Euro/USD forex
./elko call yahoo_quote symbol=GLD          # Gold ETF
./elko call yahoo_quote symbol=^VIX         # VIX index
```

**REST:**

```bash
curl -s -XPOST localhost:8080/v1/call/yahoo_quote \
  -H 'Content-Type: application/json' \
  -d '{"symbol": "TSLA"}'
```

**Sample output:**

```
Name:           Apple Inc.
Symbol:         AAPL
Exchange:       NASDAQ
Currency:       USD
Price:          189.30
Previous Close: 188.44
Day Range:      187.83 – 190.54
52W Range:      164.08 – 199.62
Volume:         54,832,100
Avg Volume:     58,241,300
Market Cap:     2.91T
```

---

### `yahoo_history`

OHLCV (Open, High, Low, Close, Adjusted Close, Volume) price history. Supports all intervals from 1-minute to monthly bars.

**Output format:** CSV — `Timestamp,Date,Open,High,Low,Close,AdjClose,Volume`

**Arguments:**

| Argument | Type | Required | Default | Description |
|----------|------|----------|---------|-------------|
| `symbol` | string | **yes** | — | Ticker symbol |
| `period` | string | no | `1mo` | Rolling window: `1d 5d 1mo 3mo 6mo 1y 2y 5y 10y ytd max` |
| `from` | string | no | — | Start date `YYYY-MM-DD`. Overrides `period` |
| `to` | string | no | today | End date `YYYY-MM-DD` |
| `interval` | string | no | `1d` | Bar size: `1m 5m 15m 30m 1h 1d 1wk 1mo` |

> Note: intraday intervals (`1m`–`1h`) are limited to recent history by Yahoo Finance.

**CLI examples:**

```bash
# Last month of daily bars (default)
./elko call yahoo_history symbol=AAPL

# 5 years of weekly bars
./elko call yahoo_history symbol=NVDA period=5y interval=1wk

# Specific date range, daily
./elko call yahoo_history symbol=SPY from=2024-01-01 to=2024-12-31

# Bitcoin last 3 months, hourly
./elko call yahoo_history symbol=BTC-USD period=3mo interval=1h

# Max available history for an ETF
./elko call yahoo_history symbol=QQQ period=max interval=1mo

# Pipe to file
./elko call yahoo_history symbol=AAPL period=5y interval=1d > aapl_5y.csv
```

**REST:**

```bash
curl -s -XPOST localhost:8080/v1/call/yahoo_history \
  -H 'Content-Type: application/json' \
  -d '{"symbol":"AAPL","period":"1mo","interval":"1d"}'
```

**Sample output:**

```
Timestamp,Date,Open,High,Low,Close,AdjClose,Volume
1706745600,2024-02-01,186.22,186.74,183.26,186.86,186.34,43,870,300
1706832000,2024-02-02,183.99,185.04,182.20,185.04,184.52,50,120,100
...
```

---

### `yahoo_dividends`

Historical dividend payment events for any dividend-paying symbol.

**Output format:** CSV — `Date,Amount`

**Arguments:**

| Argument | Type | Required | Default | Description |
|----------|------|----------|---------|-------------|
| `symbol` | string | **yes** | — | Ticker symbol |
| `from` | string | no | all history | Start date `YYYY-MM-DD` |
| `to` | string | no | today | End date `YYYY-MM-DD` |

**CLI examples:**

```bash
# All Coca-Cola dividends
./elko call yahoo_dividends symbol=KO

# Johnson & Johnson since 2020
./elko call yahoo_dividends symbol=JNJ from=2020-01-01

# Realty Income (monthly dividend payer)
./elko call yahoo_dividends symbol=O from=2023-01-01

# Microsoft dividends for a specific year
./elko call yahoo_dividends symbol=MSFT from=2023-01-01 to=2023-12-31
```

**Sample output:**

```
Date,Amount
2024-09-06,0.25
2024-06-07,0.25
2024-03-01,0.24
2023-12-01,0.24
...
```

---

## SEC EDGAR

Requires `SEC_USER_AGENT` environment variable per [SEC developer policy](https://www.sec.gov/developer). US-listed companies only.

---

### `edgar_financials`

Annual (10-K) or quarterly (10-Q) financial statements from SEC EDGAR XBRL data. Covers income statement, balance sheet, and cash flow statement.

**Output format:** fixed-width table, values in $B

**Arguments:**

| Argument | Type | Required | Default | Options |
|----------|------|----------|---------|---------|
| `symbol` | string | **yes** | — | US ticker |
| `statement` | string | no | `income` | `income` `balance` `cashflow` |
| `frequency` | string | no | `annual` | `annual` (10-K), `quarterly` (10-Q) |

**CLI examples:**

```bash
# Annual income statement (default)
./elko call edgar_financials symbol=AAPL

# Quarterly balance sheet
./elko call edgar_financials symbol=MSFT statement=balance frequency=quarterly

# Annual cash flow
./elko call edgar_financials symbol=AMZN statement=cashflow

# Quarterly income — recent 8 quarters
./elko call edgar_financials symbol=GOOGL statement=income frequency=quarterly

# Bank balance sheet
./elko call edgar_financials symbol=JPM statement=balance
```

**REST:**

```bash
curl -s -XPOST localhost:8080/v1/call/edgar_financials \
  -H 'Content-Type: application/json' \
  -d '{"symbol":"AAPL","statement":"income","frequency":"annual"}'
```

**Sample output (income statement):**

```
AAPL — Income Statement (Annual, $B)

Period       Revenue   Gross Profit   Op. Income   Net Income   EPS (dil)
2024         391.04       173.20        123.22       100.39       6.57
2023         383.29       169.15        114.30        96.99       6.13
2022         394.33       170.78        119.44        99.80       6.11
2021         365.82       152.84        108.95        94.68       5.61
2020         274.51       104.96         66.29        57.41       3.28
```

---

### `edgar_company_info`

Company metadata from SEC EDGAR: CIK number, industry classification, incorporation state, fiscal year end, and filer category.

**Output format:** key-value

**Arguments:**

| Argument | Type | Required | Description |
|----------|------|----------|-------------|
| `symbol` | string | **yes** | US ticker |

**CLI examples:**

```bash
./elko call edgar_company_info symbol=AAPL
./elko call edgar_company_info symbol=TSLA
./elko call edgar_company_info symbol=JPM
./elko call edgar_company_info symbol=WMT
```

**Sample output:**

```
Name:           Apple Inc.
CIK:            0000320193
SIC:            3674
SIC Desc:       Semiconductors and Related Devices
State:          CA
FY End:         09
Filer Category: Large Accelerated Filer
```

---

## US Treasury

---

### `treasury_yields`

US Treasury average interest rates from [fiscaldata.treasury.gov](https://fiscaldata.treasury.gov). Returns rates by security type across the full maturity spectrum.

**Output format:** table — Date, Security, Rate (%)

**Arguments:**

| Argument | Type | Required | Default | Description |
|----------|------|----------|---------|-------------|
| `from` | string | no | 30 days ago | Start date `YYYY-MM-DD` |
| `to` | string | no | today | End date `YYYY-MM-DD` |
| `latest` | boolean | no | false | Return only the most recent available date |

**CLI examples:**

```bash
# Most recent yield curve snapshot
./elko call treasury_yields latest=true

# Last 30 days (default)
./elko call treasury_yields

# 2024 full year
./elko call treasury_yields from=2024-01-01 to=2024-12-31

# Compare yield curve before and after rate hike cycle
./elko call treasury_yields from=2022-01-01 to=2022-12-31
```

**Sample output:**

```
record_date    security_desc               avg_interest_rate_amt
2024-02-29     Treasury Bills              5.41
2024-02-29     Treasury Notes              4.18
2024-02-29     Treasury Bonds              4.36
2024-02-29     Treasury Inflation-Protected Securities  2.12
...
```

---

## Bureau of Labor Statistics

---

### `bls_series`

Any BLS public time series by series ID. Covers hundreds of economic indicators including all CPI variants, employment data, wage data, and producer prices.

**Output format:** table — Year, Period, Value

**Arguments:**

| Argument | Type | Required | Default | Description |
|----------|------|----------|---------|-------------|
| `series_id` | string | **yes** | — | BLS series ID (see below) |
| `start_year` | string | no | 5 years ago | `YYYY` |
| `end_year` | string | no | current year | `YYYY` |

**Well-known series IDs:**

| Series ID | Name |
|-----------|------|
| `CUUR0000SA0` | CPI-U All Items (not seasonally adjusted) |
| `CUSR0000SA0` | CPI-U All Items (seasonally adjusted) |
| `CUUR0000SA0L1E` | CPI-U Less Food and Energy (core CPI, NSA) |
| `LNS14000000` | Unemployment Rate (seasonally adjusted) |
| `CES0000000001` | Total Nonfarm Payrolls (seasonally adjusted) |
| `WPUFD49104` | PPI Final Demand |
| `CES0500000003` | Average Hourly Earnings, Private |

Find more at [bls.gov/data](https://www.bls.gov/data/).

**CLI examples:**

```bash
# CPI last 5 years (default window)
./elko call bls_series series_id=CUUR0000SA0

# Unemployment rate 2018–2025
./elko call bls_series series_id=LNS14000000 start_year=2018 end_year=2025

# Nonfarm payrolls since COVID
./elko call bls_series series_id=CES0000000001 start_year=2020

# Core CPI
./elko call bls_series series_id=CUUR0000SA0L1E start_year=2021

# PPI
./elko call bls_series series_id=WPUFD49104 start_year=2020
```

**Sample output:**

```
Year   Period   Value
2024   M01      308.417
2024   M02      308.901
2024   M03      309.685
...
```

---

## FDIC

---

### `fdic_bank_search`

Search FDIC-insured depository institutions by name and/or state. Returns cert number, name, location, status, and asset size. The **cert number** is used with `fdic_bank_financials`.

**Output format:** table

**Arguments:**

| Argument | Type | Required | Default | Description |
|----------|------|----------|---------|-------------|
| `name` | string | no | — | Partial name match |
| `state` | string | no | — | Two-letter state code (e.g. `NY`, `TX`) |
| `limit` | integer | no | 10 | Max results (1–50) |

> At least one of `name` or `state` should be provided.

**CLI examples:**

```bash
# Find Wells Fargo
./elko call fdic_bank_search name="Wells Fargo"

# All active banks in Vermont
./elko call fdic_bank_search state=VT limit=50

# Community banks in Texas
./elko call fdic_bank_search state=TX limit=20

# Find by partial name
./elko call fdic_bank_search name="Silicon Valley"

# JP Morgan entities
./elko call fdic_bank_search name="JPMorgan"
```

**Sample output:**

```
Cert    Name                        City/State        Active   Assets ($M)
628     JPMorgan Chase Bank, NA     Columbus, OH      Yes      3,394,601
3510    Bank of America, NA         Charlotte, NC     Yes      2,541,398
3511    Wells Fargo Bank, NA        Sioux Falls, SD   Yes      1,743,420
```

---

### `fdic_bank_financials`

Detailed financial data for a specific FDIC-insured institution by certificate number. Get the cert from `fdic_bank_search`.

**Output format:** table — assets, deposits, loans, equity, net income, ROA, ROE, capital ratios

**Arguments:**

| Argument | Type | Required | Default | Description |
|----------|------|----------|---------|-------------|
| `cert` | string | **yes** | — | FDIC certificate number |
| `year` | integer | no | all available | Filter to a specific report year |

**CLI examples:**

```bash
# Step 1: find the cert
./elko call fdic_bank_search name="JPMorgan Chase"
# → cert 628

# Step 2: pull financials
./elko call fdic_bank_financials cert=628

# Specific year
./elko call fdic_bank_financials cert=628 year=2022

# Wells Fargo
./elko call fdic_bank_financials cert=3511

# Bank of America
./elko call fdic_bank_financials cert=3510

# Regional bank
./elko call fdic_bank_search name="Signature Bank"
./elko call fdic_bank_financials cert=57803 year=2022
```

**Sample output:**

```
repdte     asset      dep        lnlsnet    eq         netinc     roa     roe     rbcrwaj
20231231   3394601    2387156    1312489    327894     50241      1.48    15.33   15.81
20221231   3201942    2480073    1130498    292011     37676      1.18    12.90   15.43
...
```

---

## World Bank

---

### `worldbank_indicator`

World Bank Open Data macroeconomic indicators for any country or globally. Covers hundreds of development indicators from 1960 to present.

**Output format:** table — Year, Value

**Arguments:**

| Argument | Type | Required | Default | Description |
|----------|------|----------|---------|-------------|
| `country` | string | **yes** | — | ISO 2-letter (`US`), 3-letter (`CHN`), or `all` |
| `indicator` | string | **yes** | — | World Bank indicator code |
| `from_year` | integer | no | 10 years ago | Start year |
| `to_year` | integer | no | current year | End year |

**Common indicator codes:**

| Code | Description |
|------|-------------|
| `NY.GDP.MKTP.CD` | GDP (current US$) |
| `NY.GDP.PCAP.CD` | GDP per capita (current US$) |
| `NY.GDP.MKTP.KD.ZG` | GDP growth (annual %) |
| `FP.CPI.TOTL.ZG` | Inflation, consumer prices (annual %) |
| `SL.UEM.TOTL.ZS` | Unemployment, total (% of labor force) |
| `SP.POP.TOTL` | Population, total |
| `NE.TRD.GNFS.ZS` | Trade (% of GDP) |
| `GC.DOD.TOTL.GD.ZS` | Central government debt (% of GDP) |

Find more at [data.worldbank.org/indicator](https://data.worldbank.org/indicator).

**CLI examples:**

```bash
# US GDP
./elko call worldbank_indicator country=US indicator=NY.GDP.MKTP.CD

# China GDP growth since 2000
./elko call worldbank_indicator country=CN indicator=NY.GDP.MKTP.KD.ZG from_year=2000

# Germany inflation
./elko call worldbank_indicator country=DE indicator=FP.CPI.TOTL.ZG

# Global unemployment
./elko call worldbank_indicator country=all indicator=SL.UEM.TOTL.ZS to_year=2023

# Japan population
./elko call worldbank_indicator country=JP indicator=SP.POP.TOTL

# EU government debt
./elko call worldbank_indicator country=EU indicator=GC.DOD.TOTL.GD.ZS from_year=2010

# US vs China GDP comparison (run separately)
./elko call worldbank_indicator country=US indicator=NY.GDP.MKTP.CD
./elko call worldbank_indicator country=CN indicator=NY.GDP.MKTP.CD
```

**Sample output:**

```
Year   Value
2023   27,357,600,000,000
2022   25,462,700,000,000
2021   23,315,100,000,000
...
```

---

## Interesting Workflows

### Earnings deep dive

```bash
./elko call yahoo_quote         symbol=AAPL
./elko call edgar_company_info  symbol=AAPL
./elko call edgar_financials    symbol=AAPL statement=income
./elko call edgar_financials    symbol=AAPL statement=balance
./elko call edgar_financials    symbol=AAPL statement=cashflow
./elko call yahoo_history       symbol=AAPL period=5y interval=1mo
```

### Macro context for a trade

```bash
./elko call treasury_yields     latest=true
./elko call bls_series          series_id=CUUR0000SA0         # CPI
./elko call bls_series          series_id=LNS14000000         # Unemployment
./elko call bls_series          series_id=CES0000000001       # Payrolls
./elko call worldbank_indicator country=US indicator=NY.GDP.MKTP.KD.ZG
```

### Banking sector check

```bash
./elko call fdic_bank_search    name="JPMorgan Chase"
./elko call fdic_bank_financials cert=628
./elko call yahoo_quote         symbol=JPM
./elko call edgar_financials    symbol=JPM statement=balance
```

### Crypto + macro correlation

```bash
./elko call yahoo_history symbol=BTC-USD period=3y interval=1wk
./elko call bls_series    series_id=CUUR0000SA0 start_year=2022
./elko call treasury_yields from=2022-01-01
```

### International comparison

```bash
# GDP growth comparison
for country in US CN DE JP GB IN; do
  echo "--- $country ---"
  ./elko call worldbank_indicator country=$country indicator=NY.GDP.MKTP.KD.ZG from_year=2015
done
```

### Ask Claude (MCP prompts)

With the MCP server running, paste these into Claude:

```
"Pull NVDA's 5-year price history and annual income statements. Calculate revenue CAGR and P/S ratio trend."

"Is the US yield curve currently inverted? Pull treasury yields and explain what that historically signals."

"Find community banks in Vermont with assets under $1B and compare their ROA."

"Get BTC-USD weekly bars for 3 years, CPI monthly since 2021, and 10-year Treasury yields. What's the correlation?"

"Show me Microsoft's quarterly revenue for the last 3 years and plot the trend."
```
