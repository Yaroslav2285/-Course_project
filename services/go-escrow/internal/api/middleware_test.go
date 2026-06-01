package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

// --- RateLimiter ---

func TestNewRateLimiter(t *testing.T) {
	rl := NewRateLimiter(10, 20)
	assert.NotNil(t, rl)
	assert.NotNil(t, rl.stopCh)
	rl.Stop()
}

func TestRateLimiter_GetLimiter_SameIP(t *testing.T) {
	rl := NewRateLimiter(10, 20)
	defer rl.Stop()

	l1 := rl.GetLimiter("192.168.1.1")
	l2 := rl.GetLimiter("192.168.1.1")
	assert.Same(t, l1, l2)
}

func TestRateLimiter_GetLimiter_DifferentIP(t *testing.T) {
	rl := NewRateLimiter(10, 20)
	defer rl.Stop()

	l1 := rl.GetLimiter("192.168.1.1")
	l2 := rl.GetLimiter("10.0.0.1")
	assert.NotSame(t, l1, l2)
}

func TestRateLimitMiddleware_Allows(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rl := NewRateLimiter(1000, 1000)
	defer rl.Stop()

	r := gin.New()
	r.Use(RateLimitMiddleware(rl))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRateLimitMiddleware_Blocks(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rl := NewRateLimiter(1, 1)
	defer rl.Stop()

	r := gin.New()
	r.Use(RateLimitMiddleware(rl))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest("GET", "/test", nil)
	req1.RemoteAddr = "10.0.0.1:12345"
	r.ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusOK, w1.Code)

	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/test", nil)
	req2.RemoteAddr = "10.0.0.1:12345"
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusTooManyRequests, w2.Code)
}

func TestRateLimiter_Stop(t *testing.T) {
	rl := NewRateLimiter(10, 20)
	rl.Stop()
}

// --- CORS ---

func TestCORSMiddleware_SetsHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(CORSMiddleware())
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Contains(t, w.Header().Get("Access-Control-Allow-Methods"), "GET")
	assert.Contains(t, w.Header().Get("Access-Control-Allow-Headers"), "X-Idempotency-Key")
	assert.Equal(t, "86400", w.Header().Get("Access-Control-Max-Age"))
}

func TestCORSMiddleware_OPTIONS(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(CORSMiddleware())
	r.OPTIONS("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("OPTIONS", "/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

// --- RequestID ---

func TestRequestIDMiddleware_GeneratesID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(RequestIDMiddleware())
	r.GET("/test", func(c *gin.Context) {
		id, _ := c.Get("request_id")
		c.JSON(http.StatusOK, gin.H{"request_id": id})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	assert.NotEmpty(t, w.Header().Get("X-Request-ID"))
}

func TestRequestIDMiddleware_PropagatesHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(RequestIDMiddleware())
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Request-ID", "client-provided-id")
	r.ServeHTTP(w, req)

	assert.Equal(t, "client-provided-id", w.Header().Get("X-Request-ID"))
}

// --- Recovery ---

func TestRecoveryMiddleware_CatchesPanic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	log, _ := zap.NewDevelopment()

	r := gin.New()
	r.Use(RecoveryMiddleware(log))
	r.GET("/panic", func(c *gin.Context) {
		panic("test panic")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/panic", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- Logger ---

func TestLoggerMiddleware_DoesNotCrash(t *testing.T) {
	gin.SetMode(gin.TestMode)
	log, _ := zap.NewDevelopment()

	r := gin.New()
	r.Use(RequestIDMiddleware())
	r.Use(LoggerMiddleware(log))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestMiddlewareStack(t *testing.T) {
	gin.SetMode(gin.TestMode)
	log, _ := zap.NewDevelopment()
	rl := NewRateLimiter(1000, 1000)
	defer rl.Stop()

	r := gin.New()
	r.Use(RecoveryMiddleware(log))
	r.Use(LoggerMiddleware(log))
	r.Use(RequestIDMiddleware())
	r.Use(CORSMiddleware())
	r.Use(RateLimitMiddleware(rl))
	r.GET("/test", func(c *gin.Context) {
		id, _ := c.Get("request_id")
		c.JSON(http.StatusOK, gin.H{"request_id": id})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "10.0.0.1:54321"
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotEmpty(t, w.Header().Get("X-Request-ID"))
	assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
}
