// LR #9: State-machine with explicit transitions (no invalid moves)
// LR #5: All transitions are O(1) map lookups — highload-ready

package domain

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type EscrowStatus string

const (
	StatusCreated    EscrowStatus = "CREATED"
	StatusFunded     EscrowStatus = "FUNDED"
	StatusInProgress EscrowStatus = "IN_PROGRESS"
	StatusCompleted  EscrowStatus = "COMPLETED"
	StatusReleased   EscrowStatus = "RELEASED"
	StatusCancelled  EscrowStatus = "CANCELLED"
	StatusDisputed   EscrowStatus = "DISPUTED"
	StatusResolved   EscrowStatus = "RESOLVED"
)

func (s EscrowStatus) String() string {
	return string(s)
}

var validTransitions = map[EscrowStatus]map[EscrowStatus]bool{
	StatusCreated:    {StatusFunded: true},
	StatusFunded:     {StatusInProgress: true, StatusCancelled: true},
	StatusInProgress: {StatusCompleted: true, StatusCancelled: true},
	StatusCompleted:  {StatusReleased: true, StatusDisputed: true},
	StatusDisputed:   {StatusResolved: true},
}

func IsValidTransition(from, to EscrowStatus) bool {
	if next, ok := validTransitions[from]; ok {
		return next[to]
	}
	return false
}

// EscrowAccount represents an escrow account with state-machine status.
type EscrowAccount struct {
	ID        uuid.UUID       `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	OrderID   uuid.UUID       `json:"order_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Balance   decimal.Decimal `json:"balance" example:"100.0000"`
	Status    EscrowStatus    `json:"status" example:"CREATED"`
	CreatedAt time.Time       `json:"created_at" example:"2026-06-01T12:00:00Z"`
	UpdatedAt time.Time       `json:"updated_at" example:"2026-06-01T12:00:00Z"`
}

type TransactionType string

const (
	TxnFund    TransactionType = "FUND"
	TxnRelease TransactionType = "RELEASE"
	TxnCancel  TransactionType = "CANCEL"
	TxnRefund  TransactionType = "REFUND"
)

type Transaction struct {
	ID              uuid.UUID       `json:"id"`
	EscrowAccountID uuid.UUID       `json:"escrow_account_id"`
	OrderID         uuid.UUID       `json:"order_id"`
	Amount          decimal.Decimal `json:"amount"`
	TransactionType TransactionType `json:"transaction_type"`
	Status          string          `json:"status"`
	CreatedAt       time.Time       `json:"created_at"`
}

type DisputeStatus string

const (
	DisputeOpen     DisputeStatus = "OPEN"
	DisputeResolved DisputeStatus = "RESOLVED"
)

type Dispute struct {
	ID              uuid.UUID     `json:"id"`
	OrderID         uuid.UUID     `json:"order_id"`
	EscrowAccountID uuid.UUID     `json:"escrow_account_id"`
	Reason          string        `json:"reason"`
	Status          DisputeStatus `json:"status"`
	CreatedAt       time.Time     `json:"created_at"`
	UpdatedAt       time.Time     `json:"updated_at"`
}

func NewEscrowAccount(orderID uuid.UUID, amount decimal.Decimal) *EscrowAccount {
	return &EscrowAccount{
		ID:        uuid.New(),
		OrderID:   orderID,
		Balance:   decimal.Zero,
		Status:    StatusCreated,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
}

func ValidateAmount(amount decimal.Decimal) error {
	if amount.IsNegative() {
		return fmt.Errorf("amount must be positive")
	}
	if amount.IsZero() {
		return fmt.Errorf("amount must be greater than zero")
	}
	return nil
}
