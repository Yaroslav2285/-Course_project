// LR #13: Testing/Automation — PostgreSQL-backed integration test for EscrowService
// LR #6: DB — requires a running PostgreSQL instance (TEST_DB_DSN env var)
// LR #10: Multi-lang/REST — end-to-end escrow cycle with real database

//go:build integration

package service

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/marketplace/go-escrow/internal/domain"
	"github.com/marketplace/go-escrow/internal/repository"
	"github.com/marketplace/go-escrow/internal/testutil"
)

func TestPostgresEscrowFullCycle(t *testing.T) {
	dsn := testutil.DefaultTestDSN()
	database, err := testutil.NewTestDB(dsn)
	require.NoError(t, err, "connect to test database")
	defer database.Close()
	defer testutil.CleanupTestDB(database)

	logger, _ := zap.NewDevelopment()
	repo := repository.NewEscrowRepository(database.DB, logger)
	svc := NewEscrowService(repo, nil, logger, database.DB)

	orderID := uuid.New()
	amount := decimal.NewFromFloat(500.0000)

	account, err := svc.Create(context.Background(), CreateEscrowRequest{
		OrderID: orderID,
		Amount:  amount,
	})
	require.NoError(t, err)
	require.Equal(t, domain.StatusCreated, account.Status)

	account, err = svc.Fund(context.Background(), account.ID, amount)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusFunded, account.Status)

	account, err = svc.AdvanceStatus(context.Background(), account.ID, domain.StatusInProgress)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusInProgress, account.Status)

	account, err = svc.AdvanceStatus(context.Background(), account.ID, domain.StatusCompleted)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusCompleted, account.Status)

	account, err = svc.Release(context.Background(), account.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusReleased, account.Status)

	fromDB, err := repo.GetByID(context.Background(), account.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusReleased, fromDB.Status)
}

func TestPostgresEscrowCancelFromFunded(t *testing.T) {
	dsn := testutil.DefaultTestDSN()
	database, err := testutil.NewTestDB(dsn)
	require.NoError(t, err, "connect to test database")
	defer database.Close()
	defer testutil.CleanupTestDB(database)

	logger, _ := zap.NewDevelopment()
	repo := repository.NewEscrowRepository(database.DB, logger)
	svc := NewEscrowService(repo, nil, logger, database.DB)

	orderID := uuid.New()
	amount := decimal.NewFromFloat(200.0000)

	account, err := svc.Create(context.Background(), CreateEscrowRequest{
		OrderID: orderID,
		Amount:  amount,
	})
	require.NoError(t, err)

	account, err = svc.Fund(context.Background(), account.ID, amount)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusFunded, account.Status)

	account, err = svc.Cancel(context.Background(), account.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusCancelled, account.Status)
	assert.True(t, account.Balance.Equal(decimal.Zero))
}

func TestPostgresEscrowDisputeFromCompleted(t *testing.T) {
	dsn := testutil.DefaultTestDSN()
	database, err := testutil.NewTestDB(dsn)
	require.NoError(t, err, "connect to test database")
	defer database.Close()
	defer testutil.CleanupTestDB(database)

	logger, _ := zap.NewDevelopment()
	repo := repository.NewEscrowRepository(database.DB, logger)
	svc := NewEscrowService(repo, nil, logger, database.DB)

	orderID := uuid.New()
	amount := decimal.NewFromFloat(300.0000)

	account, err := svc.Create(context.Background(), CreateEscrowRequest{
		OrderID: orderID,
		Amount:  amount,
	})
	require.NoError(t, err)

	account, _ = svc.Fund(context.Background(), account.ID, amount)
	account, _ = svc.AdvanceStatus(context.Background(), account.ID, domain.StatusInProgress)
	account, _ = svc.AdvanceStatus(context.Background(), account.ID, domain.StatusCompleted)

	account, err = svc.Dispute(context.Background(), account.ID, "Integration test dispute")
	require.NoError(t, err)
	assert.Equal(t, domain.StatusDisputed, account.Status)
}
