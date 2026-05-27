// LR #13: Testing/Automation — table-driven тесты state-machine для всех 7 статусов
// LR #9: State-machine — верификация всех валидных и невалидных переходов

package domain

import (
	"testing"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

func TestIsValidTransition(t *testing.T) {
	tests := []struct {
		name string
		from EscrowStatus
		to   EscrowStatus
		want bool
	}{
		// === Valid forward transitions ===
		{"CREATED → FUNDED", StatusCreated, StatusFunded, true},
		{"FUNDED → IN_PROGRESS", StatusFunded, StatusInProgress, true},
		{"IN_PROGRESS → COMPLETED", StatusInProgress, StatusCompleted, true},
		{"COMPLETED → RELEASED", StatusCompleted, StatusReleased, true},
		{"COMPLETED → DISPUTED", StatusCompleted, StatusDisputed, true},
		{"DISPUTED → RESOLVED", StatusDisputed, StatusResolved, true},

		// === Invalid: reverse transitions ===
		{"FUNDED → CREATED (reverse)", StatusFunded, StatusCreated, false},
		{"IN_PROGRESS → FUNDED (reverse)", StatusInProgress, StatusFunded, false},
		{"COMPLETED → IN_PROGRESS (reverse)", StatusCompleted, StatusInProgress, false},
		{"RELEASED → COMPLETED (reverse)", StatusReleased, StatusCompleted, false},
		{"DISPUTED → COMPLETED (reverse)", StatusDisputed, StatusCompleted, false},
		{"RESOLVED → DISPUTED (reverse)", StatusResolved, StatusDisputed, false},

		// === Invalid: skip transitions ===
		{"CREATED → IN_PROGRESS (skip)", StatusCreated, StatusInProgress, false},
		{"CREATED → COMPLETED (skip)", StatusCreated, StatusCompleted, false},
		{"CREATED → RELEASED (skip)", StatusCreated, StatusReleased, false},
		{"CREATED → DISPUTED (skip)", StatusCreated, StatusDisputed, false},
		{"CREATED → RESOLVED (skip)", StatusCreated, StatusResolved, false},
		{"FUNDED → COMPLETED (skip)", StatusFunded, StatusCompleted, false},
		{"FUNDED → RELEASED (skip)", StatusFunded, StatusReleased, false},
		{"FUNDED → DISPUTED (skip)", StatusFunded, StatusDisputed, false},
		{"IN_PROGRESS → RELEASED (skip)", StatusInProgress, StatusReleased, false},
		{"IN_PROGRESS → DISPUTED (skip)", StatusInProgress, StatusDisputed, false},

		// === Invalid: terminal states have no outgoing transitions ===
		{"RELEASED → FUNDED (terminal)", StatusReleased, StatusFunded, false},
		{"RELEASED → CREATED (terminal)", StatusReleased, StatusCreated, false},
		{"RELEASED → IN_PROGRESS (terminal)", StatusReleased, StatusInProgress, false},
		{"RELEASED → DISPUTED (terminal)", StatusReleased, StatusDisputed, false},
		{"RELEASED → RESOLVED (terminal)", StatusReleased, StatusResolved, false},
		{"RESOLVED → CREATED (terminal)", StatusResolved, StatusCreated, false},
		{"RESOLVED → FUNDED (terminal)", StatusResolved, StatusFunded, false},
		{"RESOLVED → IN_PROGRESS (terminal)", StatusResolved, StatusInProgress, false},
		{"RESOLVED → COMPLETED (terminal)", StatusResolved, StatusCompleted, false},
		{"RESOLVED → RELEASED (terminal)", StatusResolved, StatusReleased, false},

		// === Invalid: self transitions ===
		{"CREATED → CREATED (self)", StatusCreated, StatusCreated, false},
		{"FUNDED → FUNDED (self)", StatusFunded, StatusFunded, false},
		{"IN_PROGRESS → IN_PROGRESS (self)", StatusInProgress, StatusInProgress, false},
		{"COMPLETED → COMPLETED (self)", StatusCompleted, StatusCompleted, false},
		{"RELEASED → RELEASED (self)", StatusReleased, StatusReleased, false},
		{"DISPUTED → DISPUTED (self)", StatusDisputed, StatusDisputed, false},
		{"RESOLVED → RESOLVED (self)", StatusResolved, StatusResolved, false},

		// === Invalid: unknown status ===
		{"UNKNOWN → CREATED", EscrowStatus("UNKNOWN"), StatusCreated, false},
		{"CREATED → UNKNOWN", StatusCreated, EscrowStatus("UNKNOWN"), false},
		{"UNKNOWN → UNKNOWN", EscrowStatus("UNKNOWN"), EscrowStatus("UNKNOWN"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
		{"small positive amount", decimal.NewFromFloat(0.01), false},
		{"large amount", decimal.NewFromFloat(999999.9999), false},
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

func TestEscrowStatusString(t *testing.T) {
	tests := []struct {
		status EscrowStatus
		want   string
	}{
		{StatusCreated, "CREATED"},
		{StatusFunded, "FUNDED"},
		{StatusInProgress, "IN_PROGRESS"},
		{StatusCompleted, "COMPLETED"},
		{StatusReleased, "RELEASED"},
		{StatusDisputed, "DISPUTED"},
		{StatusResolved, "RESOLVED"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.status.String())
		})
	}
}

func TestFullEscrowFlow(t *testing.T) {
	// Test the complete happy path: CREATED → FUNDED → IN_PROGRESS → COMPLETED → RELEASED
	transitions := []struct {
		from EscrowStatus
		to   EscrowStatus
	}{
		{StatusCreated, StatusFunded},
		{StatusFunded, StatusInProgress},
		{StatusInProgress, StatusCompleted},
		{StatusCompleted, StatusReleased},
	}

	current := StatusCreated
	for _, tr := range transitions {
		assert.True(t, IsValidTransition(current, tr.to), "valid transition %s → %s", current, tr.to)
		current = tr.to
	}

	// After RELEASED, no further transitions allowed
	assert.False(t, IsValidTransition(current, StatusFunded))
	assert.False(t, IsValidTransition(current, StatusDisputed))
	assert.False(t, IsValidTransition(current, StatusCreated))
}

func TestDisputeFlow(t *testing.T) {
	// Test dispute path: CREATED → FUNDED → IN_PROGRESS → COMPLETED → DISPUTED → RESOLVED
	transitions := []struct {
		from EscrowStatus
		to   EscrowStatus
	}{
		{StatusCreated, StatusFunded},
		{StatusFunded, StatusInProgress},
		{StatusInProgress, StatusCompleted},
		{StatusCompleted, StatusDisputed},
		{StatusDisputed, StatusResolved},
	}

	current := StatusCreated
	for _, tr := range transitions {
		assert.True(t, IsValidTransition(current, tr.to), "valid transition %s → %s", current, tr.to)
		current = tr.to
	}

	// After RESOLVED, no further transitions allowed
	assert.False(t, IsValidTransition(current, StatusFunded))
	assert.False(t, IsValidTransition(current, StatusCreated))
}

func TestOnlyValidForwardTransitions(t *testing.T) {
	allStatuses := []EscrowStatus{
		StatusCreated,
		StatusFunded,
		StatusInProgress,
		StatusCompleted,
		StatusReleased,
		StatusDisputed,
		StatusResolved,
	}

	validPairs := map[[2]EscrowStatus]bool{
		{StatusCreated, StatusFunded}:                      true,
		{StatusFunded, StatusInProgress}:                   true,
		{StatusInProgress, StatusCompleted}:                true,
		{StatusCompleted, StatusReleased}:                  true,
		{StatusCompleted, StatusDisputed}:                  true,
		{StatusDisputed, StatusResolved}:                   true,
	}

	for _, from := range allStatuses {
		for _, to := range allStatuses {
			pair := [2]EscrowStatus{from, to}
			expected := validPairs[pair]
			got := IsValidTransition(from, to)
			assert.Equal(t, expected, got, "IsValidTransition(%s, %s) = %v, want %v", from, to, got, expected)
		}
	}
}
