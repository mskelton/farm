package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig(t *testing.T) {
	tests := []struct {
		name        string
		configYAML  string
		expectError bool
		errorMsg    string
		validate    func(t *testing.T, c *Config)
	}{
		{
			name: "valid config with single target",
			configYAML: `
packages:
  vim:
    source: ./vim
    targets:
      - ~/.config/nvim
`,
			expectError: false,
			validate: func(t *testing.T, c *Config) {
				assert.Len(t, c.Packages, 1)
				assert.NotNil(t, c.Packages["vim"])
				assert.Len(t, c.Packages["vim"].Targets, 1)
			},
		},
		{
			name: "valid config with multiple targets",
			configYAML: `
packages:
  vscode:
    source: ./vscode
    targets:
      - ~/.config/Code/User
      - ~/.config/Cursor/User
`,
			expectError: false,
			validate: func(t *testing.T, c *Config) {
				assert.Len(t, c.Packages, 1)
				assert.NotNil(t, c.Packages["vscode"])
				assert.Len(t, c.Packages["vscode"].Targets, 2)
			},
		},
		{
			name: "config with folding settings",
			configYAML: `
packages:
  config:
    source: ./config
    targets:
      - ~/.config
    default_fold: false
    fold:
      - bin
    no_fold:
      - sensitive
`,
			expectError: false,
			validate: func(t *testing.T, c *Config) {
				pkg := c.Packages["config"]
				assert.False(t, pkg.DefaultFold)
				assert.Contains(t, pkg.Fold, "bin")
				assert.Contains(t, pkg.NoFold, "sensitive")
			},
		},
		{
			name: "missing source",
			configYAML: `
packages:
  vim:
    targets:
      - ~/.config/nvim
`,
			expectError: true,
			errorMsg:    "source is required",
		},
		{
			name: "missing targets",
			configYAML: `
packages:
  vim:
    source: ./vim
`,
			expectError: true,
			errorMsg:    "at least one target is required",
		},
		{
			name: "empty target",
			configYAML: `
packages:
  vim:
    source: ./vim
    targets:
      - ""
`,
			expectError: true,
			errorMsg:    "empty target path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpFile, err := os.CreateTemp("", "dotlink-test-*.yaml")
			require.NoError(t, err)
			defer os.Remove(tmpFile.Name())

			_, err = tmpFile.WriteString(tt.configYAML)
			require.NoError(t, err)
			tmpFile.Close()

			config, err := Load(tmpFile.Name())

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
				if tt.validate != nil {
					tt.validate(t, config)
				}
			}
		})
	}
}

func TestExpandHome(t *testing.T) {
	home, err := os.UserHomeDir()
	require.NoError(t, err)

	tests := []struct {
		input    string
		expected string
	}{
		{"~/test", filepath.Join(home, "test")},
		{"/absolute/path", "/absolute/path"},
		{"relative/path", "relative/path"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := expandHome(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
