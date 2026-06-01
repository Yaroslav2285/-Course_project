// LR #13: Testing/Automation — PostgreSQL test helper for integration tests
// LR #6: DB — connects to test database with migrations and cleanup

package testutil

import (
	"database/sql"
	"fmt"
	"os"
	"time"

	_ "github.com/lib/pq"

	"github.com/marketplace/go-escrow/internal/db"
)

var migrationSQL = `
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS escrow_accounts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id UUID NOT NULL,
    balance NUMERIC(19,4) NOT NULL DEFAULT 0.00,
    status VARCHAR(32) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_escrow_balance_non_negative CHECK (balance >= 0)
);

CREATE INDEX IF NOT EXISTS idx_escrow_accounts_order_id ON escrow_accounts(order_id);
CREATE INDEX IF NOT EXISTS idx_escrow_accounts_status ON escrow_accounts(status);

CREATE TABLE IF NOT EXISTS escrow_transactions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    escrow_account_id UUID NOT NULL,
    order_id UUID NOT NULL,
    amount NUMERIC(19,4) NOT NULL,
    transaction_type VARCHAR(32) NOT NULL,
    status VARCHAR(32) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT fk_escrow_transaction_escrow FOREIGN KEY(escrow_account_id) REFERENCES escrow_accounts(id) ON DELETE CASCADE,
    CONSTRAINT chk_escrow_transaction_amount_positive CHECK (amount > 0)
);

CREATE INDEX IF NOT EXISTS idx_escrow_transactions_order_id ON escrow_transactions(order_id);
CREATE INDEX IF NOT EXISTS idx_escrow_transactions_status ON escrow_transactions(status);

CREATE TABLE IF NOT EXISTS disputes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id UUID NOT NULL,
    escrow_account_id UUID NOT NULL,
    reason TEXT NOT NULL,
    status VARCHAR(32) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT fk_dispute_escrow FOREIGN KEY(escrow_account_id) REFERENCES escrow_accounts(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_disputes_order_id ON disputes(order_id);
CREATE INDEX IF NOT EXISTS idx_disputes_status ON disputes(status);
`

func DefaultTestDSN() string {
	dsn := os.Getenv("TEST_DB_DSN")
	if dsn == "" {
		dsn = "postgres://postgres:postgres@localhost:5432/go_escrow_test?sslmode=disable" // #nosec G101
	}
	return dsn
}

func NewTestDB(dsn string) (*db.Database, error) {
	sqldb, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("testutil: sql.Open: %w", err)
	}

	sqldb.SetMaxOpenConns(5)
	sqldb.SetMaxIdleConns(2)
	sqldb.SetConnMaxLifetime(1 * time.Minute)

	if err := sqldb.Ping(); err != nil {
		_ = sqldb.Close() // #nosec G104
		return nil, fmt.Errorf("testutil: ping: %w", err)
	}

	if _, err := sqldb.Exec(migrationSQL); err != nil {
		_ = sqldb.Close() // #nosec G104
		return nil, fmt.Errorf("testutil: migrations: %w", err)
	}

	return &db.Database{DB: sqldb}, nil
}

func CleanupTestDB(database *db.Database) error {
	_, err := database.DB.Exec(`
		TRUNCATE TABLE disputes, escrow_transactions, escrow_accounts RESTART IDENTITY CASCADE
	`)
	return err
}
