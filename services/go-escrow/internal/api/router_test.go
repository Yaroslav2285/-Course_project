package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	"github.com/marketplace/go-escrow/internal/service"
)

func newTestRouter(t *testing.T) (*gin.Engine, *RateLimiter, *IdempotencyStore) {
	t.Helper()
	log, _ := zap.NewDevelopment()
	rl := NewRateLimiter(1000, 2000)
	store := NewIdempotencyStore(5 * time.Minute)

	svc := service.NewEscrowService(nil, nil, log, nil)
	r := NewRouter(svc, log, rl, store)
	return r, rl, store
}

func TestNewRouter_HealthEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r, rl, store := newTestRouter(t)
	defer rl.Stop()
	defer store.Stop()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/health", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	assert.Contains(t, w.Body.String(), `"status":"ok"`)
	assert.Contains(t, w.Body.String(), `"service":"go-escrow"`)
}

func TestNewRouter_RouteRegistration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r, rl, store := newTestRouter(t)
	defer rl.Stop()
	defer store.Stop()

	type expectedRoute struct {
		method string
		path   string
	}

	expectedRoutes := []expectedRoute{
		{"GET", "/health"},
		{"GET", "/swagger/*any"},
		{"POST", "/v1/escrow"},
		{"GET", "/v1/escrow/:id"},
		{"POST", "/v1/escrow/:id/fund"},
		{"POST", "/v1/escrow/:id/advance"},
		{"POST", "/v1/escrow/:id/release"},
		{"POST", "/v1/escrow/:id/cancel"},
		{"POST", "/v1/escrow/:id/dispute"},
		{"POST", "/v1/escrow/:id/resolve"},
	}

	registered := r.Routes()
	assert.Len(t, registered, len(expectedRoutes), "should have exactly %d routes", len(expectedRoutes))

	for _, exp := range expectedRoutes {
		found := false
		for _, route := range registered {
			if route.Method == exp.method && route.Path == exp.path {
				found = true
				break
			}
		}
		assert.True(t, found, "route %s %s should be registered", exp.method, exp.path)
	}
}

func TestNewRouter_GlobalMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r, rl, store := newTestRouter(t)
	defer rl.Stop()
	defer store.Stop()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/health", nil)
	r.ServeHTTP(w, req)

	assert.NotEmpty(t, w.Header().Get("X-Request-ID"), "RequestIDMiddleware should set X-Request-ID")
	assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"), "CORSMiddleware should set Allow-Origin")
	assert.Contains(t, w.Header().Get("Access-Control-Allow-Methods"), "GET", "CORSMiddleware should set Allow-Methods")
}

func TestNewRouter_NotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r, rl, store := newTestRouter(t)
	defer rl.Stop()
	defer store.Stop()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/nonexistent", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestNewRouter_UnregisteredRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r, rl, store := newTestRouter(t)
	defer rl.Stop()
	defer store.Stop()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/v1/escrow/some-id", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestNewRouter_CORS_Preflight(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r, rl, store := newTestRouter(t)
	defer rl.Stop()
	defer store.Stop()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("OPTIONS", "/v1/escrow", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "86400", w.Header().Get("Access-Control-Max-Age"))
}
