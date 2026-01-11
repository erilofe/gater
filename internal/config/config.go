package config

import (
	"log"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

// CircuitBreakerConfig holds the settings for the circuit breakers.
type CircuitBreakerConfig struct {
	MaxRequests uint32
	Interval    time.Duration
	Timeout     time.Duration
}

// Config holds the application configuration.
type Config struct {
	// DEPRECATED: Use service discovery instead
	UserServiceURL string
	// DEPRECATED: Use service discovery instead
	PostServiceURL string
	Port           string
	ConsulAddress  string
	CircuitBreaker CircuitBreakerConfig

	// Service Discovery
	DiscoveryProvider string // "consul", "static", "kubernetes", "etcd"
	DiscoveryRoutes   string // JSON array for static provider (future use)
}

// LoadConfig loads the configuration from environment variables.
// It attempts to load from a .env file first.
func LoadConfig() *Config {
	// Load .env file if it exists, but don't fail if it doesn't (e.g. in Docker or Prod)
	_ = godotenv.Load()

	return &Config{
		// Default to empty string to indicate "use discovery" if not provided
		UserServiceURL: getEnv("USER_SERVICE_URL", ""),
		PostServiceURL: getEnv("POST_SERVICE_URL", ""),
		Port:           getEnv("PORT", "8080"),
		ConsulAddress:  getEnv("CONSUL_ADDRESS", "localhost:8500"),
		CircuitBreaker: CircuitBreakerConfig{
			MaxRequests: getEnvAsUint32("CB_MAX_REQUESTS", 1),
			Interval:    getEnvAsDuration("CB_INTERVAL", 10*time.Second),
			Timeout:     getEnvAsDuration("CB_TIMEOUT", 30*time.Second),
		},

		// Service Discovery
		DiscoveryProvider: getEnv("DISCOVERY_PROVIDER", ""),
		DiscoveryRoutes:   getEnv("DISCOVERY_ROUTES", ""),
	}
}

// getEnv retrieves the value of the environment variable named by the key.
func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

// getEnvAsUint32 retrieves the value as a uint32.
func getEnvAsUint32(key string, fallback uint32) uint32 {
	valueStr, exists := os.LookupEnv(key)
	if !exists {
		return fallback
	}
	value, err := strconv.ParseUint(valueStr, 10, 32)
	if err != nil {
		log.Printf("Invalid value for %s: %s. Using fallback: %d", key, valueStr, fallback)
		return fallback
	}
	return uint32(value)
}

// getEnvAsDuration retrieves the value as a time.Duration.
func getEnvAsDuration(key string, fallback time.Duration) time.Duration {
	valueStr, exists := os.LookupEnv(key)
	if !exists {
		return fallback
	}
	value, err := time.ParseDuration(valueStr)
	if err != nil {
		log.Printf("Invalid value for %s: %s. Using fallback: %v", key, valueStr, fallback)
		return fallback
	}
	return value
}
