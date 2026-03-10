// Package extract registers all channel extractors.
package extract

import "github.com/jsoprych/elko-market-mcp/internal/channel"

// RegisterAll registers every source extractor with the runner.
func RegisterAll(r *channel.Runner) {
	RegisterYahoo(r)
	RegisterEDGAR(r)
	RegisterTreasury(r)
	RegisterBLS(r)
	RegisterFDIC(r)
	RegisterWorldBank(r)
	RegisterFRED(r)
}
