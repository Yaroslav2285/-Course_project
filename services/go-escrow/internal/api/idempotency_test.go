package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// --- IdempotencyStore ---

func TestNewIdempotencyStore(t *testing.T) {
	store := NewIdempotencyStore(5 * time.Minute)
	assert.NotNil(t, store)
	assert.NotNil(t, store.stopCh)
	store.Stop()
}

func TestIdempotencyStore_SetAndGet(t *testing.T) {
	store := NewIdempotencyStore(5 * time.Minute)
	defer store.Stop()

	store.Set("key1", http.StatusOK, []byte(`{"status":"ok"}`))

	cached, ok := store.Get("key1")
	assert.True(t, ok)
	assert.Equal(t, http.StatusOK, cached.StatusCode)
	assert.Equal(t, `{"status":"ok"}`, string(cached.Body))
}

func TestIdempotencyStore_Get_Missing(t *testing.T) {
	store := NewIdempotencyStore(5 * time.Minute)
	defer store.Stop()

	cached, ok := store.Get("nonexistent")
	assert.False(t, ok)
	assert.Nil(t, cached)
}

func TestIdempotencyStore_Get_Expired(t *testing.T) {
	store := NewIdempotencyStore(1 * time.Nanosecond)
	defer store.Stop()

	store.Set("key1", http.StatusOK, []byte(`{"status":"ok"}`))
	time.Sleep(10 * time.Millisecond)

	cached, ok := store.Get("key1")
	assert.False(t, ok)
	assert.Nil(t, cached)
}

func TestIdempotencyStore_Stop(t *testing.T) {
	store := NewIdempotencyStore(5 * time.Minute)
	store.Stop()
}

// --- IdempotencyMiddleware ---

func TestIdempotencyMiddleware_NoKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := NewIdempotencyStore(5 * time.Minute)
	defer store.Stop()

	r := gin.New()
	r.Use(IdempotencyMiddleware(store))
	callCount := 0
	r.POST("/test", func(c *gin.Context) {
		callCount++
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, 1, callCount)
}

func TestIdempotencyMiddleware_FirstRequestPasses(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := NewIdempotencyStore(5 * time.Minute)
	defer store.Stop()

	r := gin.New()
	r.Use(IdempotencyMiddleware(store))
	callCount := 0
	r.POST("/test", func(c *gin.Context) {
		callCount++
		c.JSON(http.StatusOK, gin.H{"status": "ok", "call": callCount})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/test", nil)
	req.Header.Set("X-Idempotency-Key", "key-1")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, 1, callCount)
}

func TestIdempotencyMiddleware_SecondRequestReturnsCached(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := NewIdempotencyStore(5 * time.Minute)
	defer store.Stop()

	r := gin.New()
	r.Use(IdempotencyMiddleware(store))
	callCount := 0
	r.POST("/test", func(c *gin.Context) {
		callCount++
		c.JSON(http.StatusOK, gin.H{"status": "ok", "call": callCount})
	})

	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest("POST", "/test", nil)
	req1.Header.Set("X-Idempotency-Key", "same-key")
	r.ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusOK, w1.Code)
	assert.Equal(t, 1, callCount)

	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("POST", "/test", nil)
	req2.Header.Set("X-Idempotency-Key", "same-key")
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)
	assert.Equal(t, 1, callCount, "handler should not be called again")
	assert.Equal(t, w1.Body.String(), w2.Body.String())
}

func TestIdempotencyMiddleware_DifferentKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := NewIdempotencyStore(5 * time.Minute)
	defer store.Stop()

	r := gin.New()
	r.Use(IdempotencyMiddleware(store))
	callCount := 0
	r.POST("/test", func(c *gin.Context) {
		callCount++
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest("POST", "/test", nil)
	req1.Header.Set("X-Idempotency-Key", "key-1")
	r.ServeHTTP(w1, req1)
	assert.Equal(t, 1, callCount)

	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("POST", "/test", nil)
	req2.Header.Set("X-Idempotency-Key", "key-2")
	r.ServeHTTP(w2, req2)
	assert.Equal(t, 2, callCount)
}

func TestIdempotencyMiddleware_SkipNonPost(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := NewIdempotencyStore(5 * time.Minute)
	defer store.Stop()

	r := gin.New()
	r.Use(IdempotencyMiddleware(store))
	callCount := 0
	r.GET("/test", func(c *gin.Context) {
		callCount++
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Idempotency-Key", "some-key")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, 1, callCount)
}

func TestIdempotencyMiddleware_ServerErrorNotCached(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := NewIdempotencyStore(5 * time.Minute)
	defer store.Stop()

	r := gin.New()
	r.Use(IdempotencyMiddleware(store))
	callCount := 0
	r.POST("/test", func(c *gin.Context) {
		callCount++
		c.JSON(http.StatusInternalServerError, gin.H{"error": "server error"})
	})

	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest("POST", "/test", nil)
	req1.Header.Set("X-Idempotency-Key", "err-key")
	r.ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusInternalServerError, w1.Code)
	assert.Equal(t, 1, callCount)

	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("POST", "/test", nil)
	req2.Header.Set("X-Idempotency-Key", "err-key")
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusInternalServerError, w2.Code)
	assert.Equal(t, 2, callCount, "5xx should not be cached, handler should be called again")
}
