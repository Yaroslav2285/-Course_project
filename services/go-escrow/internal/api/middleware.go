// LR #5: Highload — rate limiter with token bucket, structured logging
// LR #9: Concurrency — sync.Mutex for rate-limiter visitor map

package api

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

type RateLimiter struct {
	mu       sync.Mutex
	visitors map[string]*rate.Limiter
	rps      rate.Limit
	burst    int
	stopCh   chan struct{}
}

func NewRateLimiter(rps, burst int) *RateLimiter {
	rl := &RateLimiter{
		visitors: make(map[string]*rate.Limiter),
		rps:      rate.Limit(rps),
		burst:    burst,
		stopCh:   make(chan struct{}),
	}
	go rl.cleanupLoop()
	return rl
}

func (rl *RateLimiter) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			rl.mu.Lock()
			for ip, lim := range rl.visitors {
				if lim.Tokens() == float64(rl.burst) {
					delete(rl.visitors, ip)
				}
			}
			rl.mu.Unlock()
		case <-rl.stopCh:
			return
		}
	}
}

func (rl *RateLimiter) GetLimiter(ip string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	limiter, ok := rl.visitors[ip]
	if !ok {
		limiter = rate.NewLimiter(rl.rps, rl.burst)
		rl.visitors[ip] = limiter
	}
	return limiter
}

func (rl *RateLimiter) Stop() {
	close(rl.stopCh)
}

func RateLimitMiddleware(rl *RateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		limiter := rl.GetLimiter(ip)
		if !limiter.Allow() {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"errors": []gin.H{
					{"code": "RATE_LIMITED", "detail": "Too many requests. Try again later."},
				},
			})
			c.Abort()
			return
		}
		c.Next()
	}
}

func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID, X-Idempotency-Key")
		c.Header("Access-Control-Max-Age", "86400")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func RequestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		rid := c.GetHeader("X-Request-ID")
		if rid == "" {
			rid = c.Writer.Header().Get("X-Request-ID")
			if rid == "" {
				rid = time.Now().Format("20060102150405") + "-" +
					c.ClientIP()
			}
		}
		c.Set("request_id", rid)
		c.Header("X-Request-ID", rid)
		c.Next()
	}
}

func LoggerMiddleware(log *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()
		rid, _ := c.Get("request_id")

		log.Info("HTTP request",
			zap.String("request_id", rid.(string)),
			zap.Int("status", status),
			zap.String("method", c.Request.Method),
			zap.String("path", path),
			zap.String("query", query),
			zap.Duration("latency", latency),
			zap.String("ip", c.ClientIP()),
		)
	}
}

func RecoveryMiddleware(log *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				log.Error("panic recovered",
					zap.Any("error", err),
					zap.String("path", c.Request.URL.Path),
				)
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"errors": []gin.H{
						{"code": "INTERNAL", "detail": "Internal server error"},
					},
				})
			}
		}()
		c.Next()
	}
}
