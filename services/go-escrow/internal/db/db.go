// LR #5: Highload — connection pooling with MaxOpenConns/MaxIdleConns
// LR #6: DB — auto-migration on startup, prepared statements in repository layer

package db

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"time"

	_ "github.com/lib/pq"
)

//go:embed migrations/001_initial.sql
var migrationSQL string

type Database struct {
	DB *sql.DB
}

func Connect(dsn string) (*Database, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("sql.Open: %w", err)
	}

	// LR #5: Connection pool tuning for highload
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetConnMaxIdleTime(1 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("db.Ping: %w", err)
	}

	if err := runMigrations(db); err != nil {
		return nil, fmt.Errorf("migrations: %w", err)
	}

	return &Database{DB: db}, nil
}

func runMigrations(db *sql.DB) error {
	_, err := db.Exec(migrationSQL)
	if err != nil {
		return fmt.Errorf("exec migration: %w", err)
	}
	return nil
}

func (d *Database) Close() error {
	return d.DB.Close()
}

// WithTx wraps fn in a database transaction. Rolls back on error, commits on success.
// LR #9: Context-based transaction with defer rollback pattern.
func WithTx(db *sql.DB, opts *sql.TxOptions, fn func(tx *sql.Tx) error) error {
	tx, err := db.BeginTx(context.Background(), opts)
	if err != nil {
		return fmt.Errorf("BeginTx: %w", err)
	}
	defer tx.Rollback()

	if err := fn(tx); err != nil {
		return err
	}

	return tx.Commit()
}
