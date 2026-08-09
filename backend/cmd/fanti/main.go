// Command fanti runs the Fanti 繁体 API server.
package main

import (
	"github.com/alecthomas/kong"
)

type cli struct {
	Serve   serveCmd   `cmd:"" default:"1"                                                  help:"Start the Fanti API server."`
	Migrate migrateCmd `cmd:"" help:"Apply database migrations."`
	Seed    seedCmd    `cmd:"" help:"Load dictionaries, stroke data, and authored content."`

	TatoebaPrepare tatoebaPrepareCmd `cmd:"" help:"Build the vendored Tatoeba sentence-pair derivative."`
}

func main() {
	app := kong.Parse(&cli{},
		kong.Name("fanti"),
		kong.Description("Fanti 繁体 · 玉簡閣 — Chinese reading & study API server."),
		kong.UsageOnError(),
	)
	app.FatalIfErrorf(app.Run())
}
