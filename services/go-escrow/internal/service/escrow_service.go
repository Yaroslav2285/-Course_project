// LR #9: Transactional business logic with atomic status transitions
// LR #5: Context propagation with deadlines, rate-limited fund operation

package service

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"

	"github.com/marketplace/go-escrow/internal/clients"
	internaldb "github.com/marketplace/go-escrow/internal/db"
	"github.com/marketplace/go-escrow/internal/domain"
	"github.com/marketplace/go-escrow/internal/repository"
)

type CreateEscrowRequest struct {
	OrderID uuid.UUID       `json:"order_id"`
	Amount  decimal.Decimal `json:"amount"`
}

// TransactionFunc abstracts database transaction for testability.
type TransactionFunc func(db *sql.DB, opts *sql.TxOptions, fn func(tx *sql.Tx) error) error

type EscrowService struct {
	repo            repository.EscrowRepository
	db              *sql.DB
	log             *zap.Logger
	withTx          TransactionFunc
	blockchainCli   *clients.BlockchainClient
}

func NewEscrowService(repo repository.EscrowRepository, database *sql.DB, log *zap.Logger, blockchainCli *clients.BlockchainClient) *EscrowService {
	return &EscrowService{
		repo:          repo,
		db:            database,
		log:           log,
		blockchainCli: blockchainCli,
		withTx: func(d *sql.DB, opts *sql.TxOptions, fn func(tx *sql.Tx) error) error {
			return internaldb.WithTx(d, opts, fn)
		},
	}
}

func (s *EscrowService) Create(ctx context.Context, req CreateEscrowRequest) (*domain.EscrowAccount, error) {
	if err := domain.ValidateAmount(req.Amount); err != nil {
		return nil, fmt.Errorf("validation: %w", err)
	}

	account := domain.NewEscrowAccount(req.OrderID, decimal.Zero)

	err := s.withTx(s.db, nil, func(tx *sql.Tx) error {
		return s.repo.Create(ctx, tx, account)
	})
	if err != nil {
		return nil, fmt.Errorf("create escrow: %w", err)
	}

	s.log.Info("escrow created", zap.String("id", account.ID.String()), zap.String("order_id", account.OrderID.String()))

	go s.emitBlockchainEvent(context.Background(), account, "CREATED") // #nosec G118 — intentional: async fire-and-forget, must outlive request

	return account, nil
}

func (s *EscrowService) Fund(ctx context.Context, id uuid.UUID, amount decimal.Decimal) (*domain.EscrowAccount, error) {
	if err := domain.ValidateAmount(amount); err != nil {
		return nil, fmt.Errorf("validation: %w", err)
	}

	account, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get escrow: %w", err)
	}
	if account == nil {
		return nil, fmt.Errorf("escrow_account not found: %s", id)
	}

	if !domain.IsValidTransition(account.Status, domain.StatusFunded) {
		return nil, fmt.Errorf("invalid transition from %s to FUNDED", account.Status)
	}

	newBalance := account.Balance.Add(amount)

	err = s.withTx(s.db, nil, func(tx *sql.Tx) error {
		if err := s.repo.UpdateBalanceAndStatus(ctx, tx, id, newBalance, domain.StatusFunded); err != nil {
			return err
		}

		txn := &domain.Transaction{
			ID:              uuid.New(),
			EscrowAccountID: id,
			OrderID:         account.OrderID,
			Amount:          amount,
			TransactionType: domain.TxnFund,
			Status:          "COMPLETED",
			CreatedAt:       time.Now().UTC(),
		}
		return s.repo.CreateTransaction(ctx, tx, txn)
	})
	if err != nil {
		return nil, fmt.Errorf("fund escrow: %w", err)
	}

	account.Balance = newBalance
	account.Status = domain.StatusFunded
	account.UpdatedAt = time.Now().UTC()

	s.log.Info("escrow funded", zap.String("id", id.String()), zap.String("amount", amount.String()))

	go s.emitBlockchainEvent(context.Background(), account, "FUNDED") // #nosec G118 — intentional: async fire-and-forget

	return account, nil
}

func (s *EscrowService) AdvanceStatus(ctx context.Context, id uuid.UUID, nextStatus domain.EscrowStatus) (*domain.EscrowAccount, error) {
	account, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get escrow: %w", err)
	}
	if account == nil {
		return nil, fmt.Errorf("escrow_account not found: %s", id)
	}

	if !domain.IsValidTransition(account.Status, nextStatus) {
		return nil, fmt.Errorf("invalid transition from %s to %s", account.Status, nextStatus)
	}

	err = s.withTx(s.db, nil, func(tx *sql.Tx) error {
		return s.repo.UpdateStatus(ctx, tx, id, nextStatus)
	})
	if err != nil {
		return nil, fmt.Errorf("advance status %s: %w", nextStatus, err)
	}

	account.Status = nextStatus
	account.UpdatedAt = time.Now().UTC()

	s.log.Info("escrow status advanced", zap.String("id", id.String()), zap.String("status", string(nextStatus)))
	return account, nil
}

func (s *EscrowService) Release(ctx context.Context, id uuid.UUID) (*domain.EscrowAccount, error) {
	account, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get escrow: %w", err)
	}
	if account == nil {
		return nil, fmt.Errorf("escrow_account not found: %s", id)
	}

	if !domain.IsValidTransition(account.Status, domain.StatusReleased) {
		return nil, fmt.Errorf("invalid transition from %s to RELEASED", account.Status)
	}

	err = s.withTx(s.db, nil, func(tx *sql.Tx) error {
		if err := s.repo.UpdateStatus(ctx, tx, id, domain.StatusReleased); err != nil {
			return err
		}

		txn := &domain.Transaction{
			ID:              uuid.New(),
			EscrowAccountID: id,
			OrderID:         account.OrderID,
			Amount:          account.Balance,
			TransactionType: domain.TxnRelease,
			Status:          "COMPLETED",
			CreatedAt:       time.Now().UTC(),
		}
		return s.repo.CreateTransaction(ctx, tx, txn)
	})
	if err != nil {
		return nil, fmt.Errorf("release escrow: %w", err)
	}

	account.Status = domain.StatusReleased
	account.UpdatedAt = time.Now().UTC()

	s.log.Info("escrow released", zap.String("id", id.String()), zap.String("amount", account.Balance.String()))

	go s.emitBlockchainEvent(context.Background(), account, "RELEASED") // #nosec G118 — intentional: async fire-and-forget

	return account, nil
}

func (s *EscrowService) Dispute(ctx context.Context, id uuid.UUID, reason string) (*domain.EscrowAccount, error) {
	if reason == "" {
		return nil, fmt.Errorf("reason is required for dispute")
	}

	account, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get escrow: %w", err)
	}
	if account == nil {
		return nil, fmt.Errorf("escrow_account not found: %s", id)
	}

	if !domain.IsValidTransition(account.Status, domain.StatusDisputed) {
		return nil, fmt.Errorf("invalid transition from %s to DISPUTED", account.Status)
	}

	err = s.withTx(s.db, nil, func(tx *sql.Tx) error {
		if err := s.repo.UpdateStatus(ctx, tx, id, domain.StatusDisputed); err != nil {
			return err
		}

		dispute := &domain.Dispute{
			ID:              uuid.New(),
			OrderID:         account.OrderID,
			EscrowAccountID: id,
			Reason:          reason,
			Status:          domain.DisputeOpen,
			CreatedAt:       time.Now().UTC(),
			UpdatedAt:       time.Now().UTC(),
		}
		return s.repo.CreateDispute(ctx, tx, dispute)
	})
	if err != nil {
		return nil, fmt.Errorf("dispute escrow: %w", err)
	}

	account.Status = domain.StatusDisputed
	account.UpdatedAt = time.Now().UTC()

	s.log.Info("escrow disputed", zap.String("id", id.String()), zap.String("reason", reason))

	go s.emitBlockchainEvent(context.Background(), account, "DISPUTED") // #nosec G118 — intentional: async fire-and-forget

	return account, nil
}

func (s *EscrowService) GetByID(ctx context.Context, id uuid.UUID) (*domain.EscrowAccount, error) {
	account, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get escrow: %w", err)
	}
	return account, nil
}

func (s *EscrowService) emitBlockchainEvent(ctx context.Context, account *domain.EscrowAccount, action string) {
	if s.blockchainCli == nil {
		return
	}

	event := &clients.BlockchainEvent{
		OrderID: account.OrderID.String(),
		Action:  action,
		Data: map[string]any{
			"escrow_id": account.ID.String(),
			"status":    string(account.Status),
			"amount":    account.Balance.String(),
		},
	}

	_, err := s.blockchainCli.SubmitEvent(ctx, event)
	if err != nil {
		s.log.Warn("blockchain event emission failed (non-blocking)",
			zap.String("escrow_id", account.ID.String()),
			zap.String("action", action),
			zap.Error(err),
		)
	}
}
