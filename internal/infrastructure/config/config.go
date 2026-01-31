package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	// Server configuration
	Environment string
	Port        int
	MetricsPort int

	// Database configuration
	DatabaseURL     string
	MigrationsPath  string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration

	// Authentication & Authorization
	JWTSecret     string
	JWTExpiration time.Duration

	// Rate Limiting
	RateLimitRPS   int
	RateLimitBurst int

	// TLS Configuration
	TLSEnabled  bool
	TLSCertFile string
	TLSKeyFile  string

	// Observability
	JaegerEndpoint      string
	PrometheusNamespace string
	LogLevel            string
	LogFormat           string // json or console

	// Graceful Shutdown
	ShutdownTimeout time.Duration

	// Feature Flags
	EnableMetrics     bool
	EnableTracing     bool
	EnableHealthCheck bool
	EnableReflection  bool

	// AWS Configuration
	AWSRegion          string
	SecretsManagerName string
	UseSecretsManager  bool

	// Cache Configuration (for idempotency)
	CacheEnabled bool
	CacheTTL     time.Duration
	CacheMaxSize int

	// Timeouts
	RequestTimeout  time.Duration
	DatabaseTimeout time.Duration
}

func Load() (*Config, error) {
	// Load .env file if exists (for local development)
	_ = godotenv.Load()

	cfg := &Config{
		// Server
		Environment: getEnv("ENVIRONMENT", "development"),
		Port:        getEnvAsInt("PORT", 8080),
		MetricsPort: getEnvAsInt("METRICS_PORT", 9090),

		// Database
		DatabaseURL:     getEnv("DATABASE_URL", ""),
		MigrationsPath:  getEnv("MIGRATIONS_PATH", "./internal/infrastructure/postgres/migrations"),
		MaxOpenConns:    getEnvAsInt("DB_MAX_OPEN_CONNS", 25),
		MaxIdleConns:    getEnvAsInt("DB_MAX_IDLE_CONNS", 5),
		ConnMaxLifetime: getEnvAsDuration("DB_CONN_MAX_LIFETIME", 5*time.Minute),
		ConnMaxIdleTime: getEnvAsDuration("DB_CONN_MAX_IDLE_TIME", 1*time.Minute),

		// Auth
		JWTSecret:     getEnv("JWT_SECRET", ""),
		JWTExpiration: getEnvAsDuration("JWT_EXPIRATION", 24*time.Hour),

		// Rate Limiting
		RateLimitRPS:   getEnvAsInt("RATE_LIMIT_RPS", 1000),
		RateLimitBurst: getEnvAsInt("RATE_LIMIT_BURST", 2000),

		// TLS
		TLSEnabled:  getEnvAsBool("TLS_ENABLED", false),
		TLSCertFile: getEnv("TLS_CERT_FILE", "/etc/tls/tls.crt"),
		TLSKeyFile:  getEnv("TLS_KEY_FILE", "/etc/tls/tls.key"),

		// Observability
		JaegerEndpoint:      getEnv("JAEGER_ENDPOINT", "http://localhost:14268/api/traces"),
		PrometheusNamespace: getEnv("PROMETHEUS_NAMESPACE", "todo_service"),
		LogLevel:            getEnv("LOG_LEVEL", "info"),
		LogFormat:           getEnv("LOG_FORMAT", "json"),

		// Graceful Shutdown
		ShutdownTimeout: getEnvAsDuration("SHUTDOWN_TIMEOUT", 30*time.Second),

		// Feature Flags
		EnableMetrics:     getEnvAsBool("ENABLE_METRICS", true),
		EnableTracing:     getEnvAsBool("ENABLE_TRACING", true),
		EnableHealthCheck: getEnvAsBool("ENABLE_HEALTH_CHECK", true),
		EnableReflection:  getEnvAsBool("ENABLE_REFLECTION", false),

		// AWS
		AWSRegion:          getEnv("AWS_REGION", "us-east-1"),
		SecretsManagerName: getEnv("SECRETS_MANAGER_NAME", ""),
		UseSecretsManager:  getEnvAsBool("USE_SECRETS_MANAGER", false),

		// Cache
		CacheEnabled: getEnvAsBool("CACHE_ENABLED", true),
		CacheTTL:     getEnvAsDuration("CACHE_TTL", 24*time.Hour),
		CacheMaxSize: getEnvAsInt("CACHE_MAX_SIZE", 10000),

		// Timeouts
		RequestTimeout:  getEnvAsDuration("REQUEST_TIMEOUT", 30*time.Second),
		DatabaseTimeout: getEnvAsDuration("DATABASE_TIMEOUT", 10*time.Second),
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return cfg, nil
}

func (c *Config) Validate() error {
	// Database URL is required
	if c.DatabaseURL == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}

	// JWT secret is required in production
	if c.Environment == "production" && c.JWTSecret == "" {
		return fmt.Errorf("JWT_SECRET is required in production")
	}

	// TLS files must exist if TLS is enabled
	if c.TLSEnabled {
		if c.TLSCertFile == "" || c.TLSKeyFile == "" {
			return fmt.Errorf("TLS_CERT_FILE and TLS_KEY_FILE are required when TLS is enabled")
		}
		if _, err := os.Stat(c.TLSCertFile); os.IsNotExist(err) {
			return fmt.Errorf("TLS certificate file not found: %s", c.TLSCertFile)
		}
		if _, err := os.Stat(c.TLSKeyFile); os.IsNotExist(err) {
			return fmt.Errorf("TLS key file not found: %s", c.TLSKeyFile)
		}
	}

	// Port validation
	if c.Port < 1 || c.Port > 65535 {
		return fmt.Errorf("invalid port: %d", c.Port)
	}
	if c.MetricsPort < 1 || c.MetricsPort > 65535 {
		return fmt.Errorf("invalid metrics port: %d", c.MetricsPort)
	}

	// Connection pool validation
	if c.MaxOpenConns < c.MaxIdleConns {
		return fmt.Errorf("max_open_conns (%d) must be >= max_idle_conns (%d)",
			c.MaxOpenConns, c.MaxIdleConns)
	}

	// Log level validation
	validLogLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
	}
	if !validLogLevels[c.LogLevel] {
		return fmt.Errorf("invalid log level: %s (valid: debug, info, warn, error)", c.LogLevel)
	}

	// Log format validation
	if c.LogFormat != "json" && c.LogFormat != "console" {
		return fmt.Errorf("invalid log format: %s (valid: json, console)", c.LogFormat)
	}

	return nil
}

func (c *Config) IsDevelopment() bool {
	return c.Environment == "development" || c.Environment == "dev"
}

func (c *Config) IsProduction() bool {
	return c.Environment == "production" || c.Environment == "prod"
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}

	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return defaultValue
	}
	return value
}

func getEnvAsBool(key string, defaultValue bool) bool {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}

	value, err := strconv.ParseBool(valueStr)
	if err != nil {
		return defaultValue
	}
	return value
}

func getEnvAsDuration(key string, defaultValue time.Duration) time.Duration {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}

	value, err := time.ParseDuration(valueStr)
	if err != nil {
		return defaultValue
	}
	return value
}

type DatabaseConfig struct {
	URL             string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
	Timeout         time.Duration
}

func (c *Config) GetDatabaseConfig() DatabaseConfig {
	return DatabaseConfig{
		URL:             c.DatabaseURL,
		MaxOpenConns:    c.MaxOpenConns,
		MaxIdleConns:    c.MaxIdleConns,
		ConnMaxLifetime: c.ConnMaxLifetime,
		ConnMaxIdleTime: c.ConnMaxIdleTime,
		Timeout:         c.DatabaseTimeout,
	}
}

type ServerConfig struct {
	Port            int
	MetricsPort     int
	TLSEnabled      bool
	TLSCertFile     string
	TLSKeyFile      string
	ShutdownTimeout time.Duration
	RequestTimeout  time.Duration
}

func (c *Config) GetServerConfig() ServerConfig {
	return ServerConfig{
		Port:            c.Port,
		MetricsPort:     c.MetricsPort,
		TLSEnabled:      c.TLSEnabled,
		TLSCertFile:     c.TLSCertFile,
		TLSKeyFile:      c.TLSKeyFile,
		ShutdownTimeout: c.ShutdownTimeout,
		RequestTimeout:  c.RequestTimeout,
	}
}

type ObservabilityConfig struct {
	EnableMetrics       bool
	EnableTracing       bool
	JaegerEndpoint      string
	PrometheusNamespace string
	LogLevel            string
	LogFormat           string
}

func (c *Config) GetObservabilityConfig() ObservabilityConfig {
	return ObservabilityConfig{
		EnableMetrics:       c.EnableMetrics,
		EnableTracing:       c.EnableTracing,
		JaegerEndpoint:      c.JaegerEndpoint,
		PrometheusNamespace: c.PrometheusNamespace,
		LogLevel:            c.LogLevel,
		LogFormat:           c.LogFormat,
	}
}
