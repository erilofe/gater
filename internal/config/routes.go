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
		if routeCfg.Path[0] != '/' {
			return nil, fmt.Errorf("route %d: path must start with '/'", i)
		}
		if strings.ContainsAny(routeCfg.Path, " \t\r\n") {
			return nil, fmt.Errorf("route %d: path must not contain whitespace", i)
		}
		if strings.ContainsAny(routeCfg.Path, "?#") {
			return nil, fmt.Errorf("route %d: path must not contain '?' or '#'", i)
		}
		if strings.Contains(routeCfg.Path, "//") {
			return nil, fmt.Errorf("route %d: path must not contain consecutive slashes ('//')", i)
		}
		if strings.ContainsAny(routeCfg.Path, ":*") {
			return nil, fmt.Errorf("route %d: path must not contain ':' or '*' (use a static prefix only)", i)
		}

		// Normalize trailing slash (keep "/" as-is)
		path := routeCfg.Path
		if len(path) > 1 && strings.HasSuffix(path, "/") {
			path = strings.TrimRight(path, "/")
		}

		routes = append(routes, &Route{
			Path:        path,
			Methods:     routeCfg.Methods,
			ServiceName: routeCfg.ServiceName,
			Priority:    routeCfg.Priority,
		})
	}

	return routes, nil
}
