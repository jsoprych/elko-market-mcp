# elko-market-mcp — API Keys & Credentials

Complete list of environment variables, API keys, and registration requirements
for every current and planned elko channel.

---

## Quick Reference

| Env Var | Channel(s) | Required? | Cost | Registration URL |
|---------|-----------|-----------|------|-----------------|
| `FRED_API_KEY` | `fred_series`, `fred_search` | **Yes** | Free | https://fred.stlouisfed.org/docs/api/api_key.html |
| `SEC_USER_AGENT` | `edgar_*` | Recommended | Free | n/a — see note |
| *(none)* | `yahoo_*` | No | Free | n/a |
| *(none)* | `treasury_yields` | No | Free | n/a |
| *(none)* | `bls_series` | No | Free (rate-limited) | n/a |
| *(none)* | `fdic_*` | No | Free | n/a |
| *(none)* | `worldbank_indicator` | No | Free | n/a |

---

## Current Channels

### FRED — Federal Reserve Economic Data

| Item | Detail |
|------|--------|
| **Env var** | `FRED_API_KEY` |
| **Required** | Yes — FRED API now requires a key for all requests |
| **Cost** | Free — personal or commercial use |
| **Rate limit (no key)** | Blocked |
| **Rate limit (with key)** | 120 requests/minute |
| **Registration** | https://fred.stlouisfed.org/docs/api/api_key.html |
| **Steps** | 1. Create account at stlouisfed.org → 2. Request API key → 3. Key emailed within minutes |
| **Affects** | `fred_series`, `fred_search` |

```bash
export FRED_API_KEY="your_key_here"
```

---

### SEC EDGAR

| Item | Detail |
|------|--------|
| **Env var** | `SEC_USER_AGENT` |
| **Required** | No — but **strongly recommended** to avoid rate-limiting |
| **Cost** | Free — no registration |
| **Required format** | `Company Name contact@yourdomain.com` |
| **Rate limit** | 10 requests/second per IP; undeclared bots get blocked |
| **Policy** | https://www.sec.gov/developer |
| **Affects** | `edgar_financials`, `edgar_company_info`, `edgar_insider_trades` |

The SEC requires automated tools to identify themselves. The default User-Agent
(`elko-market-mcp/1.0 contact@localhost`) satisfies the format requirement but
you should set your own for production use.

```bash
export SEC_USER_AGENT="Acme Corp dev@acmecorp.com"
```

---

### Yahoo Finance

| Item | Detail |
|------|--------|
| **Env var** | *(none)* |
| **Required** | No |
| **Cost** | Free — unofficial public API |
| **Rate limit** | Undocumented; ~100 req/min per IP is safe |
| **Stability** | No SLA; Yahoo occasionally changes endpoints without notice |
| **Note** | Integration test will catch breakage |
| **Affects** | `yahoo_quote`, `yahoo_history`, `yahoo_dividends` |

---

### US Treasury (fiscaldata.treasury.gov)

| Item | Detail |
|------|--------|
| **Env var** | *(none)* |
| **Required** | No |
| **Cost** | Free |
| **Rate limit** | None documented; public government API |
| **Affects** | `treasury_yields` |

---

### Bureau of Labor Statistics (BLS)

| Item | Detail |
|------|--------|
| **Env var** | *(none currently; `BLS_API_KEY` planned)* |
| **Required** | No — unauthenticated v1 API used |
| **Cost** | Free |
| **Rate limit (no key)** | 25 requests/day, 50 series/query, 10 years of data |
| **Rate limit (with key)** | 500 requests/day, 50 series/query, 20 years of data |
| **Registration** | https://data.bls.gov/registrationEngine/ |
| **Note** | Current implementation uses v1 (no key). Adding `BLS_API_KEY` support will unlock v2 and lift the daily cap. |
| **Affects** | `bls_series` |

```bash
# Not yet wired — coming soon
export BLS_API_KEY="your_key_here"
```

---

### FDIC (api.fdic.gov)

| Item | Detail |
|------|--------|
| **Env var** | *(none)* |
| **Required** | No |
| **Cost** | Free |
| **Rate limit** | None documented; public government API |
| **Affects** | `fdic_bank_search`, `fdic_bank_financials` |

---

### World Bank

| Item | Detail |
|------|--------|
| **Env var** | *(none)* |
| **Required** | No |
| **Cost** | Free |
| **Rate limit** | None documented; public API |
| **Note** | Responses can be slow (2–20s) for large country/year ranges |
| **Affects** | `worldbank_indicator` |

---

## Planned Channels — Key Requirements

### Congress Stock Trades (STOCK Act disclosures)

| Item | Detail |
|------|--------|
| **Env var** | *(none planned)* |
| **Source** | Community-maintained JSON from official House/Senate disclosures |
| **House data** | https://house-stock-watcher-data.s3-us-west-2.amazonaws.com/data/all_transactions.json |
| **Senate data** | https://senate-stock-watcher-data.s3-us-west-2.amazonaws.com/aggregate/all_transactions.json |
| **Cost** | Free |
| **Maintainer** | Timothy Carambat (open source project) |
| **Note** | No API key needed. If the community project goes dark, fall back to parsing https://disclosures.house.gov directly. |

---

### Federal Election Commission (FEC) — Political Contributions

| Item | Detail |
|------|--------|
| **Env var** | `FEC_API_KEY` *(planned)* |
| **Required** | No — demo key `DEMO_KEY` works for light use |
| **Cost** | Free |
| **Rate limit (demo key)** | 30 req/hr, 50 req/day |
| **Rate limit (registered)** | 1,000 req/hr |
| **Registration** | https://api.open.fec.gov/developers/ (uses api.data.gov key) |
| **Note** | Same key works for many US government APIs (Census, NASA, etc.) |

```bash
export FEC_API_KEY="your_api_data_gov_key"
```

---

### CoinGecko — Crypto Market Data

| Item | Detail |
|------|--------|
| **Env var** | `COINGECKO_API_KEY` *(planned; optional)* |
| **Required** | No — public API has a free tier without a key |
| **Cost** | Free tier available; Pro plans from $129/month |
| **Rate limit (no key)** | 10–30 calls/minute (varies) |
| **Rate limit (with free key)** | 30 calls/minute |
| **Registration** | https://www.coingecko.com/en/api |

```bash
export COINGECKO_API_KEY="CG-your_key_here"
```

---

### Open-Meteo — Weather

| Item | Detail |
|------|--------|
| **Env var** | *(none planned)* |
| **Required** | No — completely free, no key, no registration |
| **Cost** | Free for non-commercial; commercial use requires subscription |
| **Rate limit** | 10,000 requests/day |
| **API docs** | https://open-meteo.com/en/docs |

---

### Alpha Vantage — Stocks / Forex / Crypto

| Item | Detail |
|------|--------|
| **Env var** | `ALPHA_VANTAGE_API_KEY` *(planned)* |
| **Required** | Yes |
| **Cost** | Free tier: 25 requests/day. Premium from $50/month |
| **Registration** | https://www.alphavantage.co/support/#api-key |

---

### OECD Statistics

| Item | Detail |
|------|--------|
| **Env var** | *(none planned)* |
| **Required** | No |
| **Cost** | Free |
| **API docs** | https://data.oecd.org/api/ |

---

### IMF Data

| Item | Detail |
|------|--------|
| **Env var** | *(none planned)* |
| **Required** | No |
| **Cost** | Free |
| **API docs** | https://datahelp.imf.org/knowledgebase/articles/667681 |

---

### ECB (European Central Bank)

| Item | Detail |
|------|--------|
| **Env var** | *(none planned)* |
| **Required** | No |
| **Cost** | Free |
| **API docs** | https://data.ecb.europa.eu/help/api/overview |

---

## Setting Keys

### For CLI / serve mode

Add to your shell profile or `.env` file:

```bash
# Required
export FRED_API_KEY="your_fred_key"

# Strongly recommended
export SEC_USER_AGENT="Your Name your@email.com"

# Optional
export BLS_API_KEY="your_bls_key"
export COINGECKO_API_KEY="CG-your_key"
export FEC_API_KEY="your_api_data_gov_key"
```

### For MCP (Claude Desktop / Cursor)

Set `env` in your MCP config block:

```json
{
  "mcpServers": {
    "elko-market-mcp": {
      "command": "/path/to/elko",
      "args": ["mcp"],
      "env": {
        "FRED_API_KEY": "your_fred_key",
        "SEC_USER_AGENT": "Your Name your@email.com"
      }
    }
  }
}
```

### For Docker

```bash
docker run -e FRED_API_KEY=your_key \
           -e SEC_USER_AGENT="Your Name your@email.com" \
           -p 8080:8080 \
           ghcr.io/jsoprych/elko-market-mcp:latest serve
```

Or in `docker-compose.yml`:

```yaml
environment:
  - FRED_API_KEY=${FRED_API_KEY}
  - SEC_USER_AGENT=${SEC_USER_AGENT}
```

---

## Notes

**api.data.gov key** — One registration gives you access to FEC, Census Bureau,
NASA, USDA, and dozens of other US government APIs. Worth registering regardless
of whether you use the FEC channel. Register at https://api.data.gov/signup/.

**Yahoo Finance** — No key, but the most fragile channel. Yahoo has broken the
unofficial API multiple times. The integration test exists specifically to catch
this. No mitigation other than fixing the extractor when it breaks.

**SEC rate limits** — 10 requests/second hard limit across all EDGAR endpoints.
The channel cache (default TTLs: company_info 24h, financials 24h, insider_trades 4h)
keeps production request rates well under this. Be careful running integration
tests without the cache — 30+ XML fetches in a tight loop can trigger a temporary
IP block.
