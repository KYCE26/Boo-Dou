// Package testdb gives tests a migrated, empty database from DATABASE_URL_TEST.
package testdb

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"boodou/backend/internal/db"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

const lockKey = 7_420_011

// URL reads DATABASE_URL_TEST from the environment, falling back to
// backend/.env so `go test ./...` works without exporting anything.
func URL() string {
	if v := strings.TrimSpace(os.Getenv("DATABASE_URL_TEST")); v != "" {
		return v
	}
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			env, err := godotenv.Read(filepath.Join(dir, ".env"))
			if err != nil {
				return ""
			}
			return strings.TrimSpace(env["DATABASE_URL_TEST"])
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// Open skips the test when no test database is configured, and refuses any
// database whose name does not end in _test, because every call wipes it.
func Open(t testing.TB) *pgxpool.Pool {
	t.Helper()
	url := URL()
	if url == "" {
		t.Skip("DATABASE_URL_TEST kosong: tes database dilewati. Isi di backend/.env atau environment.")
	}

	ctx := context.Background()
	pool, err := db.Open(ctx, url)
	if err != nil {
		t.Fatalf("buka database tes: %v", err)
	}
	t.Cleanup(pool.Close)

	if name := pool.Config().ConnConfig.Database; !strings.HasSuffix(name, "_test") {
		t.Fatalf("DATABASE_URL_TEST menunjuk ke database %q; nama harus berakhiran _test karena tes mengosongkannya", name)
	}

	// go test runs packages in parallel against the same database; without
	// this lock one package's TRUNCATE would wipe another's rows mid-test.
	lockConn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("ambil koneksi lock: %v", err)
	}
	if _, err := lockConn.Exec(ctx, `SELECT pg_advisory_lock($1)`, lockKey); err != nil {
		t.Fatalf("advisory lock: %v", err)
	}
	t.Cleanup(func() {
		_, _ = lockConn.Exec(context.Background(), `SELECT pg_advisory_unlock($1)`, lockKey)
		lockConn.Release()
	})

	if _, err := db.Migrate(ctx, pool); err != nil {
		t.Fatalf("migrasi database tes: %v", err)
	}
	if _, err := pool.Exec(ctx, `TRUNCATE conversations, messages, attachments CASCADE`); err != nil {
		t.Fatalf("kosongkan database tes: %v", err)
	}
	return pool
}
