// Package channel provides the data-driven channel architecture:
// JSON specs define what a channel is; Go extractors define how it fetches and parses.
package channel

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"strings"
	"time"
)

// Spec describes a single tool channel — name, schema, HTTP config, extractor reference.
type Spec struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Source      string          `json:"source"`
	Category    string          `json:"category"`
	Schema      json.RawMessage `json:"schema"`
	Request     RequestSpec     `json:"request"`
	Response    ResponseSpec    `json:"response"`
}

// RequestSpec holds HTTP connection config for a channel.
type RequestSpec struct {
	BaseURL    string            `json:"base_url"`
	Headers    map[string]string `json:"headers"`
	EnvHeaders map[string]string `json:"env_headers"` // headerName → envVarName
	TTL        string            `json:"ttl"`
}

// ParseTTL parses the TTL string (e.g. "1h", "5m", "24h"). Defaults to 1h on error.
func (r RequestSpec) ParseTTL() time.Duration {
	d, err := time.ParseDuration(r.TTL)
	if err != nil || d <= 0 {
		return time.Hour
	}
	return d
}

// ResponseSpec names the extractor that handles this channel's response.
type ResponseSpec struct {
	Extractor string `json:"extractor"`
}

// LoadFS walks an fs.FS and returns all *.json Spec files found.
func LoadFS(fsys fs.FS) ([]Spec, error) {
	var specs []Spec
	err := fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".json") {
			return nil
		}
		data, err := fs.ReadFile(fsys, path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		var s Spec
		if err := json.Unmarshal(data, &s); err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
		specs = append(specs, s)
		return nil
	})
	return specs, err
}
