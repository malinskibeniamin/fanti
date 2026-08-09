package main

import (
	"archive/zip"
	"compress/bzip2"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/malinskibeniamin/fanti/backend/internal/config"
	"github.com/malinskibeniamin/fanti/backend/internal/convert"
	"github.com/malinskibeniamin/fanti/backend/internal/db"
	"github.com/malinskibeniamin/fanti/backend/internal/seed"
)

type migrateCmd struct{}

func (migrateCmd) Run() error {
	cfg, err := config.Load(nil)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	if err := db.Migrate(cfg.DatabaseURL); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	return nil
}

type seedCmd struct {
	DownloadDir      string `default:"data/downloads"                                                 help:"Cache directory for downloaded datasets."`
	IfEmpty          bool   `help:"Skip seeding when all required bootstrap data is already present."`
	SkipCedict       bool   `help:"Skip the CC-CEDICT dictionary load."`
	SkipStrokes      bool   `help:"Skip the hanzi-writer stroke data load."`
	SkipFrequency    bool   `help:"Skip the character frequency rank load."`
	SkipUnihan       bool   `help:"Skip the Unihan long-tail reading backfill."`
	SkipTatoeba      bool   `help:"Skip the Tatoeba example sentences load."`
	SkipHistory      bool   `help:"Skip the optional Wikimedia character-history import."`
	HistoryRankLimit int    `default:"500"                                                            help:"Import history through this character frequency rank."`
	RefreshHistory   bool   `help:"Recheck character-history forms already imported."`
	Classics         bool   `help:"Fetch the Project Gutenberg classics (large download)."`
	ForceClassics    bool   `help:"Re-seed the classics even when their chapters already exist."`

	ForceExamplesFill bool `help:"Re-pick auto-filled example sentences (authored examples stay untouched)."`
}

const fullSeedRun = "full-v4"

func (c seedCmd) Run() error {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	ctx := context.Background()

	cfg, err := config.Load(nil)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	if err := db.Migrate(cfg.DatabaseURL); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer pool.Close()

	if c.IfEmpty {
		present, err := seedDataPresent(ctx, pool)
		if err != nil {
			return fmt.Errorf("check seed data: %w", err)
		}
		if present {
			c.seedCharacterHistory(ctx, pool, logger)
			logger.InfoContext(ctx, "seed data already present, skipping")

			return nil
		}
	}

	sources, cleanup, err := c.openSources(ctx)
	defer cleanup()

	if err != nil {
		return err
	}

	if err := seed.Run(ctx, pool, sources, logger); err != nil {
		return fmt.Errorf("seed: %w", err)
	}

	c.seedCharacterHistory(ctx, pool, logger)

	var engine *convert.Engine
	if !c.SkipTatoeba || c.Classics {
		if engine, err = convert.NewEngine(); err != nil {
			return fmt.Errorf("load convert engine: %w", err)
		}
	}

	// Tatoeba sentences load after the Tatoeba-derived frequency step because
	// sentence grading reads the ranks that step just assigned.
	if !c.SkipTatoeba {
		if err := seedTatoebaFromDerivative(ctx, pool, engine, c, logger); err != nil {
			return err
		}
	}

	// The fill runs even when the Tatoeba load is skipped: seed.Run just
	// reset the authored fixtures' examples, and any previously loaded
	// sentences can repopulate the auto-filled ones.
	if err := seed.FillExamples(ctx, pool, c.ForceExamplesFill, logger); err != nil {
		return fmt.Errorf("fill examples: %w", err)
	}

	if c.Classics {
		if err := seed.SeedGutenberg(ctx, pool, engine, c.DownloadDir, c.ForceClassics, logger); err != nil {
			return fmt.Errorf("seed classics: %w", err)
		}
	}

	// Vendor the Arphic licence beside the download cache so the data's
	// terms travel with it (see NOTICES.md).
	if sources.StrokesTarball != nil {
		if err := vendorArphicLicence(c.DownloadDir); err != nil {
			logger.WarnContext(ctx, "vendor arphic licence", slog.Any("error", err))
		}
	}

	if !c.SkipCedict && !c.SkipStrokes && !c.SkipFrequency && !c.SkipUnihan && !c.SkipTatoeba {
		if err := markSeedDataPresent(ctx, pool); err != nil {
			return fmt.Errorf("record seed completion: %w", err)
		}
	}

	logger.InfoContext(ctx, "seed complete")

	return nil
}

func (c seedCmd) characterHistoryOptions() seed.CharacterHistoryOptions {
	return seed.CharacterHistoryOptions{
		CacheDir:  c.DownloadDir,
		RankLimit: c.HistoryRankLimit,
		Refresh:   c.RefreshHistory,
	}
}

func (c seedCmd) seedCharacterHistory(
	ctx context.Context,
	pool *pgxpool.Pool,
	logger *slog.Logger,
) {
	if c.SkipHistory {
		return
	}

	seedCharacterHistoryBestEffort(ctx, pool, c.characterHistoryOptions(), logger)
}

func seedCharacterHistoryBestEffort(
	ctx context.Context,
	pool *pgxpool.Pool,
	options seed.CharacterHistoryOptions,
	logger *slog.Logger,
) {
	if err := seed.SeedCharacterHistory(ctx, pool, options, logger); err != nil {
		logger.WarnContext(ctx, "character history import deferred", slog.Any("error", err))
	}
}

func seedDataPresent(ctx context.Context, pool *pgxpool.Pool) (bool, error) {
	var present bool
	err := pool.QueryRow(ctx,
		"SELECT EXISTS (SELECT 1 FROM seed_runs WHERE name = $1)", fullSeedRun).Scan(&present)

	return present, err
}

func markSeedDataPresent(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, `
		INSERT INTO seed_runs (name) VALUES ($1)
		ON CONFLICT (name) DO UPDATE SET completed_at = now()`, fullSeedRun)

	return err
}

// openSources downloads and opens the enabled external datasets. The
// returned cleanup closes every opened reader and is safe to call even
// when an error is returned.
func (c seedCmd) openSources(ctx context.Context) (seed.Sources, func(), error) {
	var (
		sources seed.Sources
		closers []io.Closer
	)

	cleanup := func() {
		for _, cl := range closers {
			_ = cl.Close()
		}
	}

	open := func(filename, url string) (*os.File, error) {
		path, err := seed.DownloadCached(ctx, c.DownloadDir, filename, url)
		if err != nil {
			return nil, fmt.Errorf("download %s: %w", filename, err)
		}

		f, err := os.Open(path) //nolint:gosec // operator-controlled cache dir
		if err != nil {
			return nil, fmt.Errorf("open %s: %w", path, err)
		}

		closers = append(closers, f)

		return f, nil
	}

	if !c.SkipCedict {
		f, err := open("cedict.txt.gz", seed.CEDICTURL)
		if err != nil {
			return sources, cleanup, err
		}

		gz, err := gzip.NewReader(f)
		if err != nil {
			return sources, cleanup, fmt.Errorf("gunzip cedict: %w", err)
		}

		closers = append(closers, gz)
		sources.CEDICT = gz
	}

	if !c.SkipStrokes {
		f, err := open("hanzi-writer-data.tgz", seed.StrokesURL)
		if err != nil {
			return sources, cleanup, err
		}

		sources.StrokesTarball = f

		f, err = open("makemeahanzi-dictionary.txt", seed.DecompositionsURL)
		if err != nil {
			return sources, cleanup, err
		}

		sources.Decompositions = f
	}

	if !c.SkipFrequency {
		path := filepath.Join(c.DownloadDir, "tatoeba_cmn_eng.tsv.gz")

		f, err := os.Open(path) //nolint:gosec // operator-controlled cache dir
		if err != nil {
			return sources, cleanup, fmt.Errorf(
				"open Tatoeba frequency corpus (run `fanti tatoeba-prepare` to build it): %w", err,
			)
		}

		closers = append(closers, f)

		gz, err := gzip.NewReader(f)
		if err != nil {
			return sources, cleanup, fmt.Errorf("gunzip Tatoeba frequency corpus: %w", err)
		}

		closers = append(closers, gz)
		sources.Frequency = gz
	}

	if !c.SkipUnihan {
		path, err := seed.DownloadCached(ctx, c.DownloadDir, "unihan.zip", seed.UnihanURL)
		if err != nil {
			return sources, cleanup, fmt.Errorf("download unihan: %w", err)
		}

		archive, err := zip.OpenReader(path)
		if err != nil {
			return sources, cleanup, fmt.Errorf("open unihan zip: %w", err)
		}

		closers = append(closers, archive)

		readings, err := archive.Open("Unihan_Readings.txt")
		if err != nil {
			return sources, cleanup, fmt.Errorf("open unihan readings: %w", err)
		}

		closers = append(closers, readings)
		sources.UnihanReadings = readings
	}

	return sources, cleanup, nil
}

// seedTatoebaFromDerivative loads the vendored sentence pairs and fills
// example-less characters from them. The derivative ships in the repo;
// when absent, `fanti tatoeba-prepare` rebuilds it.
func seedTatoebaFromDerivative(
	ctx context.Context, pool *pgxpool.Pool, engine *convert.Engine, c seedCmd, logger *slog.Logger,
) error {
	path := filepath.Join(c.DownloadDir, "tatoeba_cmn_eng.tsv.gz")

	f, err := os.Open(path) //nolint:gosec // operator-controlled cache dir
	if err != nil {
		return fmt.Errorf("open tatoeba derivative (run `fanti tatoeba-prepare` to build it): %w", err)
	}

	defer func() { _ = f.Close() }()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("gunzip tatoeba derivative: %w", err)
	}

	defer func() { _ = gz.Close() }()

	if err := seed.SeedTatoeba(ctx, pool, engine, gz, logger); err != nil {
		return fmt.Errorf("seed tatoeba: %w", err)
	}

	return nil
}

type tatoebaPrepareCmd struct {
	DownloadDir string `default:"data/downloads"                        help:"Cache directory for downloaded datasets."`
	Out         string `default:"data/downloads/tatoeba_cmn_eng.tsv.gz" help:"Vendored derivative destination."`
}

// Run builds the vendored Tatoeba derivative: Mandarin sentences joined to
// one English translation each. The raw exports land in the (gitignored)
// download cache; only the compact joined file is meant to be committed.
func (c tatoebaPrepareCmd) Run() error {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	ctx := context.Background()

	exports := []struct {
		filename string
		url      string
	}{
		{"tatoeba_cmn_sentences.tsv.bz2", seed.TatoebaCmnURL},
		{"tatoeba_cmn_eng_links.tsv.bz2", seed.TatoebaLinksURL},
		{"tatoeba_eng_sentences.tsv.bz2", seed.TatoebaEngURL},
	}

	readers := make([]io.Reader, len(exports))

	for i, e := range exports {
		path, err := seed.DownloadCached(ctx, c.DownloadDir, e.filename, e.url)
		if err != nil {
			return fmt.Errorf("download %s: %w", e.filename, err)
		}

		f, err := os.Open(path) //nolint:gosec // operator-controlled cache dir
		if err != nil {
			return fmt.Errorf("open %s: %w", path, err)
		}

		defer func() { _ = f.Close() }()

		readers[i] = bzip2.NewReader(f)
	}

	tmp := c.Out + ".tmp"

	out, err := os.Create(tmp) //nolint:gosec // operator-controlled destination
	if err != nil {
		return fmt.Errorf("create %s: %w", tmp, err)
	}

	gz := gzip.NewWriter(out)

	rows, err := seed.PrepareTatoeba(readers[0], readers[1], readers[2], gz)
	if err != nil {
		_ = out.Close()
		_ = os.Remove(tmp)

		return fmt.Errorf("prepare tatoeba: %w", err)
	}

	if err := gz.Close(); err != nil {
		return fmt.Errorf("close gzip: %w", err)
	}

	if err := out.Close(); err != nil {
		return fmt.Errorf("close %s: %w", tmp, err)
	}

	if err := os.Rename(tmp, c.Out); err != nil {
		return fmt.Errorf("finalize %s: %w", c.Out, err)
	}

	logger.InfoContext(ctx, "tatoeba derivative written",
		slog.String("path", c.Out), slog.Int("pairs", rows))

	return nil
}

func vendorArphicLicence(downloadDir string) error {
	tgz, err := os.Open(filepath.Join(downloadDir, "hanzi-writer-data.tgz")) //nolint:gosec // operator path
	if err != nil {
		return fmt.Errorf("open tarball: %w", err)
	}

	defer func() { _ = tgz.Close() }()

	_, licence, err := seed.ExtractStrokeData(tgz)
	if err != nil {
		return err
	}

	dest := filepath.Join("data", "strokes")
	if err := os.MkdirAll(dest, 0o750); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	if err := os.WriteFile(filepath.Join(dest, "ARPHICPL.TXT"), []byte(licence), 0o644); err != nil { //nolint:gosec // licence text
		return fmt.Errorf("write licence: %w", err)
	}

	return nil
}
