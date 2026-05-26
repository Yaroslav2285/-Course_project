// LR #10: Multi-lang REST — Gin router with standardized API prefix /v1

package api

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/marketplace/go-escrow/internal/service"
)

func NewRouter(svc *service.EscrowService, log *zap.Logger, rateLimiter *RateLimiter, idempotencyStore *IdempotencyStore) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)

	r := gin.New()

	// Global middleware — order matters
	r.Use(RecoveryMiddleware(log))
	r.Use(LoggerMiddleware(log))
	r.Use(RequestIDMiddleware())
	r.Use(CORSMiddleware())

	// Health endpoint (no auth, no rate limit)
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "service": "go-escrow"})
	})

	handler := NewEscrowHandler(svc)

	// API v1 group
	v1 := r.Group("/v1/escrow")
	{
		// Create escrow account
		v1.POST("/", IdempotencyMiddleware(idempotencyStore), handler.Create)

		// Get by ID
		v1.GET("/:id", handler.GetByID)

		// Fund — with rate limiting
		fundGroup := v1.Group("/:id/fund")
		fundGroup.Use(RateLimitMiddleware(rateLimiter))
		fundGroup.POST("", IdempotencyMiddleware(idempotencyStore), handler.Fund)

		// Release
		v1.POST("/:id/release", IdempotencyMiddleware(idempotencyStore), handler.Release)

		// Dispute
		v1.POST("/:id/dispute", IdempotencyMiddleware(idempotencyStore), handler.Dispute)
	}

	return r
}
