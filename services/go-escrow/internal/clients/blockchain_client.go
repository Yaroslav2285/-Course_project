// LR #10: Multi-lang/REST — Go → Blockchain Sim HTTP client
// LR #13: Testing/Automation — retry with jitter, graceful fallback
// LR #5: Highload — non-blocking error queue, context-based timeouts

package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"time"

	"go.uber.org/zap"

	"github.com/google/uuid"
)

const (
	defaultTimeout  = 5 * time.Second
	maxRetries      = 3
	baseBackoff     = 100 * time.Millisecond
	retryQueueCap   = 100
)

// BlockchainEvent represents an event sent to the blockchain simulator.
type BlockchainEvent struct {
	OrderID string `json:"order_id"`
	Action  string `json:"action"`
	Data    any    `json:"data"`
}

// BlockchainClient sends events to the blockchain simulator.
type BlockchainClient struct {
	baseURL    string
	httpClient *http.Client
	log        *zap.Logger
	retryQueue chan *retryItem
}

type retryItem struct {
	event   *BlockchainEvent
	attempt int
}

// NewBlockchainClient creates a new client with the given base URL and logger.
func NewBlockchainClient(baseURL string, log *zap.Logger) *BlockchainClient {
	c := &BlockchainClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
		log:        log,
		retryQueue: make(chan *retryItem, retryQueueCap),
	}
	go c.processRetryQueue()
	return c
}

// SubmitEvent sends an event to the blockchain simulator with retry logic.
// Returns the HTTP response body on success. Non-blocking on failure — queues for retry.
func (c *BlockchainClient) SubmitEvent(ctx context.Context, event *BlockchainEvent) (map[string]any, error) {
	body, err := c.doRequest(ctx, event, 1)
	if err == nil {
		return body, nil
	}

	c.log.Warn("blockchain submit failed, queueing for retry",
		zap.String("order_id", event.OrderID),
		zap.String("action", event.Action),
		zap.Error(err),
	)

	select {
	case c.retryQueue <- &retryItem{event: event, attempt: 1}:
	default:
		c.log.Warn("retry queue full, dropping event",
			zap.String("order_id", event.OrderID),
			zap.String("action", event.Action),
		)
	}

	return nil, fmt.Errorf("blockchain submit failed after %d retries: %w", maxRetries, err)
}

func (c *BlockchainClient) doRequest(ctx context.Context, event *BlockchainEvent, attempt int) (map[string]any, error) {
	payload, err := json.Marshal(event)
	if err != nil {
		return nil, fmt.Errorf("marshal event: %w", err)
	}

	url := fmt.Sprintf("%s/v1/chain/submit", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Request-ID", uuid.New().String())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if attempt < maxRetries {
			c.log.Debug("blockchain retry",
				zap.Int("attempt", attempt),
				zap.String("order_id", event.OrderID),
				zap.Error(err),
			)
			jitter := time.Duration(rand.Intn(50)) * time.Millisecond // #nosec G404 — jitter is timing-only, not cryptographic security
			delay := baseBackoff*time.Duration(1<<(attempt-1)) + jitter
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
			return c.doRequest(ctx, event, attempt+1)
		}
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return result, nil
}

func (c *BlockchainClient) processRetryQueue() {
	for item := range c.retryQueue {
		ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
		result, err := c.doRequest(ctx, item.event, item.attempt+1)
		cancel()

		if err != nil {
			if item.attempt < maxRetries {
				c.log.Warn("retry failed, re-queueing",
					zap.Int("attempt", item.attempt),
					zap.String("order_id", item.event.OrderID),
					zap.String("action", item.event.Action),
					zap.Error(err),
				)
				select {
				case c.retryQueue <- &retryItem{event: item.event, attempt: item.attempt + 1}:
				default:
					c.log.Warn("retry queue full, dropping retry",
						zap.String("order_id", item.event.OrderID),
					)
				}
			} else {
				c.log.Error("blockchain retry exhausted",
					zap.String("order_id", item.event.OrderID),
					zap.String("action", item.event.Action),
					zap.Error(err),
				)
			}
		} else {
			c.log.Info("blockchain event submitted (retry)",
				zap.String("order_id", item.event.OrderID),
				zap.String("action", item.event.Action),
				zap.Any("result", result),
			)
		}
	}
}
