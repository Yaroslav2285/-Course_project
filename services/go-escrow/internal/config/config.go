// LR #5: Highload — fail-fast config validation on startup

package config

import (
	"fmt"
	"time"

	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	DBURL          string        `envconfig:"DB_URL" required:"true"`
	Port           string        `envconfig:"GO_ESCROW_PORT" default:"8081"`
	RateLimitRPS   int           `envconfig:"RATE_LIMIT_RPS" default:"10"`
	RateLimitBurst int           `envconfig:"RATE_LIMIT_BURST" default:"20"`
	IdempotencyTTL time.Duration `envconfig:"IDEMPOTENCY_TTL" default:"1h"`
	LogLevel       string        `envconfig:"LOG_LEVEL" default:"info"`
}

func Load() (*Config, error) {
	var cfg Config
	if err := envconfig.Process("", &cfg); err != nil {
		return nil, fmt.Errorf("envconfig: %w", err)
	}
	return &cfg, nil
}
