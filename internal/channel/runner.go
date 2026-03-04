package channel

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/jsoprych/elko-market-mcp/internal/cache"
	"github.com/jsoprych/elko-market-mcp/internal/registry"
)

// Channel bundles a parsed Spec with a ready-to-use Fetch function.
// Extractors call ch.Fetch() for all HTTP; the runner handles caching,
// header merging (including env-var-sourced), and error formatting.
type Channel struct {
	Spec  Spec
	Fetch func(ctx context.Context, rawURL string) ([]byte, error)
}

// ExtractorFunc handles URL-building, fetch calls, response parsing, and output formatting.
type ExtractorFunc func(ctx context.Context, args json.RawMessage, ch *Channel) (string, error)

// Runner holds the shared HTTP client, cache, and extractor registry.
type Runner struct {
	http       *http.Client
	cache      *cache.Cache
	extractors map[string]ExtractorFunc
}

// NewRunner creates a Runner with a shared HTTP client and the provided cache.
func NewRunner(c *cache.Cache) *Runner {
	return &Runner{
		http:       &http.Client{Timeout: 20 * time.Second},
		cache:      c,
		extractors: make(map[string]ExtractorFunc),
	}
}

// RegisterExtractor registers a named extractor function.
func (r *Runner) RegisterExtractor(name string, fn ExtractorFunc) {
	r.extractors[name] = fn
}

// Register resolves specs to extractors and populates the tool registry.
// Returns an error if any spec references an unknown extractor.
func (r *Runner) Register(reg *registry.Registry, specs []Spec) error {
	for _, s := range specs {
		fn, ok := r.extractors[s.Response.Extractor]
		if !ok {
			return fmt.Errorf("channel %q: unknown extractor %q", s.Name, s.Response.Extractor)
		}
		ch := &Channel{
			Spec:  s,
			Fetch: r.makeFetch(s),
		}
		// Capture loop variables for closure.
		spec := s
		extractor := fn
		_ = spec
		reg.Register(registry.Tool{
			Name:         s.Name,
			Description:  s.Description,
			Schema:       s.Schema,
			Source:       s.Source,
			Category:     s.Category,
			ResultFormat: s.ResultFormat,
			Chart:        s.Chart,
			Handler: func(ctx context.Context, args json.RawMessage) (string, error) {
				return extractor(ctx, args, ch)
			},
		})
	}
	return nil
}

// makeFetch returns a closure that: checks cache → performs HTTP GET with merged
// headers (static + env-var-sourced) → stores result in cache.
func (r *Runner) makeFetch(spec Spec) func(ctx context.Context, rawURL string) ([]byte, error) {
	ttl := spec.Request.ParseTTL()
	cachePrefix := spec.Source + ":"
	return func(ctx context.Context, rawURL string) ([]byte, error) {
		cacheKey := cachePrefix + rawURL
		if b, ok := r.cache.Get(cacheKey); ok {
			return b, nil
		}
		req, err := http.NewRequestWithContext(ctx, "GET", rawURL, nil)
		if err != nil {
			return nil, err
		}
		// Apply static headers first.
		for k, v := range spec.Request.Headers {
			req.Header.Set(k, v)
		}
		// Env-var-sourced headers override static values when the env var is set.
		for header, envVar := range spec.Request.EnvHeaders {
			if val := os.Getenv(envVar); val != "" {
				req.Header.Set(header, val)
			}
		}
		resp, err := r.http.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("%s HTTP %d: %s", spec.Source, resp.StatusCode, strings.TrimSpace(string(body)))
		}
		r.cache.Set(cacheKey, body, ttl)
		return body, nil
	}
}
