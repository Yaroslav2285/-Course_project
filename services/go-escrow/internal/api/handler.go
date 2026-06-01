// LR #10: Multi-lang REST — standard REST API with unified JSON envelope
// LR #5: Highload — context-based handlers with timeout, rate-limited fund

package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/marketplace/go-escrow/internal/domain"
	"github.com/marketplace/go-escrow/internal/service"
)

// EscrowService interface for handler testability.
type EscrowService interface {
	Create(ctx context.Context, req service.CreateEscrowRequest) (*domain.EscrowAccount, error)
	Fund(ctx context.Context, id uuid.UUID, amount decimal.Decimal) (*domain.EscrowAccount, error)
	AdvanceStatus(ctx context.Context, id uuid.UUID, nextStatus domain.EscrowStatus) (*domain.EscrowAccount, error)
	Release(ctx context.Context, id uuid.UUID) (*domain.EscrowAccount, error)
	Cancel(ctx context.Context, id uuid.UUID) (*domain.EscrowAccount, error)
	Dispute(ctx context.Context, id uuid.UUID, reason string) (*domain.EscrowAccount, error)
	Resolve(ctx context.Context, id uuid.UUID) (*domain.EscrowAccount, error)
	GetByID(ctx context.Context, id uuid.UUID) (*domain.EscrowAccount, error)
}

// createEscrowRequest represents the request body for creating an escrow account.
type createEscrowRequest struct {
	OrderID string `json:"order_id" binding:"required" example:"550e8400-e29b-41d4-a716-446655440000"`
	Amount  string `json:"amount" binding:"required" example:"100.0000"`
}

// fundEscrowRequest represents the request body for funding an escrow account.
type fundEscrowRequest struct {
	Amount string `json:"amount" binding:"required" example:"100.0000"`
}

// advanceRequest represents the request body for advancing an escrow status.
type advanceRequest struct {
	Status string `json:"status" binding:"required" example:"IN_PROGRESS" enums:"IN_PROGRESS,COMPLETED"`
}

// disputeRequest represents the request body for disputing an escrow account.
type disputeRequest struct {
	Reason string `json:"reason" example:"Service not delivered as described"`
}

// ErrorItem represents a single validation/error message.
type ErrorItem struct {
	Code   string `json:"code" example:"INVALID_UUID"`
	Detail string `json:"detail" example:"id must be a valid UUID"`
}

// ErrorResponse represents the error envelope returned by the API.
type ErrorResponse struct {
	Errors []ErrorItem `json:"errors"`
}

// SuccessResponse represents the success envelope returned by the API.
type SuccessResponse struct {
	Data interface{} `json:"data"`
}

type EscrowHandler struct {
	svc EscrowService
}

func NewEscrowHandler(svc EscrowService) *EscrowHandler {
	return &EscrowHandler{svc: svc}
}

// Create
// @Summary Create a new escrow account
// @Description Creates a new escrow account linked to an order with the specified amount.
// @Tags escrow
// @Accept json
// @Produce json
// @Param X-Idempotency-Key header string false "Idempotency key (UUID)"
// @Param request body createEscrowRequest true "Create escrow request"
// @Success 201 {object} SuccessResponse "Escrow account created"
// @Failure 400 {object} ErrorResponse "INVALID_UUID or VALIDATION_ERROR"
// @Failure 422 {object} ErrorResponse "VALIDATION_ERROR"
// @Failure 500 {object} ErrorResponse "INTERNAL"
// @Router /v1/escrow [post]
func (h *EscrowHandler) Create(c *gin.Context) {
	var req createEscrowRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeErrorResponse(c, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid request body: "+err.Error())
		return
	}

	orderID, err := uuid.Parse(strings.TrimSpace(req.OrderID))
	if err != nil {
		writeErrorResponse(c, http.StatusBadRequest, "INVALID_UUID", "order_id must be a valid UUID")
		return
	}

	amount, err := decimal.NewFromString(strings.TrimSpace(req.Amount))
	if err != nil {
		writeErrorResponse(c, http.StatusBadRequest, "INVALID_AMOUNT", "amount must be a valid decimal number")
		return
	}
	amount = amount.Truncate(4)

	svcReq := service.CreateEscrowRequest{
		OrderID: orderID,
		Amount:  amount,
	}

	account, err := h.svc.Create(c.Request.Context(), svcReq)
	if err != nil {
		if strings.Contains(err.Error(), "validation") {
			writeErrorResponse(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", err.Error())
			return
		}
		writeErrorResponse(c, http.StatusInternalServerError, "INTERNAL", err.Error())
		return
	}

	successResponse(c, http.StatusCreated, account)
}

// Fund
// @Summary Fund an escrow account
// @Description Adds funds to an existing escrow account. Rate-limited per IP.
// @Tags escrow
// @Accept json
// @Produce json
// @Param id path string true "Escrow account ID (UUID)"
// @Param X-Idempotency-Key header string false "Idempotency key (UUID)"
// @Param request body fundEscrowRequest true "Fund request"
// @Success 200 {object} SuccessResponse "Escrow account funded"
// @Failure 400 {object} ErrorResponse "INVALID_UUID or VALIDATION_ERROR"
// @Failure 404 {object} ErrorResponse "NOT_FOUND"
// @Failure 409 {object} ErrorResponse "INVALID_TRANSITION"
// @Failure 429 {object} ErrorResponse "RATE_LIMITED"
// @Failure 500 {object} ErrorResponse "INTERNAL"
// @Router /v1/escrow/{id}/fund [post]
func (h *EscrowHandler) Fund(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeErrorResponse(c, http.StatusBadRequest, "INVALID_UUID", "id must be a valid UUID")
		return
	}

	var req fundEscrowRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeErrorResponse(c, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid request body: "+err.Error())
		return
	}

	amount, err := decimal.NewFromString(strings.TrimSpace(req.Amount))
	if err != nil {
		writeErrorResponse(c, http.StatusBadRequest, "INVALID_AMOUNT", "amount must be a valid decimal number")
		return
	}
	amount = amount.Truncate(4)

	account, err := h.svc.Fund(c.Request.Context(), id, amount)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeErrorResponse(c, http.StatusNotFound, "NOT_FOUND", err.Error())
			return
		}
		if strings.Contains(err.Error(), "invalid transition") || strings.Contains(err.Error(), "validation") {
			writeErrorResponse(c, http.StatusConflict, "INVALID_TRANSITION", err.Error())
			return
		}
		writeErrorResponse(c, http.StatusInternalServerError, "INTERNAL", err.Error())
		return
	}

	successResponse(c, http.StatusOK, account)
}

// Release
// @Summary Release an escrow account
// @Description Releases funds from the escrow account to the seller.
// @Tags escrow
// @Accept json
// @Produce json
// @Param id path string true "Escrow account ID (UUID)"
// @Param X-Idempotency-Key header string false "Idempotency key (UUID)"
// @Success 200 {object} SuccessResponse "Escrow account released"
// @Failure 400 {object} ErrorResponse "INVALID_UUID"
// @Failure 404 {object} ErrorResponse "NOT_FOUND"
// @Failure 409 {object} ErrorResponse "INVALID_TRANSITION"
// @Failure 500 {object} ErrorResponse "INTERNAL"
// @Router /v1/escrow/{id}/release [post]
func (h *EscrowHandler) Release(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeErrorResponse(c, http.StatusBadRequest, "INVALID_UUID", "id must be a valid UUID")
		return
	}

	account, err := h.svc.Release(c.Request.Context(), id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeErrorResponse(c, http.StatusNotFound, "NOT_FOUND", err.Error())
			return
		}
		if strings.Contains(err.Error(), "invalid transition") {
			writeErrorResponse(c, http.StatusConflict, "INVALID_TRANSITION", err.Error())
			return
		}
		writeErrorResponse(c, http.StatusInternalServerError, "INTERNAL", err.Error())
		return
	}

	successResponse(c, http.StatusOK, account)
}

// Cancel
// @Summary Cancel an escrow account
// @Description Cancels a funded or in-progress escrow account. Balance is zeroed and a cancel event is emitted.
// @Tags escrow
// @Accept json
// @Produce json
// @Param id path string true "Escrow account ID (UUID)"
// @Param X-Idempotency-Key header string false "Idempotency key (UUID)"
// @Success 200 {object} SuccessResponse "Escrow account cancelled"
// @Failure 400 {object} ErrorResponse "INVALID_UUID"
// @Failure 404 {object} ErrorResponse "NOT_FOUND"
// @Failure 409 {object} ErrorResponse "INVALID_TRANSITION"
// @Failure 500 {object} ErrorResponse "INTERNAL"
// @Router /v1/escrow/{id}/cancel [post]
func (h *EscrowHandler) Cancel(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeErrorResponse(c, http.StatusBadRequest, "INVALID_UUID", "id must be a valid UUID")
		return
	}

	account, err := h.svc.Cancel(c.Request.Context(), id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeErrorResponse(c, http.StatusNotFound, "NOT_FOUND", err.Error())
			return
		}
		if strings.Contains(err.Error(), "invalid transition") {
			writeErrorResponse(c, http.StatusConflict, "INVALID_TRANSITION", err.Error())
			return
		}
		writeErrorResponse(c, http.StatusInternalServerError, "INTERNAL", err.Error())
		return
	}

	successResponse(c, http.StatusOK, account)
}

// Dispute
// @Summary Dispute an escrow account
// @Description Opens a dispute for a completed escrow account. Reason is required.
// @Tags escrow
// @Accept json
// @Produce json
// @Param id path string true "Escrow account ID (UUID)"
// @Param X-Idempotency-Key header string false "Idempotency key (UUID)"
// @Param request body disputeRequest true "Dispute request"
// @Success 200 {object} SuccessResponse "Dispute opened"
// @Failure 400 {object} ErrorResponse "INVALID_UUID or VALIDATION_ERROR"
// @Failure 404 {object} ErrorResponse "NOT_FOUND"
// @Failure 409 {object} ErrorResponse "INVALID_TRANSITION"
// @Failure 422 {object} ErrorResponse "VALIDATION_ERROR (reason required)"
// @Failure 500 {object} ErrorResponse "INTERNAL"
// @Router /v1/escrow/{id}/dispute [post]
func (h *EscrowHandler) Dispute(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeErrorResponse(c, http.StatusBadRequest, "INVALID_UUID", "id must be a valid UUID")
		return
	}

	var req disputeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeErrorResponse(c, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid request body: "+err.Error())
		return
	}

	account, err := h.svc.Dispute(c.Request.Context(), id, req.Reason)
	if err != nil {
		if strings.Contains(err.Error(), "reason is required") {
			writeErrorResponse(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", err.Error())
			return
		}
		if strings.Contains(err.Error(), "not found") {
			writeErrorResponse(c, http.StatusNotFound, "NOT_FOUND", err.Error())
			return
		}
		if strings.Contains(err.Error(), "invalid transition") {
			writeErrorResponse(c, http.StatusConflict, "INVALID_TRANSITION", err.Error())
			return
		}
		writeErrorResponse(c, http.StatusInternalServerError, "INTERNAL", err.Error())
		return
	}

	successResponse(c, http.StatusOK, account)
}

// Resolve
// @Summary Resolve a disputed escrow account
// @Description Resolves a disputed escrow account (DISPUTED → RESOLVED). Closes the dispute and records a RESOLVE transaction.
// @Tags escrow
// @Accept json
// @Produce json
// @Param id path string true "Escrow account ID (UUID)"
// @Success 200 {object} SuccessResponse "Dispute resolved"
// @Failure 400 {object} ErrorResponse "INVALID_UUID"
// @Failure 404 {object} ErrorResponse "NOT_FOUND"
// @Failure 409 {object} ErrorResponse "INVALID_TRANSITION"
// @Failure 500 {object} ErrorResponse "INTERNAL"
// @Router /v1/escrow/{id}/resolve [post]
func (h *EscrowHandler) Resolve(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeErrorResponse(c, http.StatusBadRequest, "INVALID_UUID", "id must be a valid UUID")
		return
	}

	account, err := h.svc.Resolve(c.Request.Context(), id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeErrorResponse(c, http.StatusNotFound, "NOT_FOUND", err.Error())
			return
		}
		if strings.Contains(err.Error(), "invalid transition") {
			writeErrorResponse(c, http.StatusConflict, "INVALID_TRANSITION", err.Error())
			return
		}
		writeErrorResponse(c, http.StatusInternalServerError, "INTERNAL", err.Error())
		return
	}

	successResponse(c, http.StatusOK, account)
}

// Advance
// @Summary Advance escrow status
// @Description Advances the escrow account to the next status (FUNDED → IN_PROGRESS → COMPLETED).
// @Tags escrow
// @Accept json
// @Produce json
// @Param id path string true "Escrow account ID (UUID)"
// @Param request body advanceRequest true "Advance request"
// @Success 200 {object} SuccessResponse "Escrow status advanced"
// @Failure 400 {object} ErrorResponse "INVALID_UUID or VALIDATION_ERROR"
// @Failure 404 {object} ErrorResponse "NOT_FOUND"
// @Failure 409 {object} ErrorResponse "INVALID_TRANSITION"
// @Failure 500 {object} ErrorResponse "INTERNAL"
// @Router /v1/escrow/{id}/advance [post]
func (h *EscrowHandler) Advance(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeErrorResponse(c, http.StatusBadRequest, "INVALID_UUID", "id must be a valid UUID")
		return
	}

	var req advanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeErrorResponse(c, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid request body: "+err.Error())
		return
	}

	nextStatus := domain.EscrowStatus(strings.ToUpper(strings.TrimSpace(req.Status)))
	account, err := h.svc.AdvanceStatus(c.Request.Context(), id, nextStatus)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeErrorResponse(c, http.StatusNotFound, "NOT_FOUND", err.Error())
			return
		}
		if strings.Contains(err.Error(), "invalid transition") {
			writeErrorResponse(c, http.StatusConflict, "INVALID_TRANSITION", err.Error())
			return
		}
		writeErrorResponse(c, http.StatusInternalServerError, "INTERNAL", err.Error())
		return
	}

	successResponse(c, http.StatusOK, account)
}

// GetByID
// @Summary Get escrow account by ID
// @Description Retrieves an escrow account by its UUID.
// @Tags escrow
// @Produce json
// @Param id path string true "Escrow account ID (UUID)"
// @Success 200 {object} SuccessResponse "Escrow account details"
// @Failure 400 {object} ErrorResponse "INVALID_UUID"
// @Failure 404 {object} ErrorResponse "NOT_FOUND"
// @Failure 500 {object} ErrorResponse "INTERNAL"
// @Router /v1/escrow/{id} [get]
func (h *EscrowHandler) GetByID(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeErrorResponse(c, http.StatusBadRequest, "INVALID_UUID", "id must be a valid UUID")
		return
	}

	account, err := h.svc.GetByID(c.Request.Context(), id)
	if err != nil {
		writeErrorResponse(c, http.StatusInternalServerError, "INTERNAL", err.Error())
		return
	}
	if account == nil {
		writeErrorResponse(c, http.StatusNotFound, "NOT_FOUND", "Escrow account not found")
		return
	}

	successResponse(c, http.StatusOK, account)
}

func writeErrorResponse(c *gin.Context, statusCode int, code, detail string) {
	c.AbortWithStatusJSON(statusCode, gin.H{
		"errors": []gin.H{
			{"code": code, "detail": detail},
		},
	})
}

func successResponse(c *gin.Context, statusCode int, data interface{}) {
	c.JSON(statusCode, gin.H{
		"data": data,
	})
}
