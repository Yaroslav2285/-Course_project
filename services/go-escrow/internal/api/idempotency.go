// LR #9: Idempotency with background goroutine cleanup (channels + ticker)
// LR #5: Highload — O(1) map lookup for idempotency keys

package api

import (
	"bytes"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type cachedResponse struct {
	StatusCode int
	Body       []byte
	ExpiresAt  time.Time
}

type IdempotencyStore struct {
	mu     sync.RWMutex
	data   map[string]*cachedResponse
	ttl    time.Duration
	stopCh chan struct{}
}

func NewIdempotencyStore(ttl time.Duration) *IdempotencyStore {
	s := &IdempotencyStore{
		data:   make(map[string]*cachedResponse),
		ttl:    ttl,
		stopCh: make(chan struct{}),
	}
	go s.cleanupLoop()
	return s
}

func (s *IdempotencyStore) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.mu.Lock()
			now := time.Now()
			for k, v := range s.data {
				if now.After(v.ExpiresAt) {
					delete(s.data, k)
				}
			}
			s.mu.Unlock()
		case <-s.stopCh:
			return
		}
	}
}

func (s *IdempotencyStore) Stop() {
	close(s.stopCh)
}

func (s *IdempotencyStore) Get(key string) (*cachedResponse, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entry, ok := s.data[key]
	if !ok {
		return nil, false
	}
	if time.Now().After(entry.ExpiresAt) {
		return nil, false
	}
	return entry, true
}

func (s *IdempotencyStore) Set(key string, statusCode int, body []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = &cachedResponse{
		StatusCode: statusCode,
		Body:       body,
		ExpiresAt:  time.Now().Add(s.ttl),
	}
}

func IdempotencyMiddleware(store *IdempotencyStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method != http.MethodPost {
			c.Next()
			return
		}

		key := c.GetHeader("X-Idempotency-Key")
		if key == "" {
			c.Next()
			return
		}

		routeKey := key + ":" + c.Request.URL.Path

		if cached, ok := store.Get(routeKey); ok {
			c.Data(cached.StatusCode, "application/json", cached.Body)
			c.Abort()
			return
		}

		blw := &bodyLogWriter{body: bytes.NewBuffer(nil), ResponseWriter: c.Writer}
		c.Writer = blw

		c.Next()

		if c.Writer.Status() < 500 {
			store.Set(routeKey, c.Writer.Status(), blw.body.Bytes())
		}
	}
}

type bodyLogWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (w *bodyLogWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

func (w *bodyLogWriter) WriteString(s string) (int, error) {
	w.body.WriteString(s)
	return w.ResponseWriter.WriteString(s)
}


