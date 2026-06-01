package repository

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/marketplace/go-escrow/internal/domain"
)

func newRepoWithMock(t *testing.T) (*escrowRepository, sqlmock.Sqlmock, *sql.DB) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	repo := &escrowRepository{db: db, log: zap.NewNop()}
	return repo, mock, db
}

func beginTx(t *testing.T, db *sql.DB) *sql.Tx {
	t.Helper()
	tx, err := db.Begin()
	require.NoError(t, err)
	return tx
}

// --- Create ---

func TestCreate_Success(t *testing.T) {
	repo, mock, db := newRepoWithMock(t)
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO escrow_accounts").
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	tx := beginTx(t, db)
	account := &domain.EscrowAccount{
		ID:        uuid.New(),
		OrderID:   uuid.New(),
		Balance:   decimal.NewFromFloat(100.00),
		Status:    domain.StatusCreated,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	err := repo.Create(context.Background(), tx, account)
	assert.NoError(t, err)
	assert.NoError(t, tx.Commit())
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreate_DBError(t *testing.T) {
	repo, mock, db := newRepoWithMock(t)
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO escrow_accounts").
		WillReturnError(errors.New("unique constraint violation"))
	mock.ExpectRollback()

	tx := beginTx(t, db)
	account := &domain.EscrowAccount{
		ID:      uuid.New(),
		OrderID: uuid.New(),
	}

	err := repo.Create(context.Background(), tx, account)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "insert escrow_account")
	assert.NoError(t, tx.Rollback())
	assert.NoError(t, mock.ExpectationsWereMet())
}

// --- GetByID ---

func TestGetByID_Success(t *testing.T) {
	repo, mock, db := newRepoWithMock(t)
	defer db.Close()

	id := uuid.New()
	orderID := uuid.New()
	now := time.Now().UTC()

	mock.ExpectQuery("SELECT id, order_id, balance, status, created_at, updated_at FROM escrow_accounts WHERE id = \\$1").
		WithArgs(id).
		WillReturnRows(sqlmock.NewRows([]string{"id", "order_id", "balance", "status", "created_at", "updated_at"}).
			AddRow(id.String(), orderID.String(), "100.00", "CREATED", now, now))

	account, err := repo.GetByID(context.Background(), id)
	assert.NoError(t, err)
	assert.NotNil(t, account)
	assert.Equal(t, id, account.ID)
	assert.Equal(t, orderID, account.OrderID)
	assert.True(t, decimal.NewFromFloat(100.00).Equal(account.Balance))
	assert.Equal(t, domain.StatusCreated, account.Status)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetByID_NoRows(t *testing.T) {
	repo, mock, db := newRepoWithMock(t)
	defer db.Close()

	id := uuid.New()
	mock.ExpectQuery("SELECT id, order_id, balance, status, created_at, updated_at FROM escrow_accounts WHERE id = \\$1").
		WithArgs(id).
		WillReturnError(sql.ErrNoRows)

	account, err := repo.GetByID(context.Background(), id)
	assert.NoError(t, err)
	assert.Nil(t, account)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetByID_ScanError(t *testing.T) {
	repo, mock, db := newRepoWithMock(t)
	defer db.Close()

	id := uuid.New()
	mock.ExpectQuery("SELECT id, order_id, balance, status, created_at, updated_at FROM escrow_accounts WHERE id = \\$1").
		WithArgs(id).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("bad"))

	_, err := repo.GetByID(context.Background(), id)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "scan escrow_account")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// --- GetByOrderID ---

func TestGetByOrderID_Success(t *testing.T) {
	repo, mock, db := newRepoWithMock(t)
	defer db.Close()

	id := uuid.New()
	orderID := uuid.New()
	now := time.Now().UTC()

	mock.ExpectQuery("SELECT id, order_id, balance, status, created_at, updated_at FROM escrow_accounts WHERE order_id = \\$1").
		WithArgs(orderID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "order_id", "balance", "status", "created_at", "updated_at"}).
			AddRow(id.String(), orderID.String(), "200.00", "FUNDED", now, now))

	account, err := repo.GetByOrderID(context.Background(), orderID)
	assert.NoError(t, err)
	assert.NotNil(t, account)
	assert.Equal(t, id, account.ID)
	assert.Equal(t, orderID, account.OrderID)
	assert.True(t, decimal.NewFromFloat(200.00).Equal(account.Balance))
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetByOrderID_NoRows(t *testing.T) {
	repo, mock, db := newRepoWithMock(t)
	defer db.Close()

	orderID := uuid.New()
	mock.ExpectQuery("SELECT id, order_id, balance, status, created_at, updated_at FROM escrow_accounts WHERE order_id = \\$1").
		WithArgs(orderID).
		WillReturnError(sql.ErrNoRows)

	account, err := repo.GetByOrderID(context.Background(), orderID)
	assert.NoError(t, err)
	assert.Nil(t, account)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetByOrderID_ScanError(t *testing.T) {
	repo, mock, db := newRepoWithMock(t)
	defer db.Close()

	orderID := uuid.New()
	mock.ExpectQuery("SELECT id, order_id, balance, status, created_at, updated_at FROM escrow_accounts WHERE order_id = \\$1").
		WithArgs(orderID).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("bad"))

	_, err := repo.GetByOrderID(context.Background(), orderID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "scan escrow_account by order_id")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// --- UpdateStatus ---

func TestUpdateStatus_Success(t *testing.T) {
	repo, mock, db := newRepoWithMock(t)
	defer db.Close()

	id := uuid.New()

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE escrow_accounts SET status").
		WithArgs(domain.StatusFunded, sqlmock.AnyArg(), id).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	tx := beginTx(t, db)
	err := repo.UpdateStatus(context.Background(), tx, id, domain.StatusFunded)
	assert.NoError(t, err)
	assert.NoError(t, tx.Commit())
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateStatus_NotFound(t *testing.T) {
	repo, mock, db := newRepoWithMock(t)
	defer db.Close()

	id := uuid.New()

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE escrow_accounts SET status").
		WithArgs(domain.StatusFunded, sqlmock.AnyArg(), id).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()

	tx := beginTx(t, db)
	err := repo.UpdateStatus(context.Background(), tx, id, domain.StatusFunded)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
	assert.NoError(t, tx.Rollback())
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateStatus_DBError(t *testing.T) {
	repo, mock, db := newRepoWithMock(t)
	defer db.Close()

	id := uuid.New()

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE escrow_accounts SET status").
		WithArgs(domain.StatusFunded, sqlmock.AnyArg(), id).
		WillReturnError(errors.New("connection lost"))
	mock.ExpectRollback()

	tx := beginTx(t, db)
	err := repo.UpdateStatus(context.Background(), tx, id, domain.StatusFunded)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "update escrow status")
	assert.NoError(t, tx.Rollback())
	assert.NoError(t, mock.ExpectationsWereMet())
}

// --- UpdateBalanceAndStatus ---

func TestUpdateBalanceAndStatus_Success(t *testing.T) {
	repo, mock, db := newRepoWithMock(t)
	defer db.Close()

	id := uuid.New()
	balance := decimal.NewFromFloat(50.00)

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE escrow_accounts SET balance").
		WithArgs(balance, domain.StatusCancelled, sqlmock.AnyArg(), id).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	tx := beginTx(t, db)
	err := repo.UpdateBalanceAndStatus(context.Background(), tx, id, balance, domain.StatusCancelled)
	assert.NoError(t, err)
	assert.NoError(t, tx.Commit())
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateBalanceAndStatus_NotFound(t *testing.T) {
	repo, mock, db := newRepoWithMock(t)
	defer db.Close()

	id := uuid.New()

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE escrow_accounts SET balance").
		WithArgs(decimal.Zero, domain.StatusCancelled, sqlmock.AnyArg(), id).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()

	tx := beginTx(t, db)
	err := repo.UpdateBalanceAndStatus(context.Background(), tx, id, decimal.Zero, domain.StatusCancelled)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
	assert.NoError(t, tx.Rollback())
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateBalanceAndStatus_DBError(t *testing.T) {
	repo, mock, db := newRepoWithMock(t)
	defer db.Close()

	id := uuid.New()

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE escrow_accounts SET balance").
		WithArgs(decimal.Zero, domain.StatusCancelled, sqlmock.AnyArg(), id).
		WillReturnError(errors.New("timeout"))
	mock.ExpectRollback()

	tx := beginTx(t, db)
	err := repo.UpdateBalanceAndStatus(context.Background(), tx, id, decimal.Zero, domain.StatusCancelled)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "update escrow balance/status")
	assert.NoError(t, tx.Rollback())
	assert.NoError(t, mock.ExpectationsWereMet())
}

// --- CreateTransaction ---

func TestCreateTransaction_Success(t *testing.T) {
	repo, mock, db := newRepoWithMock(t)
	defer db.Close()

	txn := &domain.Transaction{
		ID:              uuid.New(),
		EscrowAccountID: uuid.New(),
		OrderID:         uuid.New(),
		Amount:          decimal.NewFromFloat(100.00),
		TransactionType: domain.TxnFund,
		Status:          "completed",
		CreatedAt:       time.Now().UTC(),
	}

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO escrow_transactions").
		WithArgs(txn.ID, txn.EscrowAccountID, txn.OrderID, txn.Amount, txn.TransactionType, txn.Status, txn.CreatedAt).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	tx := beginTx(t, db)
	err := repo.CreateTransaction(context.Background(), tx, txn)
	assert.NoError(t, err)
	assert.NoError(t, tx.Commit())
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateTransaction_DBError(t *testing.T) {
	repo, mock, db := newRepoWithMock(t)
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO escrow_transactions").
		WillReturnError(errors.New("table locked"))
	mock.ExpectRollback()

	tx := beginTx(t, db)
	err := repo.CreateTransaction(context.Background(), tx, &domain.Transaction{ID: uuid.New()})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "insert transaction")
	assert.NoError(t, tx.Rollback())
	assert.NoError(t, mock.ExpectationsWereMet())
}

// --- CreateDispute ---

func TestCreateDispute_Success(t *testing.T) {
	repo, mock, db := newRepoWithMock(t)
	defer db.Close()

	dispute := &domain.Dispute{
		ID:              uuid.New(),
		OrderID:         uuid.New(),
		EscrowAccountID: uuid.New(),
		Reason:          "defective item",
		Status:          domain.DisputeOpen,
		CreatedAt:       time.Now().UTC(),
		UpdatedAt:       time.Now().UTC(),
	}

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO disputes").
		WithArgs(dispute.ID, dispute.OrderID, dispute.EscrowAccountID, dispute.Reason, dispute.Status, dispute.CreatedAt, dispute.UpdatedAt).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	tx := beginTx(t, db)
	err := repo.CreateDispute(context.Background(), tx, dispute)
	assert.NoError(t, err)
	assert.NoError(t, tx.Commit())
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateDispute_DBError(t *testing.T) {
	repo, mock, db := newRepoWithMock(t)
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO disputes").
		WillReturnError(errors.New("fk constraint violation"))
	mock.ExpectRollback()

	tx := beginTx(t, db)
	err := repo.CreateDispute(context.Background(), tx, &domain.Dispute{ID: uuid.New()})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "insert dispute")
	assert.NoError(t, tx.Rollback())
	assert.NoError(t, mock.ExpectationsWereMet())
}

// --- GetTransactionsByEscrowID ---

func TestGetTransactionsByEscrowID_Success(t *testing.T) {
	repo, mock, db := newRepoWithMock(t)
	defer db.Close()

	escrowID := uuid.New()
	now := time.Now().UTC()

	mock.ExpectQuery("SELECT id, escrow_account_id, order_id, amount, transaction_type, status, created_at FROM escrow_transactions WHERE escrow_account_id = \\$1 ORDER BY created_at").
		WithArgs(escrowID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "escrow_account_id", "order_id", "amount", "transaction_type", "status", "created_at"}).
			AddRow(uuid.New().String(), escrowID.String(), uuid.New().String(), "100.00", "FUND", "completed", now).
			AddRow(uuid.New().String(), escrowID.String(), uuid.New().String(), "50.00", "RELEASE", "completed", now.Add(time.Minute)))

	txns, err := repo.GetTransactionsByEscrowID(context.Background(), escrowID)
	assert.NoError(t, err)
	assert.Len(t, txns, 2)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetTransactionsByEscrowID_Empty(t *testing.T) {
	repo, mock, db := newRepoWithMock(t)
	defer db.Close()

	escrowID := uuid.New()

	mock.ExpectQuery("SELECT id, escrow_account_id, order_id, amount, transaction_type, status, created_at FROM escrow_transactions WHERE escrow_account_id = \\$1 ORDER BY created_at").
		WithArgs(escrowID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "escrow_account_id", "order_id", "amount", "transaction_type", "status", "created_at"}))

	txns, err := repo.GetTransactionsByEscrowID(context.Background(), escrowID)
	assert.NoError(t, err)
	assert.Empty(t, txns)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// --- UpdateDisputeStatus ---

func TestUpdateDisputeStatus_Success(t *testing.T) {
	repo, mock, db := newRepoWithMock(t)
	defer db.Close()

	escrowID := uuid.New()

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE disputes SET status = \\$1, updated_at = \\$2 WHERE escrow_account_id = \\$3").
		WithArgs(domain.DisputeResolved, sqlmock.AnyArg(), escrowID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	tx := beginTx(t, db)
	err := repo.UpdateDisputeStatus(context.Background(), tx, escrowID, domain.DisputeResolved)
	assert.NoError(t, err)
	assert.NoError(t, tx.Commit())
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateDisputeStatus_NotFound(t *testing.T) {
	repo, mock, db := newRepoWithMock(t)
	defer db.Close()

	escrowID := uuid.New()

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE disputes SET status = \\$1, updated_at = \\$2 WHERE escrow_account_id = \\$3").
		WithArgs(domain.DisputeResolved, sqlmock.AnyArg(), escrowID).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()

	tx := beginTx(t, db)
	err := repo.UpdateDisputeStatus(context.Background(), tx, escrowID, domain.DisputeResolved)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "dispute not found")
	assert.NoError(t, tx.Rollback())
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateDisputeStatus_DBError(t *testing.T) {
	repo, mock, db := newRepoWithMock(t)
	defer db.Close()

	escrowID := uuid.New()

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE disputes SET status = \\$1, updated_at = \\$2 WHERE escrow_account_id = \\$3").
		WithArgs(domain.DisputeResolved, sqlmock.AnyArg(), escrowID).
		WillReturnError(errors.New("deadlock detected"))
	mock.ExpectRollback()

	tx := beginTx(t, db)
	err := repo.UpdateDisputeStatus(context.Background(), tx, escrowID, domain.DisputeResolved)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "update dispute status")
	assert.NoError(t, tx.Rollback())
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetTransactionsByEscrowID_QueryError(t *testing.T) {
	repo, mock, db := newRepoWithMock(t)
	defer db.Close()

	escrowID := uuid.New()
	mock.ExpectQuery("SELECT id, escrow_account_id, order_id, amount, transaction_type, status, created_at FROM escrow_transactions WHERE escrow_account_id = \\$1 ORDER BY created_at").
		WithArgs(escrowID).
		WillReturnError(errors.New("permission denied"))

	_, err := repo.GetTransactionsByEscrowID(context.Background(), escrowID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "query transactions")
	assert.NoError(t, mock.ExpectationsWereMet())
}
