package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jsoprych/elko-market-mcp/internal/calllog"
	"github.com/jsoprych/elko-market-mcp/internal/cache"
	"github.com/jsoprych/elko-market-mcp/internal/registry"
)

func newTestAPI(t *testing.T) (*Server, *registry.Registry) {
	t.Helper()
	reg := registry.New()
	reg.Register(registry.Tool{
		Name:         "ping_tool",
		Description:  "returns pong",
		Source:       "test",
		Category:     "test",
		ResultFormat: "kv",
		Schema:       json.RawMessage(`{"type":"object","properties":{}}`),
		Handler: func(_ context.Context, _ json.RawMessage) (string, error) {
			return "pong", nil
		},
	})
	return New(reg, "test-version"), reg
}

func TestAPI_Health(t *testing.T) {
	srv, _ := newTestAPI(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
	var body map[string]any
	json.NewDecoder(rec.Body).Decode(&body)
	if body["status"] != "ok" {
		t.Errorf("want status=ok, got %v", body["status"])
	}
	if body["version"] != "test-version" {
		t.Errorf("want version=test-version, got %v", body["version"])
	}
}

func TestAPI_Catalogue(t *testing.T) {
	srv, _ := newTestAPI(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/catalogue", nil)
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
	var body map[string]any
	json.NewDecoder(rec.Body).Decode(&body)
	if int(body["count"].(float64)) != 1 {
		t.Errorf("want count=1, got %v", body["count"])
	}
}

func TestAPI_Call_Success(t *testing.T) {
	srv, _ := newTestAPI(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/call/ping_tool",
		strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	json.NewDecoder(rec.Body).Decode(&body)
	if body["result"] != "pong" {
		t.Errorf("want result=pong, got %v", body["result"])
	}
}

func TestAPI_Call_UnknownTool(t *testing.T) {
	srv, _ := newTestAPI(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/call/no_such_tool",
		strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d", rec.Code)
	}
}

func TestAPI_Call_BadJSON(t *testing.T) {
	srv, _ := newTestAPI(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/call/ping_tool",
		strings.NewReader(`not json`))
	req.Header.Set("Content-Type", "application/json")
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
}

func TestAPI_Sources(t *testing.T) {
	srv, _ := newTestAPI(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/sources", nil)
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
	var body map[string]any
	json.NewDecoder(rec.Body).Decode(&body)
	srcs := body["sources"].([]any)
	if len(srcs) == 0 {
		t.Error("expected at least one source")
	}
}

func TestAPI_Logs_404_WhenNoLogger(t *testing.T) {
	srv, _ := newTestAPI(t) // no WithLogger
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/logs", nil)
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("want 404 when no logger, got %d", rec.Code)
	}
}

func TestAPI_Logs_200_WithLogger(t *testing.T) {
	srv, reg := newTestAPI(t)

	db, err := cache.OpenSQLite(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	logger := calllog.New(db, calllog.DefaultMaxOutput)
	reg.SetLogger(logger)
	srv = srv.WithLogger(logger)

	// Populate one log entry
	logger.Log("ping_tool", "test", json.RawMessage(`{}`), "pong", nil, 0)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/logs?limit=10", nil)
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	json.NewDecoder(rec.Body).Decode(&body)
	if int(body["count"].(float64)) != 1 {
		t.Errorf("want count=1, got %v", body["count"])
	}
}

func TestAPI_Logs_FilterByTool(t *testing.T) {
	srv, _ := newTestAPI(t)

	db, _ := cache.OpenSQLite(":memory:")
	defer db.Close()
	logger := calllog.New(db, calllog.DefaultMaxOutput)
	srv = srv.WithLogger(logger)

	logger.Log("tool_a", "src", json.RawMessage(`{}`), "", nil, 0)
	logger.Log("tool_b", "src", json.RawMessage(`{}`), "", nil, 0)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/logs?tool=tool_a", nil)
	srv.Handler().ServeHTTP(rec, req)

	var body map[string]any
	json.NewDecoder(rec.Body).Decode(&body)
	if int(body["count"].(float64)) != 1 {
		t.Errorf("want count=1 for tool_a filter, got %v", body["count"])
	}
}
