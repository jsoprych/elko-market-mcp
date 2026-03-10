package registry

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func echoTool(name string) Tool {
	return Tool{
		Name:   name,
		Source: "test",
		Schema: json.RawMessage(`{"type":"object"}`),
		Handler: func(_ context.Context, args json.RawMessage) (string, error) {
			return "result:" + name, nil
		},
	}
}

func TestRegistry_RegisterAndList(t *testing.T) {
	r := New()
	r.Register(echoTool("tool_a"))
	r.Register(echoTool("tool_b"))

	tools := r.List()
	if len(tools) != 2 {
		t.Fatalf("want 2 tools, got %d", len(tools))
	}
	// registration order preserved
	if tools[0].Name != "tool_a" || tools[1].Name != "tool_b" {
		t.Errorf("order wrong: %v", tools)
	}
}

func TestRegistry_RegisterDuplicate_Panics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic on duplicate registration")
		}
	}()
	r := New()
	r.Register(echoTool("dup"))
	r.Register(echoTool("dup"))
}

func TestRegistry_Dispatch_Success(t *testing.T) {
	r := New()
	r.Register(echoTool("my_tool"))
	got, err := r.Dispatch(context.Background(), "my_tool", json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if got != "result:my_tool" {
		t.Errorf("unexpected: %q", got)
	}
}

func TestRegistry_Dispatch_UnknownTool(t *testing.T) {
	r := New()
	_, err := r.Dispatch(context.Background(), "ghost", json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected error for unknown tool")
	}
	if !errors.Is(err, err) || err.Error() == "" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRegistry_ListBySource(t *testing.T) {
	r := New()
	r.Register(Tool{Name: "a", Source: "yahoo", Schema: json.RawMessage(`{}`), Handler: func(_ context.Context, _ json.RawMessage) (string, error) { return "", nil }})
	r.Register(Tool{Name: "b", Source: "edgar", Schema: json.RawMessage(`{}`), Handler: func(_ context.Context, _ json.RawMessage) (string, error) { return "", nil }})
	r.Register(Tool{Name: "c", Source: "yahoo", Schema: json.RawMessage(`{}`), Handler: func(_ context.Context, _ json.RawMessage) (string, error) { return "", nil }})

	yahoo := r.ListBySource("yahoo")
	if len(yahoo) != 2 {
		t.Errorf("want 2 yahoo tools, got %d", len(yahoo))
	}
	edgar := r.ListBySource("edgar")
	if len(edgar) != 1 {
		t.Errorf("want 1 edgar tool, got %d", len(edgar))
	}
}

func TestRegistry_Sources(t *testing.T) {
	r := New()
	r.Register(Tool{Name: "x", Source: "bls",   Schema: json.RawMessage(`{}`), Handler: func(_ context.Context, _ json.RawMessage) (string, error) { return "", nil }})
	r.Register(Tool{Name: "y", Source: "yahoo", Schema: json.RawMessage(`{}`), Handler: func(_ context.Context, _ json.RawMessage) (string, error) { return "", nil }})
	r.Register(Tool{Name: "z", Source: "bls",   Schema: json.RawMessage(`{}`), Handler: func(_ context.Context, _ json.RawMessage) (string, error) { return "", nil }})

	srcs := r.Sources()
	if len(srcs) != 2 {
		t.Errorf("want 2 unique sources, got %v", srcs)
	}
	// Sources are sorted
	if srcs[0] != "bls" || srcs[1] != "yahoo" {
		t.Errorf("want [bls yahoo], got %v", srcs)
	}
}

func TestRegistry_Logger_CalledOnDispatch(t *testing.T) {
	r := New()
	r.Register(echoTool("logged_tool"))

	var logged []string
	r.SetLogger(&captureLogger{fn: func(tool, _ string, _ json.RawMessage, _ string, _ error, _ time.Duration) {
		logged = append(logged, tool)
	}})

	r.Dispatch(context.Background(), "logged_tool", json.RawMessage(`{}`))
	r.Dispatch(context.Background(), "logged_tool", json.RawMessage(`{}`))

	if len(logged) != 2 {
		t.Errorf("expected 2 log calls, got %d", len(logged))
	}
}

func TestRegistry_Logger_NotCalledWhenNil(t *testing.T) {
	r := New()
	r.Register(echoTool("tool"))
	// No SetLogger — must not panic
	_, err := r.Dispatch(context.Background(), "tool", json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
}

// captureLogger implements CallLogger for testing.
type captureLogger struct {
	fn func(tool, source string, args json.RawMessage, result string, err error, d time.Duration)
}

func (c *captureLogger) Log(tool, source string, args json.RawMessage, result string, err error, d time.Duration) {
	c.fn(tool, source, args, result, err, d)
}
