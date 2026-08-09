package db_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/pressly/goose/v3"

	"github.com/malinskibeniamin/fanti/backend/internal/db"
	"github.com/malinskibeniamin/fanti/backend/internal/testdb"
	"github.com/malinskibeniamin/fanti/backend/migrations"
)

func TestIntegrationMigrateUpDownUp(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}

	dsn := testdb.Start(t)

	if err := db.Migrate(dsn); err != nil {
		t.Fatalf("Migrate() up error = %v", err)
	}

	ctx := context.Background()

	pool, err := db.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	defer pool.Close()

	for _, table := range []string{
		"dict_entries", "characters", "char_pinyin", "stroke_data", "compounds",
		"word_cards", "books", "chapters", "conversions", "reviews",
		"practice_days", "learning_records", "cloze_cards", "study_profile",
		"quizzes", "milestones",
	} {
		var exists bool
		if err := pool.QueryRow(ctx,
			"SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = $1)",
			table).Scan(&exists); err != nil {
			t.Fatalf("check table %s: %v", table, err)
		}

		if !exists {
			t.Errorf("table %s missing after migrate", table)
		}
	}

	// The profile singleton row must exist.
	var goal string
	if err := pool.QueryRow(ctx, "SELECT goal FROM study_profile").Scan(&goal); err != nil {
		t.Fatalf("study_profile singleton: %v", err)
	}

	if goal != "practical" {
		t.Errorf("default goal = %q, want practical", goal)
	}

	// Down migrations must be real, and re-up must work.
	if err := db.MigrateDownTo(dsn, 0); err != nil {
		t.Fatalf("Migrate() down error = %v", err)
	}

	if err := db.Migrate(dsn); err != nil {
		t.Fatalf("Migrate() re-up error = %v", err)
	}
}

func TestIntegrationMigrateRepairsDivergentVersionFive(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}

	dsn := testdb.Start(t)

	sqlDB, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer func() { _ = sqlDB.Close() }()

	goose.SetBaseFS(migrations.FS)
	goose.SetLogger(goose.NopLogger())
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("set goose dialect: %v", err)
	}
	if err := goose.UpTo(sqlDB, ".", 4); err != nil {
		t.Fatalf("migrate through shared version four: %v", err)
	}

	ctx := context.Background()

	// PR #17 briefly used version five for stroke decompositions while main
	// used the same version for character history. Reproduce a database that
	// ran the PR branch through version seven before the histories converged.
	if _, err := sqlDB.ExecContext(ctx, `
		ALTER TABLE stroke_data
			ADD COLUMN radical_parts JSONB NOT NULL DEFAULT '[]';
		INSERT INTO goose_db_version (version_id, is_applied)
		VALUES (5, TRUE), (6, TRUE), (7, TRUE)`); err != nil {
		t.Fatalf("prepare divergent migration history: %v", err)
	}

	if err := db.Migrate(dsn); err != nil {
		t.Fatalf("repair divergent migration history: %v", err)
	}

	var historyExists, radicalPartsExists bool
	if err := sqlDB.QueryRowContext(ctx, `
		SELECT
			to_regclass('character_history') IS NOT NULL,
			EXISTS (
				SELECT 1
				FROM information_schema.columns
				WHERE table_name = 'stroke_data'
					AND column_name = 'radical_parts'
			)`).Scan(&historyExists, &radicalPartsExists); err != nil {
		t.Fatalf("inspect reconciled schema: %v", err)
	}

	if !historyExists || !radicalPartsExists {
		t.Fatalf("reconciled schema history=%t radical_parts=%t, want both true",
			historyExists, radicalPartsExists)
	}
}
