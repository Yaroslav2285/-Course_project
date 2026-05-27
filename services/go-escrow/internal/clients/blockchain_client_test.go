// LR #13: Testing/Automation — table-driven тесты blockchain клиента
// LR #10: Multi-lang/REST — httptest.Server для мокирования Blockchain Sim

package clients

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func newTestLogger() *zap.Logger {
	logger, _ := zap.NewDevelopment()
	return logger
}

func TestSubmitEvent_Success(t *testing.T) {
	logger := newTestLogger()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/chain/submit", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.NotEmpty(t, r.Header.Get("X-Request-ID"))

		var event BlockchainEvent
		err := json.NewDecoder(r.Body).Decode(&event)
		require.NoError(t, err)
		assert.Equal(t, "test-order", event.OrderID)
		assert.Equal(t, "FUNDED", event.Action)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = fmt.Fprint(w, `{"block_index": 1, "tx_hash": "abc123"}`)
	}))
	defer server.Close()

	client := NewBlockchainClient(server.URL, logger)
	event := &BlockchainEvent{
		OrderID: "test-order",
		Action:  "FUNDED",
		Data:    map[string]any{"escrow_id": "esc-123", "status": "FUNDED", "amount": "500.0000"},
	}

	result, err := client.SubmitEvent(context.Background(), event)
	require.NoError(t, err)
	assert.Equal(t, float64(1), result["block_index"])
	assert.Equal(t, "abc123", result["tx_hash"])
}

func TestSubmitEvent_ServerError(t *testing.T) {
	logger := newTestLogger()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewBlockchainClient(server.URL, logger)
	event := &BlockchainEvent{
		OrderID: "test-order",
		Action:  "CREATED",
	}

	result, err := client.SubmitEvent(context.Background(), event)
	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestSubmitEvent_Timeout(t *testing.T) {
	logger := newTestLogger()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate timeout by not responding
		select {}
	}))
	defer server.Close()

	client := NewBlockchainClient(server.URL, logger)
	client.httpClient.Timeout = 1

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	event := &BlockchainEvent{
		OrderID: "test-order",
		Action:  "RELEASED",
	}

	_, err := client.SubmitEvent(ctx, event)
	assert.Error(t, err)
}

func TestNewBlockchainClient(t *testing.T) {
	logger := newTestLogger()
	client := NewBlockchainClient("http://localhost:8082", logger)
	assert.NotNil(t, client)
	assert.NotNil(t, client.retryQueue)
	assert.Equal(t, "http://localhost:8082", client.baseURL)
}

func TestSubmitEvent_AllActions(t *testing.T) {
	logger := newTestLogger()
	actions := []string{"CREATED", "FUNDED", "RELEASED", "DISPUTED"}

	for _, action := range actions {
		t.Run(action, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var event BlockchainEvent
				err := json.NewDecoder(r.Body).Decode(&event)
				require.NoError(t, err)
				assert.Equal(t, action, event.Action)

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusCreated)
				_, _ = fmt.Fprint(w, `{"block_index": 1, "tx_hash": "xyz"}`)
			}))
			defer server.Close()

			client := NewBlockchainClient(server.URL, logger)
			result, err := client.SubmitEvent(context.Background(), &BlockchainEvent{
				OrderID: "order-1",
				Action:  action,
			})
			require.NoError(t, err)
			assert.Equal(t, "xyz", result["tx_hash"])
		})
	}
}
