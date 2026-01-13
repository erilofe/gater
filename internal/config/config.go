package config

import (
	"os"

	"github.com/joho/godotenv"
)

// Config holds the application configuration.
type Config struct {
	Port          string
	ConsulAddress string

	// Service Discovery
	DiscoveryProvider string

	// Configuration Paths
	RoutesConfigPath   string // Path to routes.yml file
	ServicesConfigPath string // Path to services.yml file
}

// LoadConfig loads the configuration from environment variables.
// It attempts to load from a .env file first.
func LoadConfig() *Config {
	// Load .env file if it exists, but don't fail if it doesn't (e.g. in Docker or Prod)
	_ = godotenv.Load()

	return &Config{
		Port:          getEnv("PORT", "8080"),
		ConsulAddress: getEnv("CONSUL_ADDRESS", "localhost:8500"),

		// Service Discovery
		DiscoveryProvider: getEnv("DISCOVERY_PROVIDER", ""),

		// Configuration Paths
		RoutesConfigPath:   getEnv("ROUTES_CONFIG", "config/routes.yml"),
		ServicesConfigPath: getEnv("SERVICES_CONFIG", "config/services.yml"),
	}
}

// getEnv retrieves the value of the environment variable named by the key.
func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}
