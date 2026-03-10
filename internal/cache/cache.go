// Package cache provides a two-layer cache: in-memory L1 (always on) backed
// by an optional SQLite L2 (enabled when --db is set). Both layers use TTL
// expiry. SQLite L2 survives container restarts when the db file is on a
// mounted volume.
package cache

import (
	"database/sql"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// Cache is the two-layer TTL cache.
type Cache struct {
	mu  sync.RWMutex
	l1  map[string]l1Entry // in-memory
	db  *sql.DB            // nil when no --db flag
}

type l1Entry struct {
	data      []byte
	expiresAt time.Time
}

// New creates a cache. db may be nil (memory-only).
func New(db *sql.DB) *Cache {
	return &Cache{
		l1: make(map[string]l1Entry),
		db: db,
	}
}

// OpenSQLite opens (or creates) a SQLite database and initialises the cache
// schema. Returns the *sql.DB for use in other packages (price_history, etc).
func OpenSQLite(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1) // SQLite is single-writer
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

const schema = `
PRAGMA journal_mode=WAL;
PRAGMA foreign_keys=ON;

-- Tool call log (enabled when --db is set).
CREATE TABLE IF NOT EXISTS call_log (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    ts          INTEGER NOT NULL,
    tool        TEXT    NOT NULL DEFAULT '',
    source      TEXT    NOT NULL DEFAULT '',
    args        TEXT    NOT NULL DEFAULT '{}',
    result      TEXT    NOT NULL DEFAULT '',
    result_len  INTEGER NOT NULL DEFAULT 0,
    error       TEXT    NOT NULL DEFAULT '',
    duration_ms INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_call_log_ts   ON call_log (ts DESC);
CREATE INDEX IF NOT EXISTS idx_call_log_tool ON call_log (tool);

-- Generic HTTP response cache (all sources).
CREATE TABLE IF NOT EXISTS cache (
    key        TEXT    PRIMARY KEY,
    data       BLOB    NOT NULL,
    fetched_at INTEGER NOT NULL,
    expires_at INTEGER NOT NULL
);

-- Structured OHLCV price history (Yahoo Finance).
CREATE TABLE IF NOT EXISTS price_history (
    symbol    TEXT    NOT NULL,
    ts        INTEGER NOT NULL,
    interval  TEXT    NOT NULL DEFAULT '1d',
    open      REAL,
    high      REAL,
    low       REAL,
    close     REAL,
    adj_close REAL,
    volume    INTEGER,
    PRIMARY KEY (symbol, ts, interval)
);
CREATE INDEX IF NOT EXISTS idx_ph_symbol_interval ON price_history (symbol, interval);

-- Dividend events.
CREATE TABLE IF NOT EXISTS dividends (
    symbol TEXT    NOT NULL,
    date   TEXT    NOT NULL,  -- YYYY-MM-DD
    amount REAL    NOT NULL,
    PRIMARY KEY (symbol, date)
);

-- Financial statement rows (EDGAR XBRL).
CREATE TABLE IF NOT EXISTS financials (
    symbol    TEXT    NOT NULL,
    cik       TEXT    NOT NULL,
    statement TEXT    NOT NULL,  -- income | balance | cashflow
    frequency TEXT    NOT NULL,  -- annual | quarterly
    period    TEXT    NOT NULL,  -- YYYY-MM-DD end date
    concept   TEXT    NOT NULL,  -- GAAP tag label
    value     REAL    NOT NULL,
    fetched_at INTEGER NOT NULL,
    PRIMARY KEY (symbol, statement, frequency, period, concept)
);

-- Treasury yield curve daily snapshots.
CREATE TABLE IF NOT EXISTS yield_curve (
    date    TEXT NOT NULL,  -- YYYY-MM-DD
    tenor   TEXT NOT NULL,  -- 1mo 3mo 6mo 1y 2y 3y 5y 7y 10y 20y 30y
    rate    REAL NOT NULL,  -- percent
    PRIMARY KEY (date, tenor)
);

-- Macro time series (BLS, World Bank, etc).
CREATE TABLE IF NOT EXISTS macro_series (
    source     TEXT    NOT NULL,  -- bls | worldbank | fred
    series_id  TEXT    NOT NULL,  -- source-specific ID
    label      TEXT    NOT NULL,  -- human-readable name
    date       TEXT    NOT NULL,  -- YYYY-MM-DD or YYYY-MM or YYYY
    value      REAL    NOT NULL,
    fetched_at INTEGER NOT NULL,
    PRIMARY KEY (source, series_id, date)
);

-- FDIC bank financials.
CREATE TABLE IF NOT EXISTS bank_financials (
    cert       TEXT    NOT NULL,  -- FDIC certificate number
    name       TEXT    NOT NULL,
    report_date TEXT   NOT NULL,  -- YYYY-MM-DD
    concept    TEXT    NOT NULL,
    value      REAL    NOT NULL,
    fetched_at INTEGER NOT NULL,
    PRIMARY KEY (cert, report_date, concept)
);
`

// Get retrieves a cached value. Returns nil, false on miss or expiry.
func (c *Cache) Get(key string) ([]byte, bool) {
	now := time.Now()

	c.mu.RLock()
	if e, ok := c.l1[key]; ok && now.Before(e.expiresAt) {
		c.mu.RUnlock()
		return e.data, true
	}
	c.mu.RUnlock()

	if c.db == nil {
		return nil, false
	}

	var data []byte
	var expiresAt int64
	err := c.db.QueryRow(
		`SELECT data, expires_at FROM cache WHERE key = ?`, key,
	).Scan(&data, &expiresAt)
	if err != nil || now.Unix() >= expiresAt {
		return nil, false
	}

	// Promote to L1.
	c.mu.Lock()
	c.l1[key] = l1Entry{data: data, expiresAt: time.Unix(expiresAt, 0)}
	c.mu.Unlock()

	return data, true
}

// Set stores a value in both cache layers with the given TTL.
func (c *Cache) Set(key string, data []byte, ttl time.Duration) {
	exp := time.Now().Add(ttl)

	c.mu.Lock()
	c.l1[key] = l1Entry{data: data, expiresAt: exp}
	c.mu.Unlock()

	if c.db != nil {
		c.db.Exec(
			`INSERT OR REPLACE INTO cache (key, data, fetched_at, expires_at) VALUES (?,?,?,?)`,
			key, data, time.Now().Unix(), exp.Unix(),
		)
	}
}

// Prune deletes expired entries from L1 and SQLite.
func (c *Cache) Prune() {
	now := time.Now()

	c.mu.Lock()
	for k, e := range c.l1 {
		if now.After(e.expiresAt) {
			delete(c.l1, k)
		}
	}
	c.mu.Unlock()

	if c.db != nil {
		c.db.Exec(`DELETE FROM cache WHERE expires_at < ?`, now.Unix())
	}
}
