package config

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setEnv(t *testing.T, vars map[string]string) func() {
	t.Helper()
	prev := make(map[string]string)
	for k, v := range vars {
		prev[k] = os.Getenv(k)
		os.Setenv(k, v)
	}
	return func() {
		for k, v := range prev {
			if v == "" {
				os.Unsetenv(k)
			} else {
				os.Setenv(k, v)
			}
		}
	}
}

func TestLoad_AllEnvSet(t *testing.T) {
	restore := setEnv(t, map[string]string{
		"DB_URL":              "postgres://user:pass@localhost:5432/testdb",
		"GO_ESCROW_PORT":      "9090",
		"RATE_LIMIT_RPS":      "50",
		"RATE_LIMIT_BURST":    "100",
		"IDEMPOTENCY_TTL":     "30m",
		"LOG_LEVEL":           "debug",
		"BLOCKCHAIN_SIM_URL":  "http://localhost:9999",
	})
	defer restore()

	cfg, err := Load()
	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, "postgres://user:pass@localhost:5432/testdb", cfg.DBURL)
	assert.Equal(t, "9090", cfg.Port)
	assert.Equal(t, 50, cfg.RateLimitRPS)
	assert.Equal(t, 100, cfg.RateLimitBurst)
	assert.Equal(t, 30*time.Minute, cfg.IdempotencyTTL)
	assert.Equal(t, "debug", cfg.LogLevel)
	assert.Equal(t, "http://localhost:9999", cfg.BlockchainSimURL)
}

func TestLoad_Defaults(t *testing.T) {
	restore := setEnv(t, map[string]string{
		"DB_URL": "postgres://user:pass@localhost:5432/testdb",
	})
	defer restore()

	cfg, err := Load()
	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, "8081", cfg.Port)
	assert.Equal(t, 10, cfg.RateLimitRPS)
	assert.Equal(t, 20, cfg.RateLimitBurst)
	assert.Equal(t, 1*time.Hour, cfg.IdempotencyTTL)
	assert.Equal(t, "info", cfg.LogLevel)
	assert.Equal(t, "http://blockchain-sim:8082", cfg.BlockchainSimURL)
}

func TestLoad_MissingDBURL(t *testing.T) {
	os.Clearenv()
	_, err := Load()
	assert.Error(t, err)
}

func TestLoad_PartialEnv(t *testing.T) {
	restore := setEnv(t, map[string]string{
		"DB_URL":         "postgres://localhost:5432/mydb",
		"RATE_LIMIT_RPS": "100",
	})
	defer restore()

	cfg, err := Load()
	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, "postgres://localhost:5432/mydb", cfg.DBURL)
	assert.Equal(t, "8081", cfg.Port)
	assert.Equal(t, 100, cfg.RateLimitRPS)
	assert.Equal(t, 20, cfg.RateLimitBurst)
	assert.Equal(t, 1*time.Hour, cfg.IdempotencyTTL)
	assert.Equal(t, "info", cfg.LogLevel)
	assert.Equal(t, "http://blockchain-sim:8082", cfg.BlockchainSimURL)
}
