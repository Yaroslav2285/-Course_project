package domain

import (
	"testing"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

func TestIsValidTransition(t *testing.T) {
	tests := []struct {
		from EscrowStatus
		to   EscrowStatus
		want bool
	}{
		// Valid transitions
		{StatusCreated, StatusFunded, true},
		{StatusFunded, StatusInProgress, true},
		{StatusInProgress, StatusCompleted, true},
		{StatusCompleted, StatusReleased, true},
		{StatusCompleted, StatusDisputed, true},
		{StatusDisputed, StatusResolved, true},

		// Invalid: reverse transitions
		{StatusFunded, StatusCreated, false},
		{StatusInProgress, StatusFunded, false},
		{StatusCompleted, StatusInProgress, false},
		{StatusReleased, StatusCompleted, false},
		{StatusDisputed, StatusCompleted, false},
		{StatusResolved, StatusDisputed, false},

		// Invalid: skip transitions
		{StatusCreated, StatusReleased, false},
		{StatusCreated, StatusInProgress, false},
		{StatusFunded, StatusReleased, false},

		// Invalid: terminal states
		{StatusReleased, StatusCreated, false},
		{StatusReleased, StatusFunded, false},
		{StatusResolved, StatusCreated, false},
		{StatusResolved, StatusFunded, false},

		// Invalid: self transitions
		{StatusCreated, StatusCreated, false},
		{StatusFunded, StatusFunded, false},
		{StatusReleased, StatusReleased, false},
		{StatusResolved, StatusResolved, false},

		// Invalid: unknown status
		{EscrowStatus("UNKNOWN"), StatusCreated, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.from)+"->"+string(tt.to), func(t *testing.T) {
			got := IsValidTransition(tt.from, tt.to)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNewEscrowAccount(t *testing.T) {
	orderID := uuid.New()
	amount := decimal.NewFromFloat(100.00)

	account := NewEscrowAccount(orderID, amount)

	assert.Equal(t, StatusCreated, account.Status)
	assert.Equal(t, orderID, account.OrderID)
	assert.True(t, account.Balance.Equal(decimal.Zero))
	assert.False(t, account.ID == uuid.Nil)
	assert.False(t, account.CreatedAt.IsZero())
	assert.False(t, account.UpdatedAt.IsZero())
}

func TestValidateAmount(t *testing.T) {
	tests := []struct {
		name    string
		amount  decimal.Decimal
		wantErr bool
	}{
		{"positive amount", decimal.NewFromFloat(100.00), false},
		{"negative amount", decimal.NewFromFloat(-50.00), true},
		{"zero amount", decimal.Zero, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateAmount(tt.amount)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
