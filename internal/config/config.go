package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Packages    []*Package `yaml:"packages"`
	Ignore      []string   `yaml:"ignore,omitempty"`
	IgnoreGlobs []string
}

type Package struct {
	Source       string   `yaml:"source"`
	Targets      []string `yaml:"targets"`
	NoFold       []string `yaml:"no_fold,omitempty"`
	Fold         []string `yaml:"fold,omitempty"`
	DefaultFold  bool     `yaml:"default_fold"`
	Environments []string `yaml:"environments,omitempty"`
}

var defaultIgnorePatterns = []string{
	".DS_Store",
	".git*",
	"README*",
	"LICENSE*",
	"COPYING",
}

func Load(configPath string) (*Config, error) {
	if configPath == "" {
		configPath = "farm.yaml"
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &config, nil
}

func (c *Config) Validate() error {
	for i, pkg := range c.Packages {
		if pkg.Source == "" {
			return fmt.Errorf("package %d: source is required", i)
		}

		if len(pkg.Targets) == 0 {
			return fmt.Errorf("package %d: at least one target is required", i)
		}

		for _, target := range pkg.Targets {
			if target == "" {
				return fmt.Errorf("package %d: empty target path", i)
			}
		}

		sourceAbs, err := filepath.Abs(pkg.Source)
		if err != nil {
			return fmt.Errorf("package %d: invalid source path: %w", i, err)
		}
		pkg.Source = sourceAbs

		for j, target := range pkg.Targets {
			targetAbs, err := filepath.Abs(expandHome(target))
			if err != nil {
				return fmt.Errorf("package %d: invalid target path %s: %w", i, target, err)
			}
			pkg.Targets[j] = targetAbs
		}
	}

	// Compile ignore patterns at config level
	allPatterns := defaultIgnorePatterns
	allPatterns = append(allPatterns, c.Ignore...)
	c.IgnoreGlobs = allPatterns

	return nil
}

func (c *Config) ShouldIgnore(path string) bool {
	for _, pattern := range c.IgnoreGlobs {
		if c.matchesPath(pattern, path) {
			return true
		}
	}
	return false
}

func (c *Config) matchesPath(pattern, path string) bool {
	// Direct match
	if pattern == path {
		return true
	}

	// Check if path is under the pattern directory
	if strings.HasPrefix(path, pattern+"/") {
		return true
	}

	// Split pattern and path into parts
	pathParts := strings.Split(path, "/")
	patternParts := strings.Split(pattern, "/")

	// Multi-level pattern matching (pattern contains '/')
	if len(patternParts) > 1 {
		// Try exact substring matching - check if pattern appears anywhere in the path
		for startIdx := 0; startIdx <= len(pathParts)-len(patternParts); startIdx++ {
			allMatch := true
			for i := range patternParts {
				if matched, _ := filepath.Match(patternParts[i], pathParts[startIdx+i]); !matched {
					allMatch = false
					break
				}
			}
			if allMatch {
				return true
			}
		}

		// Also try substring matching within path components
		// This handles cases like "spoon/annotations" matching "EmmyLua.spoon/annotations"
		pathString := path
		patternString := pattern

		// Check if the pattern appears as a substring in the path
		if strings.Contains(pathString, patternString) {
			return true
		}

		// Check if pattern matches when we consider partial path components
		for startIdx := 0; startIdx < len(pathParts); startIdx++ {
			if len(pathParts[startIdx:]) >= len(patternParts) {
				allMatch := true
				for i := range patternParts {
					pathComponent := pathParts[startIdx+i]
					patternComponent := patternParts[i]

					// Try exact match first
					if matched, _ := filepath.Match(patternComponent, pathComponent); matched {
						continue
					}

					// Try substring match within the component
					if strings.Contains(pathComponent, patternComponent) {
						continue
					}

					allMatch = false
					break
				}
				if allMatch {
					return true
				}
			}
		}

		return false
	}

	// Single-part pattern matching
	// First try full path match for glob patterns
	if matched, _ := filepath.Match(pattern, path); matched {
		return true
	}

	// Check if single pattern matches any directory component in the path
	for _, part := range pathParts {
		if matched, _ := filepath.Match(pattern, part); matched {
			return true
		}
	}

	return false
}

func expandHome(path string) string {
	if len(path) > 0 && path[0] == '~' {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[1:])
	}
	return path
}

func (c *Config) GetPackagesForEnvironment(env string) []*Package {
	if env == "" {
		// If no environment specified, return all packages that don't have environment restrictions
		var packages []*Package
		for _, pkg := range c.Packages {
			if len(pkg.Environments) == 0 {
				packages = append(packages, pkg)
			}
		}
		return packages
	}

	var packages []*Package
	for _, pkg := range c.Packages {
		// Include packages that are either:
		// 1. Not environment-specific (no environments field)
		// 2. Explicitly enabled for the current environment
		if len(pkg.Environments) == 0 || contains(pkg.Environments, env) {
			packages = append(packages, pkg)
		}
	}
	return packages
}

func (c *Config) GetAvailableEnvironments() []string {
	envMap := make(map[string]bool)
	for _, pkg := range c.Packages {
		for _, env := range pkg.Environments {
			envMap[env] = true
		}
	}

	var environments []string
	for env := range envMap {
		environments = append(environments, env)
	}
	return environments
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
