// Command elko is the elko-market-mcp CLI.
// Modes: mcp (stdio), serve (REST), and one-shot data commands.
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/jsoprych/elko-market-mcp/channels"
	"github.com/jsoprych/elko-market-mcp/internal/api"
	"github.com/jsoprych/elko-market-mcp/internal/cache"
	"github.com/jsoprych/elko-market-mcp/internal/calllog"
	"github.com/jsoprych/elko-market-mcp/internal/channel"
	"github.com/jsoprych/elko-market-mcp/internal/channel/extract"
	"github.com/jsoprych/elko-market-mcp/internal/mcp"
	"github.com/jsoprych/elko-market-mcp/internal/registry"
)

const version = "0.1.0"

// Global flags
var (
	flagDB           string
	flagSources      string
	flagPort         int
	flagLogMaxOutput int
)

func main() {
	root := &cobra.Command{
		Use:     "elko",
		Short:   "elko-market-mcp — free financial data via MCP, REST, and CLI",
		Version: version,
	}

	root.PersistentFlags().StringVar(&flagDB, "db", "", "SQLite database path (enables L2 cache + call logging)")
	root.PersistentFlags().StringVar(&flagSources, "sources", "all", "Comma-separated sources to enable: yahoo,edgar,treasury,bls,fdic,worldbank,all")
	root.PersistentFlags().IntVar(&flagPort, "port", 8080, "HTTP port for serve mode")
	root.PersistentFlags().IntVar(&flagLogMaxOutput, "log-max-output", calllog.DefaultMaxOutput, "Max result characters stored per log entry (0 = unlimited)")

	root.AddCommand(mcpCmd(), serveCmd(), catalogueCmd(), callCmd())

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// ── Wiring ────────────────────────────────────────────────────────────────────

func buildRegistry(sources string) (*registry.Registry, *sql.DB, *calllog.Logger, error) {
	var db *sql.DB
	if flagDB != "" {
		var err error
		db, err = cache.OpenSQLite(flagDB)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("open db: %w", err)
		}
	}

	c := cache.New(db)
	reg := registry.New()

	runner := channel.NewRunner(c)
	extract.RegisterAll(runner)

	allSpecs, err := channel.LoadFS(channels.FS)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("load channel specs: %w", err)
	}

	enabled := parseSourceSet(sources)
	var filtered []channel.Spec
	for _, s := range allSpecs {
		if enabled[s.Source] {
			filtered = append(filtered, s)
		}
	}

	if err := runner.Register(reg, filtered); err != nil {
		return nil, nil, nil, fmt.Errorf("register channels: %w", err)
	}

	logger := calllog.New(db, flagLogMaxOutput)
	reg.SetLogger(logger)

	return reg, db, logger, nil
}

func parseSourceSet(s string) map[string]bool {
	all := []string{"yahoo", "edgar", "treasury", "bls", "fdic", "worldbank", "fred"}
	m := make(map[string]bool)
	parts := strings.Split(strings.ToLower(s), ",")
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "all" {
			for _, src := range all {
				m[src] = true
			}
			return m
		}
		m[p] = true
	}
	return m
}

// ── Commands ──────────────────────────────────────────────────────────────────

func mcpCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "mcp",
		Short: "Start MCP stdio server (for Claude Desktop, Cursor, etc.)",
		RunE: func(cmd *cobra.Command, args []string) error {
			reg, _, _, err := buildRegistry(flagSources)
			if err != nil {
				return err
			}
			ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()
			return mcp.New(reg, version).Serve(ctx)
		},
	}
}

func serveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Start REST API server",
		RunE: func(cmd *cobra.Command, args []string) error {
			reg, _, logger, err := buildRegistry(flagSources)
			if err != nil {
				return err
			}
			mcpSrv := mcp.New(reg, version)
			srv := &http.Server{
				Addr: fmt.Sprintf(":%d", flagPort),
				Handler: api.New(reg, version).
					WithWebRoot("./web").
					WithMCPHandler(mcpSrv.HTTPHandler()).
					WithLogger(logger).
					Handler(),
			}
			ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()

			fmt.Fprintf(os.Stderr, "elko-market-mcp v%s listening on :%d\n", version, flagPort)
			go func() {
				<-ctx.Done()
				srv.Shutdown(context.Background())
			}()
			if err := srv.ListenAndServe(); err != http.ErrServerClosed {
				return err
			}
			return nil
		},
	}
}

func catalogueCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "catalogue",
		Short: "Print available tools and their descriptions",
		RunE: func(cmd *cobra.Command, args []string) error {
			reg, _, _, err := buildRegistry(flagSources)
			if err != nil {
				return err
			}
			tools := reg.List()
			fmt.Printf("%-35s  %-10s  %-10s  %s\n", "Tool", "Source", "Category", "Description")
			fmt.Println(strings.Repeat("-", 100))
			for _, t := range tools {
				desc := t.Description
				if len(desc) > 60 {
					desc = desc[:57] + "..."
				}
				fmt.Printf("%-35s  %-10s  %-10s  %s\n", t.Name, t.Source, t.Category, desc)
			}
			fmt.Printf("\n%d tools registered.\n", len(tools))
			return nil
		},
	}
}

func callCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "call <tool> [key=value ...]",
		Short: "Invoke a tool directly (one-shot)",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			reg, _, _, err := buildRegistry(flagSources)
			if err != nil {
				return err
			}
			toolName := args[0]

			// Build JSON args from key=value pairs.
			// Coerce "true"/"false" to booleans and numeric strings to numbers
			// so that downstream JSON unmarshal into typed structs works correctly.
			m := make(map[string]any)
			for _, kv := range args[1:] {
				parts := strings.SplitN(kv, "=", 2)
				if len(parts) != 2 {
					continue
				}
				k, v := parts[0], parts[1]
				switch v {
				case "true":
					m[k] = true
				case "false":
					m[k] = false
				default:
					// Coerce numeric strings so int/float fields unmarshal correctly.
					if n, err := strconv.ParseFloat(v, 64); err == nil {
						m[k] = n
					} else {
						m[k] = v
					}
				}
			}

			var argsJSON []byte
			if len(m) > 0 {
				argsJSON, _ = json.Marshal(m)
			} else {
				argsJSON = []byte(`{}`)
			}

			result, err := reg.Dispatch(context.Background(), toolName, argsJSON)
			if err != nil {
				return err
			}
			fmt.Print(result)
			return nil
		},
	}
	return cmd
}
