// Package channels embeds all JSON channel spec files.
package channels

import "embed"

//go:embed yahoo/*.json edgar/*.json treasury/*.json bls/*.json fdic/*.json worldbank/*.json
var FS embed.FS
