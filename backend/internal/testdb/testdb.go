// Package testdb boots throwaway embedded Postgres instances for
// integration tests.
package testdb

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	embeddedpostgres "github.com/fergusstrange/embedded-postgres"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/malinskibeniamin/fanti/backend/internal/db"
)

// Start boots an embedded Postgres for the test and returns its DSN.
func Start(t *testing.T) string {
	t.Helper()

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("pick port: %v", err)
	}

	port := lis.Addr().(*net.TCPAddr).Port
	_ = lis.Close()

	pg := embeddedpostgres.NewDatabase(embeddedpostgres.DefaultConfig().
		Port(uint32(port)).
		DataPath(t.TempDir() + "/pg").
		RuntimePath(t.TempDir() + "/pg-runtime").
		// UTF-8 like the production compose database — char_length must
		// count characters, not bytes, for CJK text.
		Locale("en_US.UTF-8").
		StartTimeout(60 * time.Second))
	if err := pg.Start(); err != nil {
		t.Fatalf("start embedded postgres: %v", err)
	}

	t.Cleanup(func() {
		if stopErr := pg.Stop(); stopErr != nil {
			t.Errorf("stop embedded postgres: %v", stopErr)
		}
	})

	return fmt.Sprintf("postgres://postgres:postgres@127.0.0.1:%d/postgres?sslmode=disable", port)
}

// StartMigrated boots Postgres, applies all migrations, and returns a pool.
func StartMigrated(t *testing.T) *pgxpool.Pool {
	t.Helper()

	dsn := Start(t)

	if err := db.Migrate(dsn); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	pool, err := db.Connect(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}

	t.Cleanup(pool.Close)

	return pool
}
