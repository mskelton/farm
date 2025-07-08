package config

import (
	"fmt"
	"os"
	"path/filepath"

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

func (c *Config) ShouldIgnore(filename string) bool {
	for _, pattern := range c.ignoreGlobs {
		matched, _ := filepath.Match(pattern, filename)
		if matched {
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
