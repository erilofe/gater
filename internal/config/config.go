package config

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"gopkg.in/yaml.v3"
)

// CircuitBreakerConfig holds the settings for the circuit breakers.
type CircuitBreakerConfig struct {
	MaxRequests uint32
	Interval    time.Duration
	Timeout     time.Duration
}

// Config holds the application configuration.
type Config struct {
	Port           string
	ConsulAddress  string
	CircuitBreaker CircuitBreakerConfig

	// Service Discovery
	DiscoveryProvider string

	// Routing
	RoutesConfigPath string // Path to routes.yml file
}

// RouteConfig holds the configuration for a single route.
type RouteConfig struct {
	Path        string   `yaml:"path"`
	ServiceName string   `yaml:"service_name"`
	Methods     []string `yaml:"methods"`

	// For future purpose
	// LoadBalancer string `yaml:"load_balancer"`
	// Middleware   []string `yaml:"middleware"`
}

// LoadConfig loads the configuration from environment variables.
// It attempts to load from a .env file first.
func LoadConfig() *Config {
	// Load .env file if it exists, but don't fail if it doesn't (e.g. in Docker or Prod)
	_ = godotenv.Load()

	return &Config{
		Port:          getEnv("PORT", "8080"),
		ConsulAddress: getEnv("CONSUL_ADDRESS", "localhost:8500"),
		CircuitBreaker: CircuitBreakerConfig{
			MaxRequests: getEnvAsUint32("CB_MAX_REQUESTS", 1),
			Interval:    getEnvAsDuration("CB_INTERVAL", 10*time.Second),
			Timeout:     getEnvAsDuration("CB_TIMEOUT", 30*time.Second),
		},

		// Service Discovery
		DiscoveryProvider: getEnv("DISCOVERY_PROVIDER", ""),

		// Routing
		RoutesConfigPath: getEnv("ROUTES_CONFIG", "config/routes.yml"),
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

// LoadRoutesFromFile loads route configuration from a YAML file.
func LoadRoutesFromFile(filepath string) ([]RouteConfig, error) {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to read routes file: %w", err)
	}

	var config struct {
		Routes []RouteConfig `yaml:"routes"`
	}

	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse routes YAML: %w", err)
	}

	for i, route := range config.Routes {
		if route.Path == "" || route.ServiceName == "" {
			return nil, fmt.Errorf("route %d: path and service_name are required", i)
		}

		// Enforce safe route prefix rules
		if route.Path[0] != '/' {
			return nil, fmt.Errorf("route %d: path must start with '/'", i)
		}
		if strings.ContainsAny(route.Path, " \t\r\n") {
			return nil, fmt.Errorf("route %d: path must not contain whitespace", i)
		}
		if strings.ContainsAny(route.Path, "?#") {
			return nil, fmt.Errorf("route %d: path must not contain '?' or '#'", i)
		}
		if strings.Contains(route.Path, "//") {
			return nil, fmt.Errorf("route %d: path must not contain consecutive slashes ('//')", i)
		}
		if strings.ContainsAny(route.Path, ":*") {
			return nil, fmt.Errorf("route %d: path must not contain ':' or '*' (use a static prefix only)", i)
		}

		// Normalize trailing slash (keep "/" as-is)
		if len(route.Path) > 1 && strings.HasSuffix(route.Path, "/") {
			route.Path = strings.TrimRight(route.Path, "/")
		}

		// Persist normalization back into slice
		config.Routes[i] = route
	}

	return config.Routes, nil
}
