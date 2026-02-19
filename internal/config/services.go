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
	LoadBalancer *LoadBalancerConfig `yaml:"load_balancer"`
	Validator    *ValidatorConfig    `yaml:"validator"`
}

type ValidatorConfig struct {
	AllowWebsocket *bool `yaml:"websocket"`
}

type LoadBalancerConfig struct {
	Endpoints []EndpointConfig `yaml:"endpoints"`
}

type EndpointConfig struct {
	Address string `yaml:"address"`
	Port    *int   `yaml:"port"`
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

		// If not specified, default value for websocket is false
		if cfg.Validator == nil || cfg.Validator.AllowWebsocket == nil {
			cfg.Validator = &ValidatorConfig{
				AllowWebsocket: new(bool),
			}
			*cfg.Validator.AllowWebsocket = false
		}

		if cfg.LoadBalancer != nil {
			if len(cfg.LoadBalancer.Endpoints) == 0 {
				return nil, fmt.Errorf("service %s: load balancer specified at configuration level, must have at least one endpoint", name)
			}

			for i := range cfg.LoadBalancer.Endpoints {
				endpoint := &cfg.LoadBalancer.Endpoints[i]
				if endpoint.Address == "" {
					return nil, fmt.Errorf("service %s: load balancer endpoint address cannot be empty", name)
				}

				if endpoint.Port == nil {
					return nil, fmt.Errorf("service %s: load balancer endpoint port must be specified", name)
				} else if *endpoint.Port < 1 || *endpoint.Port > 65535 {
					//Sanity check, accept only valid port ranges
					return nil, fmt.Errorf("service %s: load balancer endpoint should specify valid port ranges (must be positive, must not be superior to 65535)", name)
				}
			}
		}

		// Validate circuit breaker configuration
		if cfg.Resilience.CircuitBreaker.RecoveryTimeout <= 0 {
			return nil, fmt.Errorf("service %s: circuit breaker recovery_timeout must be > 0", name)
		}
	}

	return sf.Services, nil
}
