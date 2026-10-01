// Package postgres provides the PostgreSQL-backed storage layer: connection
// pooling, migrations, and the Repository used by the rest of the backend.
package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" database/sql driver used by goose
	"github.com/pressly/goose/v3"

	"homecinema/migrations"
)

// Connect opens a pgx connection pool to the given Postgres DSN.
func Connect(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("postgres: connect: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("postgres: ping: %w", err)
	}
	return pool, nil
}

// Migrate applies all pending goose migrations embedded in the migrations package.
func Migrate(databaseURL string) error {
	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("postgres: set dialect: %w", err)
	}

	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return fmt.Errorf("postgres: open for migration: %w", err)
	}
	defer func() { _ = db.Close() }()

	if err := goose.Up(db, "."); err != nil {
		return fmt.Errorf("postgres: migrate: %w", err)
	}
	return nil
}
