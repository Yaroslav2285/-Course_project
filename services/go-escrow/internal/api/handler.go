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
	Release(ctx context.Context, id uuid.UUID) (*domain.EscrowAccount, error)
	Dispute(ctx context.Context, id uuid.UUID, reason string) (*domain.EscrowAccount, error)
	GetByID(ctx context.Context, id uuid.UUID) (*domain.EscrowAccount, error)
}

type EscrowHandler struct {
	svc EscrowService
}

func NewEscrowHandler(svc EscrowService) *EscrowHandler {
	return &EscrowHandler{svc: svc}
}

func (h *EscrowHandler) Create(c *gin.Context) {
	var req struct {
		OrderID string `json:"order_id" binding:"required"`
		Amount  string `json:"amount" binding:"required"`
	}
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

func (h *EscrowHandler) Fund(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeErrorResponse(c, http.StatusBadRequest, "INVALID_UUID", "id must be a valid UUID")
		return
	}

	var req struct {
		Amount string `json:"amount" binding:"required"`
	}
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

func (h *EscrowHandler) Dispute(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeErrorResponse(c, http.StatusBadRequest, "INVALID_UUID", "id must be a valid UUID")
		return
	}

	var req struct {
		Reason string `json:"reason"`
	}
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
