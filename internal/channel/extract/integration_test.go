//go:build integration

// Integration tests: invoke each channel tool against the live upstream API.
// Run with:  go test -tags integration -v -timeout 60s ./internal/channel/extract/
//
// These tests make real HTTP requests.  They are intentionally skipped in the
// normal test run (go test ./...) and in CI unless the integration tag is set.
// They serve as an early-warning system when an upstream provider changes its
// API (URL structure, field names, response schema, HTTP status codes).
//
// Each test is deliberately minimal:
//  - Use the smallest valid argument set for the channel.
//  - Assert: no error + non-empty result.
//  - A non-empty result confirms the full round-trip (fetch → parse → format) worked.
package extract

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/jsoprych/elko-market-mcp/channels"
	"github.com/jsoprych/elko-market-mcp/internal/cache"
	"github.com/jsoprych/elko-market-mcp/internal/channel"
	"github.com/jsoprych/elko-market-mcp/internal/registry"
)

// buildLiveRegistry wires the full stack against the embedded channel specs,
// using an in-memory (no SQLite) cache with a very short TTL so tests always
// hit the upstream.
func buildLiveRegistry(t *testing.T) *registry.Registry {
	t.Helper()
	c := cache.New(nil) // in-memory only, no SQLite
	runner := channel.NewRunner(c)
	RegisterAll(runner)

	specs, err := channel.LoadFS(channels.FS)
	if err != nil {
		t.Fatalf("load channel specs: %v", err)
	}

	reg := registry.New()
	if err := runner.Register(reg, specs); err != nil {
		t.Fatalf("register channels: %v", err)
	}
	return reg
}

// dispatch is a helper that calls a tool and asserts a non-empty result.
func dispatch(t *testing.T, reg *registry.Registry, tool string, args any) string {
	t.Helper()
	b, err := json.Marshal(args)
	if err != nil {
		t.Fatalf("marshal args: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	result, err := reg.Dispatch(ctx, tool, b)
	if err != nil {
		t.Fatalf("%s: %v", tool, err)
	}
	if result == "" {
		t.Errorf("%s: got empty result", tool)
	}
	return result
}

// ── Yahoo Finance ─────────────────────────────────────────────────────────────

func TestIntegration_YahooQuote(t *testing.T) {
	reg := buildLiveRegistry(t)
	dispatch(t, reg, "yahoo_quote", map[string]any{"symbol": "AAPL"})
}

func TestIntegration_YahooHistory(t *testing.T) {
	reg := buildLiveRegistry(t)
	dispatch(t, reg, "yahoo_history", map[string]any{"symbol": "AAPL", "period": "5d"})
}

func TestIntegration_YahooDividends(t *testing.T) {
	reg := buildLiveRegistry(t)
	dispatch(t, reg, "yahoo_dividends", map[string]any{"symbol": "AAPL"})
}

// ── SEC EDGAR ─────────────────────────────────────────────────────────────────

func TestIntegration_EDGARCompanyInfo(t *testing.T) {
	reg := buildLiveRegistry(t)
	dispatch(t, reg, "edgar_company_info", map[string]any{"symbol": "MSFT"})
}

func TestIntegration_EDGARFinancials(t *testing.T) {
	reg := buildLiveRegistry(t)
	dispatch(t, reg, "edgar_financials", map[string]any{"symbol": "MSFT", "form": "10-K", "periods": 1})
}

// ── US Treasury ───────────────────────────────────────────────────────────────

func TestIntegration_TreasuryYields(t *testing.T) {
	reg := buildLiveRegistry(t)
	dispatch(t, reg, "treasury_yields", map[string]any{"latest": true})
}

// ── Bureau of Labor Statistics ────────────────────────────────────────────────

func TestIntegration_BLSSeries(t *testing.T) {
	reg := buildLiveRegistry(t)
	// CPI-U All Items — a very stable, well-known series.
	dispatch(t, reg, "bls_series", map[string]any{
		"series_id":  "CUUR0000SA0",
		"start_year": "2024",
		"end_year":   "2024",
	})
}

// ── FDIC ──────────────────────────────────────────────────────────────────────

func TestIntegration_FDICBankSearch(t *testing.T) {
	reg := buildLiveRegistry(t)
	dispatch(t, reg, "fdic_bank_search", map[string]any{"name": "Wells", "limit": 3})
}

func TestIntegration_FDICBankFinancials(t *testing.T) {
	reg := buildLiveRegistry(t)
	// cert 3511 = First National Bank of Omaha — long-established, unlikely to disappear.
	dispatch(t, reg, "fdic_bank_financials", map[string]any{"cert": "3511"})
}

// ── World Bank ────────────────────────────────────────────────────────────────

func TestIntegration_WorldBankIndicator(t *testing.T) {
	reg := buildLiveRegistry(t)
	dispatch(t, reg, "worldbank_indicator", map[string]any{
		"country":   "US",
		"indicator": "NY.GDP.MKTP.KD.ZG", // GDP growth annual %
		"from_year": 2022,
		"to_year":   2023,
	})
}
