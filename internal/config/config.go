package config

import (
	"time"

	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	DatabaseURL         string        `envconfig:"DATABASE_URL" default:"postgres://user:pass@localhost/b3_market"`
	DatabaseMaxConns    int32         `envconfig:"DATABASE_MAX_CONNS" default:"25"`
	DatabaseMinConns    int32         `envconfig:"DATABASE_MIN_CONNS" default:"5"`
	DatabaseMaxConnLife time.Duration `envconfig:"DATABASE_MAX_CONN_LIFE" default:"1h"`

	RedisURL string        `envconfig:"REDIS_URL" default:"redis://localhost:6379"`
	CacheTTL time.Duration `envconfig:"CACHE_TTL" default:"1h"`

	BatchSize int `envconfig:"BATCH_SIZE" default:"10000"`
	Workers   int `envconfig:"WORKERS" default:"4"`

	APIHost         string        `envconfig:"API_HOST" default:"0.0.0.0"`
	APIPort         string        `envconfig:"API_PORT" default:"8000"`
	APIReadTimeout  time.Duration `envconfig:"API_READ_TIMEOUT" default:"10s"`
	APIWriteTimeout time.Duration `envconfig:"API_WRITE_TIMEOUT" default:"10s"`

	MetricsEnabled bool `envconfig:"METRICS_ENABLED" default:"true"`
	TracingEnabled bool `envconfig:"TRACING_ENABLED" default:"false"`

	LogLevel    string `envconfig:"LOG_LEVEL" default:"info"`
	LogFormat   string `envconfig:"LOG_FORMAT" default:"json"`
	Environment string `envconfig:"ENVIRONMENT" default:"development"`
}

func Load() *Config {
	var cfg Config
	if err := envconfig.Process("", &cfg); err != nil {
		panic(err)
	}
	return &cfg
}
