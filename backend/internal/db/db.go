// Package db owns the Postgres connection pool and goose migrations.
package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib" // database/sql driver for goose
	"github.com/pressly/goose/v3"

	"github.com/malinskibeniamin/fanti/backend/migrations"
)

// Connect opens a pgx pool and verifies connectivity.
func Connect(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("open pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()

		return nil, fmt.Errorf("ping: %w", err)
	}

	return pool, nil
}

// Migrate applies all pending goose migrations.
func Migrate(dsn string) error {
	return withGoose(dsn, func(sqlDB *sql.DB) error {
		return goose.Up(sqlDB, ".")
	})
}

// MigrateDownTo rolls back migrations down to the given version.
func MigrateDownTo(dsn string, version int64) error {
	return withGoose(dsn, func(sqlDB *sql.DB) error {
		return goose.DownTo(sqlDB, ".", version)
	})
}

func withGoose(dsn string, run func(*sql.DB) error) error {
	goose.SetBaseFS(migrations.FS)
	goose.SetLogger(goose.NopLogger())

	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set dialect: %w", err)
	}

	sqlDB, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("open sql db: %w", err)
	}

	defer func() { _ = sqlDB.Close() }()

	if err := run(sqlDB); err != nil {
		return fmt.Errorf("goose: %w", err)
	}

	return nil
}
