package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// ServiceConfig holds configuration for a single service from services.yml
type ServiceConfig struct {
	ServiceName string `yaml:"service_name"`
	Resilience  struct {
		CircuitBreaker struct {
			RecoveryTimeout time.Duration `yaml:"recovery_timeout"`
		} `yaml:"circuit_breaker"`
	} `yaml:"resilience"`
}

// servicesFile represents the structure of services.yml
type servicesFile struct {
	Services map[string]*ServiceConfig `yaml:"services"`
}

// LoadServicesFromFile loads service configurations from services.yml
func LoadServicesFromFile(filepath string) (map[string]*ServiceConfig, error) {
	if filepath == "" {
		return nil, fmt.Errorf("services config filepath is empty")
	}

	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to read services config file %s: %w", filepath, err)
	}

	var sf servicesFile
	if err := yaml.Unmarshal(data, &sf); err != nil {
		return nil, fmt.Errorf("failed to parse services config: %w", err)
	}

	if len(sf.Services) == 0 {
		return nil, fmt.Errorf("no services defined in %s", filepath)
	}

	// Validate each service configuration
	for name, cfg := range sf.Services {
		if cfg == nil {
			return nil, fmt.Errorf("service %s has nil configuration", name)
		}

		// Set service name from map key if not set in YAML
		if cfg.ServiceName == "" {
			cfg.ServiceName = name
		}

		// Validate circuit breaker configuration
		if cfg.Resilience.CircuitBreaker.RecoveryTimeout <= 0 {
			return nil, fmt.Errorf("service %s: circuit breaker recovery_timeout must be > 0", name)
		}
	}

	return sf.Services, nil
}
