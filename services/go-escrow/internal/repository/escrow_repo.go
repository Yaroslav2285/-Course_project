// LR #6: DB — prepared statements, parameterized queries, NUMERIC(19,4)
// LR #5: Highload — context-based queries with timeout

package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"

	"github.com/marketplace/go-escrow/internal/domain"
)

type EscrowRepository interface {
	Create(ctx context.Context, tx *sql.Tx, account *domain.EscrowAccount) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.EscrowAccount, error)
	GetByOrderID(ctx context.Context, orderID uuid.UUID) (*domain.EscrowAccount, error)
	UpdateStatus(ctx context.Context, tx *sql.Tx, id uuid.UUID, status domain.EscrowStatus) error
	UpdateBalanceAndStatus(ctx context.Context, tx *sql.Tx, id uuid.UUID, balance decimal.Decimal, status domain.EscrowStatus) error
	CreateTransaction(ctx context.Context, tx *sql.Tx, txn *domain.Transaction) error
	CreateDispute(ctx context.Context, tx *sql.Tx, dispute *domain.Dispute) error
	GetTransactionsByEscrowID(ctx context.Context, id uuid.UUID) ([]domain.Transaction, error)
}

type escrowRepository struct {
	db  *sql.DB
	log *zap.Logger
}

func NewEscrowRepository(db *sql.DB, log *zap.Logger) EscrowRepository {
	return &escrowRepository{db: db, log: log}
}

func (r *escrowRepository) Create(ctx context.Context, tx *sql.Tx, a *domain.EscrowAccount) error {
	query := `INSERT INTO escrow_accounts (id, order_id, balance, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)`
	_, err := tx.ExecContext(ctx, query,
		a.ID, a.OrderID, a.Balance, a.Status, a.CreatedAt, a.UpdatedAt)
	if err != nil {
		return fmt.Errorf("insert escrow_account: %w", err)
	}
	return nil
}

func (r *escrowRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.EscrowAccount, error) {
	query := `SELECT id, order_id, balance, status, created_at, updated_at
		FROM escrow_accounts WHERE id = $1`
	row := r.db.QueryRowContext(ctx, query, id)

	a := &domain.EscrowAccount{}
	err := row.Scan(&a.ID, &a.OrderID, &a.Balance, &a.Status, &a.CreatedAt, &a.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan escrow_account: %w", err)
	}
	return a, nil
}

func (r *escrowRepository) GetByOrderID(ctx context.Context, orderID uuid.UUID) (*domain.EscrowAccount, error) {
	query := `SELECT id, order_id, balance, status, created_at, updated_at
		FROM escrow_accounts WHERE order_id = $1`
	row := r.db.QueryRowContext(ctx, query, orderID)

	a := &domain.EscrowAccount{}
	err := row.Scan(&a.ID, &a.OrderID, &a.Balance, &a.Status, &a.CreatedAt, &a.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan escrow_account by order_id: %w", err)
	}
	return a, nil
}

func (r *escrowRepository) UpdateStatus(ctx context.Context, tx *sql.Tx, id uuid.UUID, status domain.EscrowStatus) error {
	query := `UPDATE escrow_accounts SET status = $1, updated_at = $2 WHERE id = $3`
	now := time.Now().UTC()
	res, err := tx.ExecContext(ctx, query, status, now, id)
	if err != nil {
		return fmt.Errorf("update escrow status: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("escrow_account not found: %s", id)
	}
	return nil
}

func (r *escrowRepository) UpdateBalanceAndStatus(ctx context.Context, tx *sql.Tx, id uuid.UUID, balance decimal.Decimal, status domain.EscrowStatus) error {
	query := `UPDATE escrow_accounts SET balance = $1, status = $2, updated_at = $3 WHERE id = $4`
	now := time.Now().UTC()
	res, err := tx.ExecContext(ctx, query, balance, status, now, id)
	if err != nil {
		return fmt.Errorf("update escrow balance/status: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("escrow_account not found: %s", id)
	}
	return nil
}

func (r *escrowRepository) CreateTransaction(ctx context.Context, tx *sql.Tx, t *domain.Transaction) error {
	query := `INSERT INTO transactions (id, escrow_account_id, order_id, amount, transaction_type, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`
	_, err := tx.ExecContext(ctx, query,
		t.ID, t.EscrowAccountID, t.OrderID, t.Amount, t.TransactionType, t.Status, t.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert transaction: %w", err)
	}
	return nil
}

func (r *escrowRepository) CreateDispute(ctx context.Context, tx *sql.Tx, d *domain.Dispute) error {
	query := `INSERT INTO disputes (id, order_id, escrow_account_id, reason, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`
	_, err := tx.ExecContext(ctx, query,
		d.ID, d.OrderID, d.EscrowAccountID, d.Reason, d.Status, d.CreatedAt, d.UpdatedAt)
	if err != nil {
		return fmt.Errorf("insert dispute: %w", err)
	}
	return nil
}

func (r *escrowRepository) GetTransactionsByEscrowID(ctx context.Context, id uuid.UUID) ([]domain.Transaction, error) {
	query := `SELECT id, escrow_account_id, order_id, amount, transaction_type, status, created_at
		FROM transactions WHERE escrow_account_id = $1 ORDER BY created_at`
	rows, err := r.db.QueryContext(ctx, query, id)
	if err != nil {
		return nil, fmt.Errorf("query transactions: %w", err)
	}
	defer rows.Close()

	var txns []domain.Transaction
	for rows.Next() {
		var t domain.Transaction
		if err := rows.Scan(&t.ID, &t.EscrowAccountID, &t.OrderID, &t.Amount, &t.TransactionType, &t.Status, &t.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan transaction: %w", err)
		}
		txns = append(txns, t)
	}
	return txns, rows.Err()
}
