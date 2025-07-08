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
	ignoreGlobs []string
}

type Package struct {
	Source      string   `yaml:"source"`
	Targets     []string `yaml:"targets"`
	NoFold      []string `yaml:"no_fold,omitempty"`
	Fold        []string `yaml:"fold,omitempty"`
	DefaultFold bool     `yaml:"default_fold"`
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
	c.ignoreGlobs = allPatterns

	return nil
}

func (c *Config) ShouldIgnore(path string) bool {
	for _, pattern := range c.ignoreGlobs {
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
		// Try to match from start of path
		if len(pathParts) >= len(patternParts) {
			for i := range patternParts {
				if matched, _ := filepath.Match(patternParts[i], pathParts[i]); !matched {
					return false
				}
			}
			return true
		}
		return false
	}
	
	// Single-part pattern matching
	// First try full path match for glob patterns
	if matched, _ := filepath.Match(pattern, path); matched {
		return true
	}
	
	// For filename patterns, check the last component
	filename := filepath.Base(path)
	if matched, _ := filepath.Match(pattern, filename); matched {
		// If pattern contains wildcards, it's likely a file pattern (e.g., "*.log")
		if strings.ContainsAny(pattern, "*?[") {
			return true
		}
		
		// For non-wildcard patterns (like "node_modules"), only match if:
		// 1. It's at the root level (filename == path)
		// 2. Or it's a direct child of root (no intermediate directories)
		if filename == path {
			return true // Root level file/directory
		}
		
		// Check if it's a top-level directory name (e.g., "node_modules" should match "node_modules" but not "src/node_modules")
		parentPath := strings.TrimSuffix(path, "/"+filename)
		return !strings.Contains(parentPath, "/")
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
