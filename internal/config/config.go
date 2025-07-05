package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Packages map[string]*Package `yaml:"packages"`
}

type Package struct {
	Source      string   `yaml:"source"`
	Targets     []string `yaml:"targets"`
	NoFold      []string `yaml:"no_fold,omitempty"`
	Fold        []string `yaml:"fold,omitempty"`
	DefaultFold bool     `yaml:"default_fold"`
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

	if err := config.validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &config, nil
}

func (c *Config) validate() error {
	for name, pkg := range c.Packages {
		if pkg.Source == "" {
			return fmt.Errorf("package %s: source is required", name)
		}
		if len(pkg.Targets) == 0 {
			return fmt.Errorf("package %s: at least one target is required", name)
		}

		for _, target := range pkg.Targets {
			if target == "" {
				return fmt.Errorf("package %s: empty target path", name)
			}
		}

		sourceAbs, err := filepath.Abs(pkg.Source)
		if err != nil {
			return fmt.Errorf("package %s: invalid source path: %w", name, err)
		}
		pkg.Source = sourceAbs

		for i, target := range pkg.Targets {
			targetAbs, err := filepath.Abs(expandHome(target))
			if err != nil {
				return fmt.Errorf("package %s: invalid target path %s: %w", name, target, err)
			}
			pkg.Targets[i] = targetAbs
		}
	}
	return nil
}

func expandHome(path string) string {
	if len(path) > 0 && path[0] == '~' {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[1:])
	}
	return path
}
