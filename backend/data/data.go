// Package data embeds the authored seed content used by Fanti.
package data

import "embed"

// SeedFS holds the authored JSON fixtures under seed/.
//
//go:embed seed/*.json
var SeedFS embed.FS
