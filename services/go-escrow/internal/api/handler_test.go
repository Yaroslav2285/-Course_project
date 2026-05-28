package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"

	"github.com/marketplace/go-escrow/internal/domain"
	"github.com/marketplace/go-escrow/internal/service"
)

type mockEscrowSvc struct {
	accounts map[uuid.UUID]*domain.EscrowAccount
}

func newMockEscrowSvc() *mockEscrowSvc {
	return &mockEscrowSvc{
		accounts: make(map[uuid.UUID]*domain.EscrowAccount),
	}
}

func (m *mockEscrowSvc) Create(_ context.Context, req service.CreateEscrowRequest) (*domain.EscrowAccount, error) {
	account := &domain.EscrowAccount{
		ID:        uuid.New(),
		OrderID:   req.OrderID,
		Balance:   decimal.Zero,
		Status:    domain.StatusCreated,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	m.accounts[account.ID] = account
	return account, nil
}

func (m *mockEscrowSvc) AdvanceStatus(_ context.Context, id uuid.UUID, nextStatus domain.EscrowStatus) (*domain.EscrowAccount, error) {
	account, ok := m.accounts[id]
	if !ok {
		return nil, fmt.Errorf("escrow_account not found")
	}
	account.Status = nextStatus
	account.UpdatedAt = time.Now().UTC()
	return account, nil
}

func (m *mockEscrowSvc) Fund(_ context.Context, id uuid.UUID, amount decimal.Decimal) (*domain.EscrowAccount, error) {
	account, ok := m.accounts[id]
	if !ok {
		return nil, fmt.Errorf("escrow_account not found")
	}
	account.Balance = amount
	account.Status = domain.StatusFunded
	account.UpdatedAt = time.Now().UTC()
	return account, nil
}

func (m *mockEscrowSvc) Release(_ context.Context, id uuid.UUID) (*domain.EscrowAccount, error) {
	account, ok := m.accounts[id]
	if !ok {
		return nil, fmt.Errorf("escrow_account not found")
	}
	account.Status = domain.StatusReleased
	account.UpdatedAt = time.Now().UTC()
	return account, nil
}

func (m *mockEscrowSvc) Dispute(_ context.Context, id uuid.UUID, reason string) (*domain.EscrowAccount, error) {
	account, ok := m.accounts[id]
	if !ok {
		return nil, fmt.Errorf("escrow_account not found")
	}
	account.Status = domain.StatusDisputed
	account.UpdatedAt = time.Now().UTC()
	return account, nil
}

func (m *mockEscrowSvc) GetByID(_ context.Context, id uuid.UUID) (*domain.EscrowAccount, error) {
	account, ok := m.accounts[id]
	if !ok {
		return nil, nil
	}
	return account, nil
}

func setupTestRouter() (*gin.Engine, *mockEscrowSvc) {
	gin.SetMode(gin.TestMode)

	svc := newMockEscrowSvc()
	handler := NewEscrowHandler(svc)

	r := gin.New()
	r.Use(gin.Recovery())

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "service": "go-escrow"})
	})

	v1 := r.Group("/v1/escrow")
	{
		v1.POST("/", handler.Create)
		v1.GET("/:id", handler.GetByID)
		v1.POST("/:id/fund", handler.Fund)
		v1.POST("/:id/advance", handler.Advance)
		v1.POST("/:id/release", handler.Release)
		v1.POST("/:id/dispute", handler.Dispute)
	}

	return r, svc
}

func TestHealthCheck(t *testing.T) {
	r, _ := setupTestRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/health", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "ok", resp["status"])
	assert.Equal(t, "go-escrow", resp["service"])
}

func TestCreateEscrow(t *testing.T) {
	r, _ := setupTestRouter()

	body := map[string]interface{}{
		"order_id": uuid.New().String(),
		"amount":   "100.00",
	}
	b, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/escrow/", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].(map[string]interface{})
	assert.Equal(t, "CREATED", data["status"])
	assert.NotEmpty(t, data["id"])
}

func TestCreateEscrow_InvalidBody(t *testing.T) {
	r, _ := setupTestRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/escrow/", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateEscrow_InvalidAmount(t *testing.T) {
	r, _ := setupTestRouter()

	body := map[string]interface{}{
		"order_id": uuid.New().String(),
		"amount":   "not-a-number",
	}
	b, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/escrow/", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetEscrowByID(t *testing.T) {
	r, svc := setupTestRouter()

	orderID := uuid.New()
	account := &domain.EscrowAccount{
		ID:        uuid.New(),
		OrderID:   orderID,
		Balance:   decimal.Zero,
		Status:    domain.StatusCreated,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	svc.accounts[account.ID] = account

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/escrow/"+account.ID.String(), nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].(map[string]interface{})
	assert.Equal(t, "CREATED", data["status"])
}

func TestGetEscrowByID_NotFound(t *testing.T) {
	r, _ := setupTestRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/escrow/"+uuid.New().String(), nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestFundEscrow(t *testing.T) {
	r, svc := setupTestRouter()

	orderID := uuid.New()
	account := &domain.EscrowAccount{
		ID:        uuid.New(),
		OrderID:   orderID,
		Balance:   decimal.Zero,
		Status:    domain.StatusCreated,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	svc.accounts[account.ID] = account

	body := map[string]interface{}{
		"amount": "50.00",
	}
	b, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/escrow/"+account.ID.String()+"/fund", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].(map[string]interface{})
	assert.Equal(t, "FUNDED", data["status"])
}

func TestFundEscrow_InvalidUUID(t *testing.T) {
	r, _ := setupTestRouter()

	body := map[string]interface{}{"amount": "50.00"}
	b, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/escrow/not-a-uuid/fund", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestFundEscrow_NotFound(t *testing.T) {
	r, _ := setupTestRouter()

	body := map[string]interface{}{"amount": "50.00"}
	b, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/escrow/"+uuid.New().String()+"/fund", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestReleaseEscrow(t *testing.T) {
	r, svc := setupTestRouter()

	orderID := uuid.New()
	account := &domain.EscrowAccount{
		ID:        uuid.New(),
		OrderID:   orderID,
		Balance:   decimal.NewFromFloat(100.00),
		Status:    domain.StatusCompleted,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	svc.accounts[account.ID] = account

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/escrow/"+account.ID.String()+"/release", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].(map[string]interface{})
	assert.Equal(t, "RELEASED", data["status"])
}

func TestDisputeEscrow(t *testing.T) {
	r, svc := setupTestRouter()

	orderID := uuid.New()
	account := &domain.EscrowAccount{
		ID:        uuid.New(),
		OrderID:   orderID,
		Balance:   decimal.NewFromFloat(100.00),
		Status:    domain.StatusCompleted,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	svc.accounts[account.ID] = account

	body := map[string]interface{}{
		"reason": "Work not completed",
	}
	b, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/escrow/"+account.ID.String()+"/dispute", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].(map[string]interface{})
	assert.Equal(t, "DISPUTED", data["status"])
}

func TestAdvanceEscrow_Success(t *testing.T) {
	r, svc := setupTestRouter()

	orderID := uuid.New()
	account := &domain.EscrowAccount{
		ID:        uuid.New(),
		OrderID:   orderID,
		Balance:   decimal.NewFromFloat(100.00),
		Status:    domain.StatusFunded,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	svc.accounts[account.ID] = account

	body := map[string]interface{}{
		"status": "IN_PROGRESS",
	}
	b, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/escrow/"+account.ID.String()+"/advance", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].(map[string]interface{})
	assert.Equal(t, "IN_PROGRESS", data["status"])
}

func TestAdvanceEscrow_InvalidUUID(t *testing.T) {
	r, _ := setupTestRouter()

	body := map[string]interface{}{"status": "IN_PROGRESS"}
	b, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/escrow/not-a-uuid/advance", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAdvanceEscrow_NotFound(t *testing.T) {
	r, _ := setupTestRouter()

	body := map[string]interface{}{"status": "IN_PROGRESS"}
	b, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/escrow/"+uuid.New().String()+"/advance", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestInvalidUUID(t *testing.T) {
	r, _ := setupTestRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/escrow/invalid-uuid", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
