package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/marketplace/go-escrow/internal/clients"
	"github.com/marketplace/go-escrow/internal/domain"
	"github.com/marketplace/go-escrow/internal/repository"
)

// mockRepo implements repository.EscrowRepository for testing
type mockRepo struct {
	accounts      map[uuid.UUID]*domain.EscrowAccount
	transactions  []domain.Transaction
	disputes      []domain.Dispute
	errOnGetByID  error
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
	if m.errOnGetByID != nil {
		return nil, m.errOnGetByID
	}
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

func (m *mockRepo) UpdateDisputeStatus(ctx context.Context, tx *sql.Tx, escrowAccountID uuid.UUID, status domain.DisputeStatus) error {
	for i, d := range m.disputes {
		if d.EscrowAccountID == escrowAccountID {
			m.disputes[i].Status = status
			return nil
		}
	}
	return nil
}

func (m *mockRepo) GetTransactionsByEscrowID(ctx context.Context, id uuid.UUID) ([]domain.Transaction, error) {
	return m.transactions, nil
}

func newTestService(repo repository.EscrowRepository) *EscrowService {
	logger, _ := zap.NewDevelopment()
	svc := NewEscrowService(repo, nil, logger, nil)
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

func TestEscrowService_Cancel(t *testing.T) {
	repo := newMockRepo()
	svc := newTestService(repo)

	orderID := uuid.New()
	account, _ := svc.Create(context.Background(), CreateEscrowRequest{
		OrderID: orderID,
		Amount:  decimal.NewFromFloat(100.00),
	})

	account, _ = svc.Fund(context.Background(), account.ID, decimal.NewFromFloat(100.00))

	account, err := svc.Cancel(context.Background(), account.ID)
	assert.NoError(t, err)
	assert.Equal(t, domain.StatusCancelled, account.Status)
	assert.True(t, account.Balance.Equal(decimal.Zero))

	found := false
	for _, txn := range repo.transactions {
		if txn.TransactionType == domain.TxnCancel {
			found = true
			break
		}
	}
	assert.True(t, found, "cancel transaction should exist")
}

func TestEscrowService_Cancel_InvalidTransition(t *testing.T) {
	repo := newMockRepo()
	svc := newTestService(repo)

	account, _ := svc.Create(context.Background(), CreateEscrowRequest{
		OrderID: uuid.New(),
		Amount:  decimal.NewFromFloat(100.00),
	})

	_, err := svc.Cancel(context.Background(), account.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid transition")
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

func newTestServiceWithBlockchain(repo repository.EscrowRepository, bcCli *clients.BlockchainClient) *EscrowService {
	logger, _ := zap.NewDevelopment()
	svc := NewEscrowService(repo, nil, logger, bcCli)
	svc.withTx = func(_ *sql.DB, _ *sql.TxOptions, fn func(tx *sql.Tx) error) error {
		return fn(nil)
	}
	return svc
}

func newBlockchainTestServer(events chan<- string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var event clients.BlockchainEvent
		if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		select {
		case events <- event.Action:
		default:
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"block_index":1,"tx_hash":"test"}`))
	}))
}

func waitForEvent(t *testing.T, events <-chan string, expected string) {
	t.Helper()
	select {
	case action := <-events:
		assert.Equal(t, expected, action)
	case <-time.After(3 * time.Second):
		t.Fatalf("timeout waiting for blockchain event %s", expected)
	}
}

func TestEscrowService_Create_EmitsBlockchainEvent(t *testing.T) {
	events := make(chan string, 1)
	server := newBlockchainTestServer(events)
	defer server.Close()

	logger, _ := zap.NewDevelopment()
	bcCli := clients.NewBlockchainClient(server.URL, logger)
	repo := newMockRepo()
	svc := newTestServiceWithBlockchain(repo, bcCli)

	_, err := svc.Create(context.Background(), CreateEscrowRequest{
		OrderID: uuid.New(),
		Amount:  decimal.NewFromFloat(100.00),
	})
	require.NoError(t, err)

	waitForEvent(t, events, "CREATED")
}

func TestEscrowService_Fund_EmitsBlockchainEvent(t *testing.T) {
	events := make(chan string, 1)
	server := newBlockchainTestServer(events)
	defer server.Close()

	logger, _ := zap.NewDevelopment()
	bcCli := clients.NewBlockchainClient(server.URL, logger)
	repo := newMockRepo()
	svc := newTestServiceWithBlockchain(repo, bcCli)

	account, err := svc.Create(context.Background(), CreateEscrowRequest{
		OrderID: uuid.New(),
		Amount:  decimal.NewFromFloat(100.00),
	})
	require.NoError(t, err)
	<-events // consume CREATED event

	_, err = svc.Fund(context.Background(), account.ID, decimal.NewFromFloat(100.00))
	require.NoError(t, err)

	waitForEvent(t, events, "FUNDED")
}

func TestEscrowService_Cancel_EmitsBlockchainEvent(t *testing.T) {
	events := make(chan string, 2)
	server := newBlockchainTestServer(events)
	defer server.Close()

	logger, _ := zap.NewDevelopment()
	bcCli := clients.NewBlockchainClient(server.URL, logger)
	repo := newMockRepo()
	svc := newTestServiceWithBlockchain(repo, bcCli)

	account, err := svc.Create(context.Background(), CreateEscrowRequest{
		OrderID: uuid.New(),
		Amount:  decimal.NewFromFloat(100.00),
	})
	require.NoError(t, err)
	<-events // consume CREATED

	account, err = svc.Fund(context.Background(), account.ID, decimal.NewFromFloat(100.00))
	require.NoError(t, err)
	<-events // consume FUNDED

	_, err = svc.Cancel(context.Background(), account.ID)
	require.NoError(t, err)

	waitForEvent(t, events, "CANCELLED")
}

func TestEscrowService_Release_EmitsBlockchainEvent(t *testing.T) {
	events := make(chan string, 4)
	server := newBlockchainTestServer(events)
	defer server.Close()

	logger, _ := zap.NewDevelopment()
	bcCli := clients.NewBlockchainClient(server.URL, logger)
	repo := newMockRepo()
	svc := newTestServiceWithBlockchain(repo, bcCli)

	account, err := svc.Create(context.Background(), CreateEscrowRequest{
		OrderID: uuid.New(),
		Amount:  decimal.NewFromFloat(100.00),
	})
	require.NoError(t, err)
	<-events // CREATED

	account, err = svc.Fund(context.Background(), account.ID, decimal.NewFromFloat(100.00))
	require.NoError(t, err)
	<-events // FUNDED

	account, err = svc.AdvanceStatus(context.Background(), account.ID, domain.StatusInProgress)
	require.NoError(t, err)
	<-events // IN_PROGRESS

	account, err = svc.AdvanceStatus(context.Background(), account.ID, domain.StatusCompleted)
	require.NoError(t, err)
	<-events // COMPLETED

	_, err = svc.Release(context.Background(), account.ID)
	require.NoError(t, err)

	waitForEvent(t, events, "RELEASED")
}

func TestEscrowService_Dispute_EmitsBlockchainEvent(t *testing.T) {
	events := make(chan string, 4)
	server := newBlockchainTestServer(events)
	defer server.Close()

	logger, _ := zap.NewDevelopment()
	bcCli := clients.NewBlockchainClient(server.URL, logger)
	repo := newMockRepo()
	svc := newTestServiceWithBlockchain(repo, bcCli)

	account, err := svc.Create(context.Background(), CreateEscrowRequest{
		OrderID: uuid.New(),
		Amount:  decimal.NewFromFloat(100.00),
	})
	require.NoError(t, err)
	<-events // CREATED

	account, err = svc.Fund(context.Background(), account.ID, decimal.NewFromFloat(100.00))
	require.NoError(t, err)
	<-events // FUNDED

	account, err = svc.AdvanceStatus(context.Background(), account.ID, domain.StatusInProgress)
	require.NoError(t, err)
	<-events // IN_PROGRESS

	account, err = svc.AdvanceStatus(context.Background(), account.ID, domain.StatusCompleted)
	require.NoError(t, err)
	<-events // COMPLETED

	_, err = svc.Dispute(context.Background(), account.ID, "defective product")
	require.NoError(t, err)

	waitForEvent(t, events, "DISPUTED")
}

func newTestServiceWithDBError(repo repository.EscrowRepository) *EscrowService {
	logger, _ := zap.NewDevelopment()
	svc := NewEscrowService(repo, nil, logger, nil)
	svc.withTx = func(_ *sql.DB, _ *sql.TxOptions, fn func(tx *sql.Tx) error) error {
		return fmt.Errorf("db connection failed")
	}
	return svc
}

// --- Error path tests ---

func TestEscrowService_Create_DBError(t *testing.T) {
	repo := newMockRepo()
	svc := newTestServiceWithDBError(repo)

	_, err := svc.Create(context.Background(), CreateEscrowRequest{
		OrderID: uuid.New(),
		Amount:  decimal.NewFromFloat(100.00),
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "create escrow")
}

func TestEscrowService_Fund_NotFound(t *testing.T) {
	repo := newMockRepo()
	svc := newTestService(repo)

	_, err := svc.Fund(context.Background(), uuid.New(), decimal.NewFromFloat(100.00))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestEscrowService_Fund_DBError(t *testing.T) {
	repo := newMockRepo()
	svc := newTestServiceWithDBError(repo)

	account := &domain.EscrowAccount{
		ID:      uuid.New(),
		OrderID: uuid.New(),
		Status:  domain.StatusCreated,
	}
	repo.accounts[account.ID] = account

	_, err := svc.Fund(context.Background(), account.ID, decimal.NewFromFloat(100.00))
	assert.Error(t, err)
}

func TestEscrowService_AdvanceStatus_NotFound(t *testing.T) {
	repo := newMockRepo()
	svc := newTestService(repo)

	_, err := svc.AdvanceStatus(context.Background(), uuid.New(), domain.StatusInProgress)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestEscrowService_AdvanceStatus_InvalidTransition(t *testing.T) {
	repo := newMockRepo()
	svc := newTestService(repo)

	account, _ := svc.Create(context.Background(), CreateEscrowRequest{
		OrderID: uuid.New(),
		Amount:  decimal.NewFromFloat(100.00),
	})

	_, err := svc.AdvanceStatus(context.Background(), account.ID, domain.StatusCompleted)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid transition")
}

func TestEscrowService_AdvanceStatus_DBError(t *testing.T) {
	repo := newMockRepo()
	svc := newTestServiceWithDBError(repo)

	account := &domain.EscrowAccount{
		ID:      uuid.New(),
		OrderID: uuid.New(),
		Status:  domain.StatusFunded,
	}
	repo.accounts[account.ID] = account

	_, err := svc.AdvanceStatus(context.Background(), account.ID, domain.StatusInProgress)
	assert.Error(t, err)
}

func TestEscrowService_Release_NotFound(t *testing.T) {
	repo := newMockRepo()
	svc := newTestService(repo)

	_, err := svc.Release(context.Background(), uuid.New())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestEscrowService_Release_InvalidTransition(t *testing.T) {
	repo := newMockRepo()
	svc := newTestService(repo)

	account, _ := svc.Create(context.Background(), CreateEscrowRequest{
		OrderID: uuid.New(),
		Amount:  decimal.NewFromFloat(100.00),
	})

	_, err := svc.Release(context.Background(), account.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid transition")
}

func TestEscrowService_Release_DBError(t *testing.T) {
	repo := newMockRepo()
	svc := newTestServiceWithDBError(repo)

	account := &domain.EscrowAccount{
		ID:      uuid.New(),
		OrderID: uuid.New(),
		Status:  domain.StatusCompleted,
		Balance: decimal.NewFromFloat(100.00),
	}
	repo.accounts[account.ID] = account

	_, err := svc.Release(context.Background(), account.ID)
	assert.Error(t, err)
}

func TestEscrowService_Cancel_NotFound(t *testing.T) {
	repo := newMockRepo()
	svc := newTestService(repo)

	_, err := svc.Cancel(context.Background(), uuid.New())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestEscrowService_Cancel_DBError(t *testing.T) {
	repo := newMockRepo()
	svc := newTestServiceWithDBError(repo)

	account := &domain.EscrowAccount{
		ID:      uuid.New(),
		OrderID: uuid.New(),
		Status:  domain.StatusFunded,
		Balance: decimal.NewFromFloat(100.00),
	}
	repo.accounts[account.ID] = account

	_, err := svc.Cancel(context.Background(), account.ID)
	assert.Error(t, err)
}

func TestEscrowService_Dispute_NotFound(t *testing.T) {
	repo := newMockRepo()
	svc := newTestService(repo)

	_, err := svc.Dispute(context.Background(), uuid.New(), "reason")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestEscrowService_Dispute_InvalidTransition(t *testing.T) {
	repo := newMockRepo()
	svc := newTestService(repo)

	account, _ := svc.Create(context.Background(), CreateEscrowRequest{
		OrderID: uuid.New(),
		Amount:  decimal.NewFromFloat(100.00),
	})

	_, err := svc.Dispute(context.Background(), account.ID, "bad product")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid transition")
}

func TestEscrowService_Dispute_DBError(t *testing.T) {
	repo := newMockRepo()
	svc := newTestServiceWithDBError(repo)

	account := &domain.EscrowAccount{
		ID:      uuid.New(),
		OrderID: uuid.New(),
		Status:  domain.StatusCompleted,
	}
	repo.accounts[account.ID] = account

	_, err := svc.Dispute(context.Background(), account.ID, "defective")
	assert.Error(t, err)
}

func TestEscrowService_GetByID_NotFound(t *testing.T) {
	repo := newMockRepo()
	svc := newTestService(repo)

	account, err := svc.GetByID(context.Background(), uuid.New())
	assert.NoError(t, err)
	assert.Nil(t, account)
}

func TestEscrowService_GetByID_Success(t *testing.T) {
	repo := newMockRepo()
	svc := newTestService(repo)

	created, _ := svc.Create(context.Background(), CreateEscrowRequest{
		OrderID: uuid.New(),
		Amount:  decimal.NewFromFloat(100.00),
	})

	account, err := svc.GetByID(context.Background(), created.ID)
	assert.NoError(t, err)
	assert.NotNil(t, account)
	assert.Equal(t, created.ID, account.ID)
}

func TestEscrowService_GetByID_DBError(t *testing.T) {
	repo := newMockRepo()
	repo.errOnGetByID = fmt.Errorf("query timeout")
	svc := newTestService(repo)

	_, err := svc.GetByID(context.Background(), uuid.New())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "get escrow")
}
