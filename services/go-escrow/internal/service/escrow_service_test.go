package service

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	"github.com/marketplace/go-escrow/internal/domain"
	"github.com/marketplace/go-escrow/internal/repository"
)

// mockRepo implements repository.EscrowRepository for testing
type mockRepo struct {
	accounts      map[uuid.UUID]*domain.EscrowAccount
	transactions  []domain.Transaction
	disputes      []domain.Dispute
}

func newMockRepo() *mockRepo {
	return &mockRepo{
		accounts:     make(map[uuid.UUID]*domain.EscrowAccount),
		transactions: make([]domain.Transaction, 0),
		disputes:     make([]domain.Dispute, 0),
	}
}

func (m *mockRepo) Create(ctx context.Context, tx *sql.Tx, account *domain.EscrowAccount) error {
	m.accounts[account.ID] = account
	return nil
}

func (m *mockRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.EscrowAccount, error) {
	account, ok := m.accounts[id]
	if !ok {
		return nil, nil
	}
	return account, nil
}

func (m *mockRepo) GetByOrderID(ctx context.Context, orderID uuid.UUID) (*domain.EscrowAccount, error) {
	for _, account := range m.accounts {
		if account.OrderID == orderID {
			return account, nil
		}
	}
	return nil, nil
}

func (m *mockRepo) UpdateStatus(ctx context.Context, tx *sql.Tx, id uuid.UUID, status domain.EscrowStatus) error {
	account, ok := m.accounts[id]
	if !ok {
		return nil
	}
	account.Status = status
	account.UpdatedAt = time.Now().UTC()
	return nil
}

func (m *mockRepo) UpdateBalanceAndStatus(ctx context.Context, tx *sql.Tx, id uuid.UUID, balance decimal.Decimal, status domain.EscrowStatus) error {
	account, ok := m.accounts[id]
	if !ok {
		return nil
	}
	account.Balance = balance
	account.Status = status
	account.UpdatedAt = time.Now().UTC()
	return nil
}

func (m *mockRepo) CreateTransaction(ctx context.Context, tx *sql.Tx, txn *domain.Transaction) error {
	m.transactions = append(m.transactions, *txn)
	return nil
}

func (m *mockRepo) CreateDispute(ctx context.Context, tx *sql.Tx, dispute *domain.Dispute) error {
	m.disputes = append(m.disputes, *dispute)
	return nil
}

func (m *mockRepo) GetTransactionsByEscrowID(ctx context.Context, id uuid.UUID) ([]domain.Transaction, error) {
	return m.transactions, nil
}

func newTestService(repo repository.EscrowRepository) *EscrowService {
	logger, _ := zap.NewDevelopment()
	svc := NewEscrowService(repo, nil, logger)
	// Override withTx to bypass real DB in unit tests
	svc.withTx = func(_ *sql.DB, _ *sql.TxOptions, fn func(tx *sql.Tx) error) error {
		return fn(nil)
	}
	return svc
}

func TestEscrowService_Create(t *testing.T) {
	repo := newMockRepo()
	svc := newTestService(repo)

	orderID := uuid.New()
	amount := decimal.NewFromFloat(100.00)

	req := CreateEscrowRequest{
		OrderID: orderID,
		Amount:  amount,
	}

	account, err := svc.Create(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, account)
	assert.Equal(t, domain.StatusCreated, account.Status)
	assert.Equal(t, orderID, account.OrderID)
}

func TestEscrowService_Create_InvalidAmount(t *testing.T) {
	repo := newMockRepo()
	svc := newTestService(repo)

	tests := []struct {
		name   string
		amount decimal.Decimal
	}{
		{"negative", decimal.NewFromFloat(-50.00)},
		{"zero", decimal.Zero},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.Create(context.Background(), CreateEscrowRequest{
				OrderID: uuid.New(),
				Amount:  tt.amount,
			})
			assert.Error(t, err)
		})
	}
}

func TestEscrowService_Fund(t *testing.T) {
	repo := newMockRepo()
	svc := newTestService(repo)

	orderID := uuid.New()
	account, _ := svc.Create(context.Background(), CreateEscrowRequest{
		OrderID: orderID,
		Amount:  decimal.NewFromFloat(100.00),
	})

	fundAmount := decimal.NewFromFloat(100.00)
	updated, err := svc.Fund(context.Background(), account.ID, fundAmount)
	assert.NoError(t, err)
	assert.NotNil(t, updated)
	assert.Equal(t, domain.StatusFunded, updated.Status)
	assert.True(t, updated.Balance.Equal(fundAmount))

	assert.Len(t, repo.transactions, 1)
	assert.Equal(t, domain.TxnFund, repo.transactions[0].TransactionType)
}

func TestEscrowService_Fund_InvalidTransition(t *testing.T) {
	repo := newMockRepo()
	svc := newTestService(repo)

	orderID := uuid.New()
	account, _ := svc.Create(context.Background(), CreateEscrowRequest{
		OrderID: orderID,
		Amount:  decimal.NewFromFloat(100.00),
	})

	_, err := svc.Fund(context.Background(), account.ID, decimal.NewFromFloat(100.00))
	assert.NoError(t, err)

	_, err = svc.Fund(context.Background(), account.ID, decimal.NewFromFloat(50.00))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid transition")
}

func TestEscrowService_Dispute(t *testing.T) {
	repo := newMockRepo()
	svc := newTestService(repo)

	orderID := uuid.New()
	account, _ := svc.Create(context.Background(), CreateEscrowRequest{
		OrderID: orderID,
		Amount:  decimal.NewFromFloat(100.00),
	})

	account, _ = svc.Fund(context.Background(), account.ID, decimal.NewFromFloat(100.00))

	account, err := svc.AdvanceStatus(context.Background(), account.ID, domain.StatusInProgress)
	assert.NoError(t, err)
	account, err = svc.AdvanceStatus(context.Background(), account.ID, domain.StatusCompleted)
	assert.NoError(t, err)

	account, err = svc.Dispute(context.Background(), account.ID, "Service not delivered")
	assert.NoError(t, err)
	assert.Equal(t, domain.StatusDisputed, account.Status)

	assert.Len(t, repo.disputes, 1)
	assert.Equal(t, "Service not delivered", repo.disputes[0].Reason)
}

func TestEscrowService_Dispute_NoReason(t *testing.T) {
	repo := newMockRepo()
	svc := newTestService(repo)

	account, _ := svc.Create(context.Background(), CreateEscrowRequest{
		OrderID: uuid.New(),
		Amount:  decimal.NewFromFloat(100.00),
	})

	_, err := svc.Dispute(context.Background(), account.ID, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "reason is required")
}

func TestEscrowService_Release(t *testing.T) {
	repo := newMockRepo()
	svc := newTestService(repo)

	orderID := uuid.New()
	account, _ := svc.Create(context.Background(), CreateEscrowRequest{
		OrderID: orderID,
		Amount:  decimal.NewFromFloat(100.00),
	})

	account, _ = svc.Fund(context.Background(), account.ID, decimal.NewFromFloat(100.00))

	account, _ = svc.AdvanceStatus(context.Background(), account.ID, domain.StatusInProgress)
	account, _ = svc.AdvanceStatus(context.Background(), account.ID, domain.StatusCompleted)

	account, err := svc.Release(context.Background(), account.ID)
	assert.NoError(t, err)
	assert.Equal(t, domain.StatusReleased, account.Status)

	found := false
	for _, txn := range repo.transactions {
		if txn.TransactionType == domain.TxnRelease {
			found = true
			break
		}
	}
	assert.True(t, found, "release transaction should exist")
}

func TestEscrowService_GetByID_NotFound(t *testing.T) {
	repo := newMockRepo()
	svc := newTestService(repo)

	account, err := svc.GetByID(context.Background(), uuid.New())
	assert.NoError(t, err)
	assert.Nil(t, account)
}
