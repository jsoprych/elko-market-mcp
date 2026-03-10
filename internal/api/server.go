// Package api provides the Chi-based REST API server.
package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jsoprych/elko-market-mcp/internal/calllog"
	"github.com/jsoprych/elko-market-mcp/internal/registry"
)

// Server is the REST API server.
type Server struct {
	reg        *registry.Registry
	version    string
	webRoot    string          // optional; serves static UI if non-empty
	mcpHandler http.Handler   // optional; mounted at POST /mcp when set
	logger     *calllog.Logger // optional; nil = /v1/logs returns 404
}

func New(reg *registry.Registry, version string) *Server {
	return &Server{reg: reg, version: version}
}

// WithWebRoot configures the server to serve a static UI from the given directory.
func (s *Server) WithWebRoot(root string) *Server {
	s.webRoot = root
	return s
}

// WithMCPHandler mounts an MCP JSON-RPC 2.0 handler at POST /mcp.
func (s *Server) WithMCPHandler(h http.Handler) *Server {
	s.mcpHandler = h
	return s
}

// WithLogger enables GET /v1/logs using the provided call logger.
func (s *Server) WithLogger(l *calllog.Logger) *Server {
	s.logger = l
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
	r.Get("/v1/keys", s.handleKeys)
	r.Get("/mcp.json", s.handleMCPConfig)

	if s.mcpHandler != nil {
		r.Post("/mcp", s.mcpHandler.ServeHTTP)
	}
	if s.logger != nil {
		r.Get("/v1/logs", s.handleLogs)
	}

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

func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	tool := q.Get("tool")
	errorsOnly := q.Get("error") == "true"

	entries, err := s.logger.Query(limit, tool, errorsOnly)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if entries == nil {
		entries = []calllog.Entry{} // always return an array, never null
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"entries": entries,
		"count":   len(entries),
	})
}

// knownKeys lists every env var elko reads, the source it belongs to,
// whether it is required (tool will error without it) or just recommended,
// and a registration URL shown to the user.
var knownKeys = []struct {
	source   string
	env      string
	required bool
	helpURL  string
	note     string
}{
	{"fred",  "FRED_API_KEY",   true,  "https://fred.stlouisfed.org/docs/api/api_key.html", "Free registration — required for all FRED requests"},
	{"edgar", "SEC_USER_AGENT", false, "https://www.sec.gov/developer",                     "Recommended: format \"Company Name email@domain.com\""},
	{"bls",   "BLS_API_KEY",    false, "https://data.bls.gov/registrationEngine/",           "Optional — lifts daily request cap from 25 to 500"},
}

func (s *Server) handleKeys(w http.ResponseWriter, r *http.Request) {
	// Only report keys for sources that are actually registered.
	activeSources := make(map[string]bool)
	for _, src := range s.reg.Sources() {
		activeSources[src] = true
	}

	type keyEntry struct {
		Source   string `json:"source"`
		Env      string `json:"env"`
		Required bool   `json:"required"`
		Set      bool   `json:"set"`
		HelpURL  string `json:"help_url"`
		Note     string `json:"note"`
	}

	var entries []keyEntry
	var missingRequired []string
	for _, k := range knownKeys {
		if !activeSources[k.source] {
			continue
		}
		set := os.Getenv(k.env) != ""
		entries = append(entries, keyEntry{
			Source:   k.source,
			Env:      k.env,
			Required: k.required,
			Set:      set,
			HelpURL:  k.helpURL,
			Note:     k.note,
		})
		if k.required && !set {
			missingRequired = append(missingRequired, k.env)
		}
	}
	if entries == nil {
		entries = []keyEntry{}
	}
	if missingRequired == nil {
		missingRequired = []string{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"keys":             entries,
		"missing_required": missingRequired,
	})
}

// handleMCPConfig returns a ready-to-use .mcp.json that points to this
// running server's HTTP MCP endpoint. The URL is derived from the request's
// Host header so it works on any port or remote host.
//
//	curl -s localhost:8080/mcp.json > .mcp.json
func (s *Server) handleMCPConfig(w http.ResponseWriter, r *http.Request) {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	host := r.Host
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"mcpServers": map[string]interface{}{
			"elko-market": map[string]interface{}{
				"type": "http",
				"url":  fmt.Sprintf("%s://%s/mcp", scheme, host),
			},
		},
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
