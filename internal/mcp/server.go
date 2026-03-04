// Package mcp implements a Model Context Protocol server over stdio (JSON-RPC 2.0).
// Spec: https://spec.modelcontextprotocol.io
package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/jsoprych/elko-market-mcp/internal/registry"
)

// Server is a stdio MCP server.
type Server struct {
	reg     *registry.Registry
	version string
}

func New(reg *registry.Registry, version string) *Server {
	return &Server{reg: reg, version: version}
}

// Serve runs the MCP loop until EOF or context cancellation.
func (s *Server) Serve(ctx context.Context) error {
	return s.ServeIO(ctx, os.Stdin, os.Stdout)
}

func (s *Server) ServeIO(ctx context.Context, r io.Reader, w io.Writer) error {
	enc := json.NewEncoder(w)
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 4*1024*1024), 4*1024*1024)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var req rpcRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			enc.Encode(errResp(nil, -32700, "parse error", nil))
			continue
		}

		resp := s.handle(ctx, &req)
		if resp != nil {
			enc.Encode(resp)
		}
	}
	return scanner.Err()
}

// ── JSON-RPC types ────────────────────────────────────────────────────────────

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *rpcError   `json:"error,omitempty"`
}

type rpcError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func okResp(id interface{}, result interface{}) *rpcResponse {
	return &rpcResponse{JSONRPC: "2.0", ID: id, Result: result}
}

func errResp(id interface{}, code int, msg string, data interface{}) *rpcResponse {
	return &rpcResponse{JSONRPC: "2.0", ID: id, Error: &rpcError{Code: code, Message: msg, Data: data}}
}

// ── Dispatch ──────────────────────────────────────────────────────────────────

func (s *Server) handle(ctx context.Context, req *rpcRequest) *rpcResponse {
	switch req.Method {
	case "initialize":
		return s.handleInitialize(req)
	case "notifications/initialized":
		return nil // no response for notifications
	case "tools/list":
		return s.handleToolsList(req)
	case "tools/call":
		return s.handleToolsCall(ctx, req)
	case "ping":
		return okResp(req.ID, map[string]string{})
	default:
		return errResp(req.ID, -32601, fmt.Sprintf("method not found: %s", req.Method), nil)
	}
}

func (s *Server) handleInitialize(req *rpcRequest) *rpcResponse {
	return okResp(req.ID, map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"serverInfo": map[string]string{
			"name":    "elko-market-mcp",
			"version": s.version,
		},
		"capabilities": map[string]interface{}{
			"tools": map[string]interface{}{},
		},
	})
}

func (s *Server) handleToolsList(req *rpcRequest) *rpcResponse {
	tools := s.reg.List()
	mcpTools := make([]map[string]interface{}, 0, len(tools))
	for _, t := range tools {
		mcpTools = append(mcpTools, map[string]interface{}{
			"name":        t.Name,
			"description": t.Description,
			"inputSchema": t.Schema,
		})
	}
	return okResp(req.ID, map[string]interface{}{"tools": mcpTools})
}

func (s *Server) handleToolsCall(ctx context.Context, req *rpcRequest) *rpcResponse {
	var p struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params, &p); err != nil {
		return errResp(req.ID, -32602, "invalid params", err.Error())
	}
	if p.Arguments == nil {
		p.Arguments = json.RawMessage(`{}`)
	}

	result, err := s.reg.Dispatch(ctx, p.Name, p.Arguments)
	if err != nil {
		return okResp(req.ID, map[string]interface{}{
			"content": []map[string]interface{}{
				{"type": "text", "text": fmt.Sprintf("Error: %s", err.Error())},
			},
			"isError": true,
		})
	}

	return okResp(req.ID, map[string]interface{}{
		"content": []map[string]interface{}{
			{"type": "text", "text": result},
		},
	})
}
