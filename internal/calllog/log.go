// Package calllog persists tool invocation records to SQLite and exposes them
// for the dashboard log viewer. All exported methods are nil-safe — a nil
// Logger is a valid no-op (logging disabled when --db is not set).
package calllog

import (
	"database/sql"
	"encoding/json"
	"time"
)

const DefaultMaxOutput = 2000

// Logger writes call records to SQLite.
type Logger struct {
	db        *sql.DB
	maxOutput int
}

// New returns a Logger backed by db. Returns nil when db is nil (no-op mode).
func New(db *sql.DB, maxOutput int) *Logger {
	if db == nil {
		return nil
	}
	if maxOutput <= 0 {
		maxOutput = DefaultMaxOutput
	}
	return &Logger{db: db, maxOutput: maxOutput}
}

// Entry is a single logged tool invocation.
type Entry struct {
	ID         int64     `json:"id"`
	TS         time.Time `json:"ts"`
	Tool       string    `json:"tool"`
	Source     string    `json:"source"`
	Args       string    `json:"args"`
	Result     string    `json:"result"`
	ResultLen  int       `json:"result_len"`
	Error      string    `json:"error"`
	DurationMs int64     `json:"duration_ms"`
}

// Log records one tool invocation. Safe to call on a nil Logger.
func (l *Logger) Log(tool, source string, args json.RawMessage, result string, callErr error, duration time.Duration) {
	if l == nil {
		return
	}
	errStr := ""
	if callErr != nil {
		errStr = callErr.Error()
	}
	resultLen := len(result)
	if len(result) > l.maxOutput {
		result = result[:l.maxOutput]
	}
	argsStr := "{}"
	if len(args) > 0 {
		argsStr = string(args)
	}
	l.db.Exec(
		`INSERT INTO call_log (ts, tool, source, args, result, result_len, error, duration_ms)
		 VALUES (?,?,?,?,?,?,?,?)`,
		time.Now().Unix(), tool, source, argsStr, result, resultLen, errStr, duration.Milliseconds(),
	)
}

// Query returns recent log entries, newest first.
// limit is capped at 1000; tool and errorsOnly are optional filters.
func (l *Logger) Query(limit int, tool string, errorsOnly bool) ([]Entry, error) {
	if l == nil {
		return nil, nil
	}
	if limit <= 0 || limit > 1000 {
		limit = 100
	}

	q := `SELECT id, ts, tool, source, args, result, result_len, error, duration_ms
	      FROM call_log WHERE 1=1`
	var qArgs []interface{}

	if tool != "" {
		q += " AND tool = ?"
		qArgs = append(qArgs, tool)
	}
	if errorsOnly {
		q += " AND error != ''"
	}
	q += " ORDER BY ts DESC, id DESC LIMIT ?"
	qArgs = append(qArgs, limit)

	rows, err := l.db.Query(q, qArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []Entry
	for rows.Next() {
		var e Entry
		var ts int64
		if err := rows.Scan(&e.ID, &ts, &e.Tool, &e.Source, &e.Args,
			&e.Result, &e.ResultLen, &e.Error, &e.DurationMs); err != nil {
			return nil, err
		}
		e.TS = time.Unix(ts, 0).UTC()
		entries = append(entries, e)
	}
	return entries, rows.Err()
}
