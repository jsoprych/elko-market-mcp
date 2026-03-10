package calllog

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/jsoprych/elko-market-mcp/internal/cache"
)

func openTestDB(t *testing.T) *Logger {
	t.Helper()
	db, err := cache.OpenSQLite(":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return New(db, DefaultMaxOutput)
}

func TestLogger_NilSafe(t *testing.T) {
	var l *Logger
	// All methods on nil Logger must be no-ops, not panics.
	l.Log("tool", "src", json.RawMessage(`{}`), "result", nil, time.Millisecond)
	entries, err := l.Query(10, "", false)
	if err != nil {
		t.Fatal(err)
	}
	if entries != nil {
		t.Errorf("expected nil entries from nil logger, got %v", entries)
	}
}

func TestLogger_LogAndQuery(t *testing.T) {
	l := openTestDB(t)

	l.Log("yahoo_quote", "yahoo", json.RawMessage(`{"symbol":"NVDA"}`), "result text", nil, 100*time.Millisecond)
	l.Log("treasury_yields", "treasury", json.RawMessage(`{}`), "yield data", nil, 200*time.Millisecond)

	entries, err := l.Query(10, "", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("want 2 entries, got %d", len(entries))
	}
	// newest first
	if entries[0].Tool != "treasury_yields" {
		t.Errorf("want treasury_yields first (newest), got %s", entries[0].Tool)
	}
}

func TestLogger_Query_FilterByTool(t *testing.T) {
	l := openTestDB(t)
	l.Log("tool_a", "src", json.RawMessage(`{}`), "", nil, 0)
	l.Log("tool_b", "src", json.RawMessage(`{}`), "", nil, 0)
	l.Log("tool_a", "src", json.RawMessage(`{}`), "", nil, 0)

	entries, err := l.Query(10, "tool_a", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Errorf("want 2 tool_a entries, got %d", len(entries))
	}
}

func TestLogger_Query_FilterErrorsOnly(t *testing.T) {
	l := openTestDB(t)
	l.Log("ok_tool",  "src", json.RawMessage(`{}`), "ok",    nil,                  0)
	l.Log("bad_tool", "src", json.RawMessage(`{}`), "",      errors.New("boom"),   0)
	l.Log("ok_tool2", "src", json.RawMessage(`{}`), "fine",  nil,                  0)

	entries, err := l.Query(10, "", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("want 1 error entry, got %d", len(entries))
	}
	if entries[0].Error != "boom" {
		t.Errorf("want error=boom, got %q", entries[0].Error)
	}
}

func TestLogger_ResultTruncation(t *testing.T) {
	db, err := cache.OpenSQLite(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	l := New(db, 10) // tiny limit
	long := "0123456789ABCDEF" // 16 chars
	l.Log("tool", "src", json.RawMessage(`{}`), long, nil, 0)

	entries, _ := l.Query(1, "", false)
	if len(entries) != 1 {
		t.Fatal("expected 1 entry")
	}
	if len(entries[0].Result) > 10 {
		t.Errorf("result not truncated: %q", entries[0].Result)
	}
	if entries[0].ResultLen != len(long) {
		t.Errorf("result_len should be original length %d, got %d", len(long), entries[0].ResultLen)
	}
}

func TestLogger_Query_LimitCapped(t *testing.T) {
	l := openTestDB(t)
	for i := 0; i < 5; i++ {
		l.Log("t", "s", json.RawMessage(`{}`), "", nil, 0)
	}
	entries, _ := l.Query(3, "", false)
	if len(entries) != 3 {
		t.Errorf("want 3 entries with limit=3, got %d", len(entries))
	}
}

func TestLogger_DurationRecorded(t *testing.T) {
	l := openTestDB(t)
	l.Log("tool", "src", json.RawMessage(`{}`), "", nil, 750*time.Millisecond)

	entries, _ := l.Query(1, "", false)
	if entries[0].DurationMs != 750 {
		t.Errorf("want 750ms, got %d", entries[0].DurationMs)
	}
}
