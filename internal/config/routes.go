package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Route represents a routing rule (mapping from path to service)
type Route struct {
	Path        string   // URL path prefix
	Methods     []string // HTTP methods (empty = all methods)
	ServiceName string   // Target service name
	Priority    int      // Route priority for ordering (higher = higher priority)
}

// routeConfigYAML represents the YAML structure for loading routes
type routeConfigYAML struct {
	Path        string   `yaml:"path"`
	ServiceName string   `yaml:"service_name"`
	Methods     []string `yaml:"methods"`
	Priority    int      `yaml:"priority"` // Optional, defaults to 0
}

// validateRoutePath enforces safe route prefix rules
func validateRoutePath(path string) error {
	if path == "" {
		return fmt.Errorf("route path cannot be empty")
	}

	// Path must start with '/'
	if path[0] != '/' {
		return fmt.Errorf("route %s: path must start with '/'", path)
	}

	// Path must not contain whitespace
	if strings.ContainsAny(path, " \t\r\n") {
		return fmt.Errorf("route %s: path must not contain whitespace", path)
	}

	// Path must not contain '?' or '#'
	if strings.ContainsAny(path, "?#") {
		return fmt.Errorf("route \"%s\": path must not contain '?' or '#'", path)
	}

	// Path must not contain consecutive slashes
	if strings.Contains(path, "//") {
		return fmt.Errorf("route \"%s\": path must not contain consecutive slashes ('//')", path)
	}

	// Path must not contain ':' or '*'
	if strings.ContainsAny(path, ":*") {
		return fmt.Errorf("route \"%s\": path must not contain ':' or '*' (use a static prefix only)", path)
	}

	return nil
}

// normalizeRouteMethods normalizes HTTP methods to uppercase and validates them
func normalizeRouteMethods(methods []string) ([]string, error) {
	normalized := make([]string, 0, len(methods))
	seen := make(map[string]struct{}) // To avoid duplicates

	for _, method := range methods {
		// Normalize to uppercase and trim spaces
		m := strings.ToUpper(strings.TrimSpace(method))

		// Skip empty methods
		if m == "" {
			continue
		}

		if _, exists := seen[m]; !exists {
			seen[m] = struct{}{}
			normalized = append(normalized, m)
		}
	}

	// If all methods were empty, return an error
	if len(normalized) == 0 && len(methods) > 0 {
		return nil, fmt.Errorf("route methods are empty after normalization")
	}

	return normalized, nil
}

// LoadRoutesFromFile loads route configuration from a YAML file
func LoadRoutesFromFile(filepath string) ([]*Route, error) {
	if filepath == "" {
		return nil, fmt.Errorf("routes config filepath is empty")
	}

	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to read routes file: %w", err)
	}

	var config struct {
		Routes []routeConfigYAML `yaml:"routes"`
	}

	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse routes YAML: %w", err)
	}

	if len(config.Routes) == 0 {
		return nil, fmt.Errorf("no routes defined in %s", filepath)
	}

	routes := make([]*Route, 0, len(config.Routes))

	for i, routeCfg := range config.Routes {
		if routeCfg.Path == "" || routeCfg.ServiceName == "" {
			return nil, fmt.Errorf("route %d: path and service_name are required", i)
		}

		// Enforce safe route prefix rules
		if err := validateRoutePath(routeCfg.Path); err != nil {
			return nil, err
		}

		// Normalize and validate HTTP methods
		normalizedMethods, err := normalizeRouteMethods(routeCfg.Methods)
		if err != nil {
			return nil, fmt.Errorf("route %s: %w", routeCfg.Path, err)
		}

		// Normalize trailing slash (keep "/" as-is)
		path := routeCfg.Path
		if len(routeCfg.Path) > 1 && strings.HasSuffix(routeCfg.Path, "/") {
			path = strings.TrimRight(routeCfg.Path, "/")
		}

		routes = append(routes, &Route{
			Path:        path,
			Methods:     normalizedMethods,
			ServiceName: routeCfg.ServiceName,
			Priority:    routeCfg.Priority,
		})
	}

	return routes, nil
}
