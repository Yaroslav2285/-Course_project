// LR #13: Testing/Automation — processRetryQueue тесты с mock HTTP
// LR #10: Multi-lang/REST — httptest.Server для мокирования Blockchain Sim

package clients

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestSubmitEvent_QueueAndRetry(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	var callCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewBlockchainClient(server.URL, logger)
	// Stop the retry goroutine started by NewBlockchainClient, we'll test manually
	// Actually, we want to test the full SubmitEvent + processRetryQueue flow.
	// SubmitEvent will fail (500) and queue for retry.
	// processRetryQueue will retry until exhausted.

	event := &BlockchainEvent{
		OrderID: "order-retry",
		Action:  "CREATED",
	}

	_, err := client.SubmitEvent(context.Background(), event)
	assert.NoError(t, err)

	// Allow time for retry queue to process (maxRetries-1 = 2 additional retries)
	time.Sleep(50 * time.Millisecond)

	// Initial call by SubmitEvent + up to 2 retries by processRetryQueue
	final := callCount.Load()
	assert.GreaterOrEqual(t, final, int32(2), "should have retried at least once")
	assert.LessOrEqual(t, final, int32(5), "should not retry excessively")
}

func TestSubmitEvent_RetrySuccess(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	var callCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := callCount.Add(1)
		if count <= 1 {
			// First call fails (SubmitEvent will retry internally)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"block_index": 2, "tx_hash": "retry-success"}`))
	}))
	defer server.Close()

	client := NewBlockchainClient(server.URL, logger)

	event := &BlockchainEvent{
		OrderID: "order-retry-success",
		Action:  "FUNDED",
	}

	// SubmitEvent fails internally (500) and queues for retry
	result, err := client.SubmitEvent(context.Background(), event)
	assert.NoError(t, err, "SubmitEvent should queue for async retry and return nil")
	assert.Nil(t, result, "SubmitEvent result should be nil on queueing")

	// Wait for retry goroutine to process and succeed
	time.Sleep(100 * time.Millisecond)

	final := callCount.Load()
	assert.GreaterOrEqual(t, final, int32(2), "should have retried at least once")
}

func TestSubmitEvent_QueueFull(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	// Construct client without starting the retry goroutine
	client := &BlockchainClient{
		baseURL:    server.URL,
		httpClient: &http.Client{Timeout: defaultTimeout},
		log:        logger,
		retryQueue: make(chan *retryItem, retryQueueCap),
	}

	// Fill the queue to capacity
	for i := 0; i < retryQueueCap; i++ {
		client.retryQueue <- &retryItem{
			event:   &BlockchainEvent{OrderID: "fill", Action: "CREATED"},
			attempt: 1,
		}
	}

	// Event should fail to enqueue because queue is full
	event := &BlockchainEvent{
		OrderID: "dropped",
		Action:  "RELEASED",
	}

	_, err := client.SubmitEvent(context.Background(), event)
	assert.Error(t, err)
}

func TestProcessRetryQueue_EmptyQueue(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	client := NewBlockchainClient("http://localhost:1", logger)

	// With an empty queue, processRetryQueue blocks on channel read.
	// Just verify the client was created and no panic occurs.
	assert.NotNil(t, client)
	assert.NotNil(t, client.retryQueue)
	assert.Equal(t, 0, len(client.retryQueue))
}

func TestProcessRetryQueue_Exhausted(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	var callCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewBlockchainClient(server.URL, logger)

	// Submit one event directly to the retry queue (bypass SubmitEvent)
	client.retryQueue <- &retryItem{
		event:   &BlockchainEvent{OrderID: "exhaust", Action: "RELEASED"},
		attempt: maxRetries,
	}

	// Wait for processRetryQueue to process it
	time.Sleep(50 * time.Millisecond)

	// processRetryQueue calls doRequest with attempt+1 = maxRetries+1,
	// which is > maxRetries, so no internal retry. The error log is emitted.
	// server should have been called exactly once
	assert.Equal(t, int32(1), callCount.Load(), "server should be called exactly once (no re-queue at maxRetries)")
}

func TestBlockchainClient_NewClientAndStop(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	client := NewBlockchainClient("http://localhost:9999", logger)
	assert.NotNil(t, client)
	assert.NotNil(t, client.retryQueue)
	assert.Equal(t, "http://localhost:9999", client.baseURL)
	assert.NotNil(t, client.httpClient)
	assert.Equal(t, 5*time.Second, client.httpClient.Timeout)
}
