// Package api provides the Chi-based REST API server.
package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jsoprych/elko-market-mcp/internal/registry"
)

// Server is the REST API server.
type Server struct {
	reg     *registry.Registry
	version string
	webRoot string // optional; serves static UI if non-empty
}

func New(reg *registry.Registry, version string) *Server {
	return &Server{reg: reg, version: version}
}

// WithWebRoot configures the server to serve a static UI from the given directory.
func (s *Server) WithWebRoot(root string) *Server {
	s.webRoot = root
	return s
}

// Handler returns the configured Chi router.
func (s *Server) Handler() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))

	r.Get("/health", s.handleHealth)
	r.Get("/v1/catalogue", s.handleCatalogue)
	r.Post("/v1/call/{tool}", s.handleCall)
	r.Get("/v1/sources", s.handleSources)

	if s.webRoot != "" {
		fs := http.FileServer(http.Dir(s.webRoot))
		r.NotFound(fs.ServeHTTP)
	}

	return r
}

// ── Handlers ──────────────────────────────────────────────────────────────────

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "ok",
		"version": s.version,
		"tools":   len(s.reg.List()),
	})
}

func (s *Server) handleSources(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"sources": s.reg.Sources(),
	})
}

func (s *Server) handleCatalogue(w http.ResponseWriter, r *http.Request) {
	source := r.URL.Query().Get("source")
	category := r.URL.Query().Get("category")

	var tools []registry.Tool
	if source != "" {
		tools = s.reg.ListBySource(source)
	} else {
		tools = s.reg.List()
	}

	if category != "" {
		var filtered []registry.Tool
		for _, t := range tools {
			if t.Category == category {
				filtered = append(filtered, t)
			}
		}
		tools = filtered
	}

	type toolEntry struct {
		Name         string          `json:"name"`
		Description  string          `json:"description"`
		Schema       json.RawMessage `json:"schema"`
		Source       string          `json:"source"`
		Category     string          `json:"category"`
		ResultFormat string          `json:"result_format,omitempty"`
		Chart        json.RawMessage `json:"chart,omitempty"`
	}
	entries := make([]toolEntry, 0, len(tools))
	for _, t := range tools {
		entries = append(entries, toolEntry{
			Name:         t.Name,
			Description:  t.Description,
			Schema:       t.Schema,
			Source:       t.Source,
			Category:     t.Category,
			ResultFormat: t.ResultFormat,
			Chart:        t.Chart,
		})
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"tools": entries,
		"count": len(entries),
	})
}

func (s *Server) handleCall(w http.ResponseWriter, r *http.Request) {
	toolName := chi.URLParam(r, "tool")

	var args json.RawMessage
	if r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&args); err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid JSON body: %s", err))
			return
		}
	} else {
		args = json.RawMessage(`{}`)
	}

	// Also accept query params as JSON args for simple GET-style calls.
	// Coerce "true"/"false" to booleans so typed struct fields unmarshal correctly.
	if len(r.URL.Query()) > 0 && string(args) == `{}` {
		m := make(map[string]any)
		for k, v := range r.URL.Query() {
			if len(v) == 0 {
				continue
			}
			switch v[0] {
			case "true":
				m[k] = true
			case "false":
				m[k] = false
			default:
				m[k] = v[0]
			}
		}
		b, _ := json.Marshal(m)
		args = b
	}

	result, err := s.reg.Dispatch(r.Context(), toolName, args)
	if err != nil {
		code := http.StatusInternalServerError
		if strings.Contains(err.Error(), "unknown tool") {
			code = http.StatusNotFound
		}
		writeError(w, code, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"tool":   toolName,
		"result": result,
	})
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
