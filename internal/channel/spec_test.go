package channel

import (
	"io/fs"
	"testing"
	"testing/fstest"
	"time"
)

func TestParseTTL(t *testing.T) {
	cases := []struct {
		ttl  string
		want time.Duration
	}{
		{"5m", 5 * time.Minute},
		{"24h", 24 * time.Hour},
		{"1h", time.Hour},
		{"",   time.Hour}, // default on empty
		{"bad", time.Hour}, // default on parse error
		{"0s", time.Hour}, // default on zero
		{"-1h", time.Hour}, // default on negative
	}
	for _, tc := range cases {
		r := RequestSpec{TTL: tc.ttl}
		got := r.ParseTTL()
		if got != tc.want {
			t.Errorf("ParseTTL(%q) = %v, want %v", tc.ttl, got, tc.want)
		}
	}
}

func TestLoadFS_Valid(t *testing.T) {
	fsys := fstest.MapFS{
		"yahoo/quote.json": {Data: []byte(`{
			"name":         "yahoo_quote",
			"description":  "test",
			"source":       "yahoo",
			"category":     "equity",
			"schema":       {"type":"object"},
			"result_format":"kv",
			"request":      {"base_url":"https://example.com","ttl":"5m"},
			"response":     {"extractor":"yahoo_chart_quote"}
		}`)},
	}
	specs, err := LoadFS(fsys)
	if err != nil {
		t.Fatal(err)
	}
	if len(specs) != 1 {
		t.Fatalf("want 1 spec, got %d", len(specs))
	}
	s := specs[0]
	if s.Name != "yahoo_quote" {
		t.Errorf("want name=yahoo_quote, got %q", s.Name)
	}
	if s.Request.BaseURL != "https://example.com" {
		t.Errorf("want base_url=https://example.com, got %q", s.Request.BaseURL)
	}
	if s.Request.ParseTTL() != 5*time.Minute {
		t.Errorf("want TTL=5m, got %v", s.Request.ParseTTL())
	}
}

func TestLoadFS_Options(t *testing.T) {
	fsys := fstest.MapFS{
		"fdic/search.json": {Data: []byte(`{
			"name":"fdic_bank_search","description":"d","source":"fdic","category":"banking",
			"schema":{"type":"object"},"result_format":"table",
			"options":{"field_case":"upper"},
			"request":{"base_url":"https://api.fdic.gov/banks","ttl":"24h"},
			"response":{"extractor":"fdic_bank_search"}
		}`)},
	}
	specs, err := LoadFS(fsys)
	if err != nil {
		t.Fatal(err)
	}
	if specs[0].Options["field_case"] != "upper" {
		t.Errorf("expected field_case=upper in options, got %v", specs[0].Options)
	}
}

func TestLoadFS_InvalidJSON(t *testing.T) {
	fsys := fstest.MapFS{
		"bad/channel.json": {Data: []byte(`not json`)},
	}
	_, err := LoadFS(fsys)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestLoadFS_SkipsNonJSON(t *testing.T) {
	fsys := fstest.MapFS{
		"README.md":        {Data: []byte("ignore me")},
		"yahoo/quote.json": {Data: []byte(`{
			"name":"t","description":"d","source":"s","category":"c",
			"schema":{"type":"object"},"result_format":"kv",
			"request":{"base_url":"https://x.com","ttl":"1h"},
			"response":{"extractor":"e"}
		}`)},
	}
	specs, err := LoadFS(fsys)
	if err != nil {
		t.Fatal(err)
	}
	if len(specs) != 1 {
		t.Errorf("want 1 spec (skipping .md), got %d", len(specs))
	}
}

func TestLoadFS_Empty(t *testing.T) {
	specs, err := LoadFS(fstest.MapFS{})
	if err != nil {
		t.Fatal(err)
	}
	if len(specs) != 0 {
		t.Errorf("want 0 specs, got %d", len(specs))
	}
}

var _ fs.FS = fstest.MapFS{} // compile-time interface check
