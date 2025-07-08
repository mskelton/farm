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
  - source: ./vim
    targets:
      - ~/.config/nvim
`,
			expectError: false,
			validate: func(t *testing.T, c *Config) {
				assert.Len(t, c.Packages, 1)
				assert.NotNil(t, c.Packages[0])
				assert.Len(t, c.Packages[0].Targets, 1)
			},
		},
		{
			name: "valid config with multiple targets",
			configYAML: `
packages:
  - source: ./vscode
    targets:
      - ~/.config/Code/User
      - ~/.config/Cursor/User
`,
			expectError: false,
			validate: func(t *testing.T, c *Config) {
				assert.Len(t, c.Packages, 1)
				assert.NotNil(t, c.Packages[0])
				assert.Len(t, c.Packages[0].Targets, 2)
			},
		},
		{
			name: "config with folding settings",
			configYAML: `
packages:
  - source: ./config
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
				pkg := c.Packages[0]
				assert.False(t, pkg.DefaultFold)
				assert.Contains(t, pkg.Fold, "bin")
				assert.Contains(t, pkg.NoFold, "sensitive")
			},
		},
		{
			name: "missing source",
			configYAML: `
packages:
  - targets:
      - ~/.config/nvim
`,
			expectError: true,
			errorMsg:    "source is required",
		},
		{
			name: "missing targets",
			configYAML: `
packages:
  - source: ./vim
`,
			expectError: true,
			errorMsg:    "at least one target is required",
		},
		{
			name: "empty target",
			configYAML: `
packages:
  - source: ./vim
    targets:
      - ""
`,
			expectError: true,
			errorMsg:    "empty target path",
		},
		{
			name: "config with ignore patterns",
			configYAML: `
ignore:
  - "test*"
  - "*.bak"
packages:
  - source: ./config
    targets:
      - ~/.config
`,
			expectError: false,
			validate: func(t *testing.T, c *Config) {
				assert.Contains(t, c.Ignore, "test*")
				assert.Contains(t, c.Ignore, "*.bak")
				// Check that patterns are compiled
				assert.True(t, c.ShouldIgnore("test.txt"))
				assert.True(t, c.ShouldIgnore("test_file"))
				assert.True(t, c.ShouldIgnore("file.bak"))
				assert.False(t, c.ShouldIgnore("normal.txt"))
			},
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

func TestDefaultIgnorePatterns(t *testing.T) {
	// Create a temporary config with minimal setup
	configYAML := `
packages:
  - source: ./test
    targets:
      - ./target
`
	tmpFile, err := os.CreateTemp("", "test-*.yaml")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(configYAML)
	require.NoError(t, err)
	tmpFile.Close()

	config, err := Load(tmpFile.Name())
	require.NoError(t, err)

	// Test default ignore patterns
	assert.True(t, config.ShouldIgnore(".git"))
	assert.True(t, config.ShouldIgnore(".gitignore"))
	assert.True(t, config.ShouldIgnore(".gitmodules"))
	assert.True(t, config.ShouldIgnore("README"))
	assert.True(t, config.ShouldIgnore("README.md"))
	assert.True(t, config.ShouldIgnore("LICENSE"))
	assert.True(t, config.ShouldIgnore("LICENSE.txt"))
	assert.True(t, config.ShouldIgnore("COPYING"))

	// Should not ignore these files anymore (not in default patterns)
	assert.False(t, config.ShouldIgnore(".svn"))
	assert.False(t, config.ShouldIgnore("CVS"))
	assert.False(t, config.ShouldIgnore("file.txt~"))
	assert.False(t, config.ShouldIgnore("#autosave#"))
	assert.False(t, config.ShouldIgnore("normal.txt"))
	assert.False(t, config.ShouldIgnore("myfile"))
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
