package db

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrations embed.FS

// Open connects and verifies the pool. Parse errors are not wrapped because
// pgx echoes the connection string in them; only host and database name are
// ever reported.
func Open(ctx context.Context, url string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, errors.New("DATABASE_URL tidak valid")
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("database %s/%s: gagal membuat pool", cfg.ConnConfig.Host, cfg.ConnConfig.Database)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("database %s/%s: %w", cfg.ConnConfig.Host, cfg.ConnConfig.Database, err)
	}
	return pool, nil
}

// Describe is the only form of the connection that may appear in logs.
func Describe(pool *pgxpool.Pool) string {
	if pool == nil {
		return "(tidak terhubung)"
	}
	c := pool.Config().ConnConfig
	return fmt.Sprintf("%s:%d/%s", c.Host, c.Port, c.Database)
}

// Migrate applies every pending migration. It runs on each start so a fresh
// database and an existing one end up on the same schema.
func Migrate(ctx context.Context, pool *pgxpool.Pool) ([]string, error) {
	sqlDB := stdlib.OpenDBFromPool(pool)
	defer sqlDB.Close()

	fsys, err := fs.Sub(migrations, "migrations")
	if err != nil {
		return nil, err
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, sqlDB, fsys)
	if err != nil {
		return nil, fmt.Errorf("siapkan migrasi: %w", err)
	}
	results, err := provider.Up(ctx)
	if err != nil {
		return nil, fmt.Errorf("migrasi gagal: %w", err)
	}

	applied := make([]string, 0, len(results))
	for _, r := range results {
		applied = append(applied, r.Source.Path)
	}
	return applied, nil
}
