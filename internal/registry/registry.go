// Package registry provides a unified tool catalogue shared across the MCP,
// REST API, and CLI interfaces. Register a tool once; all three interfaces
// expose it automatically.
package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"
)

// CallLogger is satisfied by *calllog.Logger. Defined here to avoid an import
// cycle; callers inject the concrete type via SetLogger.
type CallLogger interface {
	Log(tool, source string, args json.RawMessage, result string, err error, duration time.Duration)
}

// Tool is a single callable data-fetching operation.
type Tool struct {
	Name         string          // snake_case, e.g. "market_quote"
	Description  string          // human-readable, shown in MCP tools/list and REST /v1/catalogue
	Schema       json.RawMessage // JSON Schema for arguments (OpenAI-compatible)
	Source       string          // source tag: "yahoo", "edgar", "treasury", "bls", "fdic", "worldbank"
	Category     string          // data category: "equity", "macro", "rates", "banking"
	ResultFormat string          // ui hint: "table" | "csv" | "kv" | "sections"
	Chart        json.RawMessage // optional chart spec: {type,x,y}
	Handler      Handler
}

// Handler executes a tool call. Args is the raw JSON arguments object.
// Returns a human-readable (markdown/CSV/plain) result string.
type Handler func(ctx context.Context, args json.RawMessage) (string, error)

// Registry holds all registered tools.
type Registry struct {
	mu     sync.RWMutex
	tools  map[string]Tool
	order  []string
	logger CallLogger // optional; nil = logging disabled
}

// SetLogger attaches a call logger. Safe to call after registration.
func (r *Registry) SetLogger(l CallLogger) {
	r.mu.Lock()
	r.logger = l
	r.mu.Unlock()
}

func New() *Registry {
	return &Registry{tools: make(map[string]Tool)}
}

// Register adds a tool. Panics on duplicate name (caught at startup).
func (r *Registry) Register(t Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.tools[t.Name]; exists {
		panic(fmt.Sprintf("registry: duplicate tool %q", t.Name))
	}
	r.tools[t.Name] = t
	r.order = append(r.order, t.Name)
}

// List returns all tools in registration order.
func (r *Registry) List() []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Tool, 0, len(r.order))
	for _, name := range r.order {
		out = append(out, r.tools[name])
	}
	return out
}

// ListBySource returns tools filtered to the given source tag.
func (r *Registry) ListBySource(source string) []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []Tool
	for _, name := range r.order {
		if r.tools[name].Source == source {
			out = append(out, r.tools[name])
		}
	}
	return out
}

// Sources returns sorted unique source tags registered.
func (r *Registry) Sources() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	seen := map[string]bool{}
	for _, t := range r.tools {
		seen[t.Source] = true
	}
	out := make([]string, 0, len(seen))
	for s := range seen {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

// Dispatch executes a named tool and logs the call when a logger is set.
func (r *Registry) Dispatch(ctx context.Context, name string, args json.RawMessage) (string, error) {
	r.mu.RLock()
	t, ok := r.tools[name]
	logger := r.logger
	r.mu.RUnlock()
	if !ok {
		return "", fmt.Errorf("unknown tool: %q", name)
	}
	start := time.Now()
	result, err := t.Handler(ctx, args)
	if logger != nil {
		logger.Log(name, t.Source, args, result, err, time.Since(start))
	}
	return result, err
}
