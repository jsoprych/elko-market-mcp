package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/jsoprych/elko-market-mcp/internal/registry"
)

// newTestServer builds an MCP server with one registered tool for testing.
func newTestServer() *Server {
	reg := registry.New()
	reg.Register(registry.Tool{
		Name:        "echo_tool",
		Description: "echoes args back",
		Schema:      json.RawMessage(`{"type":"object","properties":{"msg":{"type":"string"}}}`),
		Source:      "test",
		Handler: func(_ context.Context, args json.RawMessage) (string, error) {
			return "echo:" + string(args), nil
		},
	})
	return New(reg, "test")
}

func rpc(method string, id any, params any) string {
	m := map[string]any{"jsonrpc": "2.0", "method": method}
	if id != nil {
		m["id"] = id
	}
	if params != nil {
		m["params"] = params
	}
	b, _ := json.Marshal(m)
	return string(b)
}

func TestMCP_Initialize(t *testing.T) {
	srv := newTestServer()
	out := runIO(t, srv, rpc("initialize", 1, map[string]any{
		"protocolVersion": "2024-11-05",
		"clientInfo":      map[string]any{"name": "test"},
	}))
	assertField(t, out, "result.protocolVersion", "2024-11-05")
	assertField(t, out, "result.serverInfo.name", "elko-market-mcp")
}

func TestMCP_ToolsList(t *testing.T) {
	srv := newTestServer()
	out := runIO(t, srv, rpc("tools/list", 2, map[string]any{}))

	tools, ok := out["result"].(map[string]any)["tools"].([]any)
	if !ok || len(tools) == 0 {
		t.Fatalf("expected tools array, got: %v", out["result"])
	}
	first := tools[0].(map[string]any)
	if first["name"] != "echo_tool" {
		t.Errorf("expected echo_tool, got %v", first["name"])
	}
}

func TestMCP_ToolsCall(t *testing.T) {
	srv := newTestServer()
	out := runIO(t, srv, rpc("tools/call", 3, map[string]any{
		"name":      "echo_tool",
		"arguments": map[string]any{"msg": "hello"},
	}))
	content := out["result"].(map[string]any)["content"].([]any)
	text := content[0].(map[string]any)["text"].(string)
	if !strings.HasPrefix(text, "echo:") {
		t.Errorf("unexpected result: %s", text)
	}
}

func TestMCP_ToolsCall_UnknownTool(t *testing.T) {
	srv := newTestServer()
	out := runIO(t, srv, rpc("tools/call", 4, map[string]any{
		"name":      "no_such_tool",
		"arguments": map[string]any{},
	}))
	// Unknown tool returns isError:true in result (not a JSON-RPC error)
	result := out["result"].(map[string]any)
	if result["isError"] != true {
		t.Errorf("expected isError:true, got %v", result)
	}
}

func TestMCP_Ping(t *testing.T) {
	srv := newTestServer()
	out := runIO(t, srv, rpc("ping", 5, nil))
	if _, ok := out["result"]; !ok {
		t.Errorf("ping should return a result, got: %v", out)
	}
}

func TestMCP_UnknownMethod_WithID_ReturnsError(t *testing.T) {
	srv := newTestServer()
	out := runIO(t, srv, rpc("resources/list", 6, nil))
	if out["error"] == nil {
		t.Errorf("expected JSON-RPC error for unknown method with id, got: %v", out)
	}
	code := int(out["error"].(map[string]any)["code"].(float64))
	if code != -32601 {
		t.Errorf("expected -32601, got %d", code)
	}
}

func TestMCP_Notification_NoID_NoResponse(t *testing.T) {
	// Notifications (no id) must never produce a response — spec violation otherwise.
	srv := newTestServer()
	var buf bytes.Buffer
	input := rpc("notifications/cancelled", nil, map[string]any{"requestId": 1}) + "\n"
	err := srv.ServeIO(context.Background(), strings.NewReader(input), &buf)
	if err != nil {
		t.Fatal(err)
	}
	if buf.Len() != 0 {
		t.Errorf("notification produced unexpected output: %q", buf.String())
	}
}

func TestMCP_InitializedNotification_NoResponse(t *testing.T) {
	srv := newTestServer()
	var buf bytes.Buffer
	input := rpc("notifications/initialized", nil, nil) + "\n"
	_ = srv.ServeIO(context.Background(), strings.NewReader(input), &buf)
	if buf.Len() != 0 {
		t.Errorf("notifications/initialized produced output: %q", buf.String())
	}
}

func TestMCP_ParseError(t *testing.T) {
	srv := newTestServer()
	out := runIO(t, srv, "this is not json")
	code := int(out["error"].(map[string]any)["code"].(float64))
	if code != -32700 {
		t.Errorf("expected -32700 parse error, got %d", code)
	}
}

// ── helpers ────────────────────────────────────────────────────────────────────

func runIO(t *testing.T, srv *Server, line string) map[string]any {
	t.Helper()
	var buf bytes.Buffer
	input := line + "\n"
	if err := srv.ServeIO(context.Background(), strings.NewReader(input), &buf); err != nil {
		t.Fatalf("ServeIO error: %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal output %q: %v", buf.String(), err)
	}
	return out
}

func assertField(t *testing.T, m map[string]any, path string, want string) {
	t.Helper()
	parts := strings.SplitN(path, ".", 2)
	val, ok := m[parts[0]]
	if !ok {
		t.Errorf("field %q missing in %v", parts[0], m)
		return
	}
	if len(parts) == 1 {
		if got := val.(string); got != want {
			t.Errorf("%s: want %q got %q", path, want, got)
		}
		return
	}
	assertField(t, val.(map[string]any), parts[1], want)
}
