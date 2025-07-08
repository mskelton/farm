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

func TestMultiLevelIgnorePatterns(t *testing.T) {
	configYAML := `
ignore:
  - "EmmyLua.spoon/annotations"
  - "deep/nested/path"
  - "*.tmp"
packages:
  - source: ./test
    targets:
      - ./target
`
	tmpFile, err := os.CreateTemp("", "test-multilevel-*.yaml")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(configYAML)
	require.NoError(t, err)
	tmpFile.Close()

	config, err := Load(tmpFile.Name())
	require.NoError(t, err)

	tests := []struct {
		path     string
		expected bool
		desc     string
	}{
		// Multi-level ignore patterns - exact matches
		{"EmmyLua.spoon/annotations", true, "should ignore multi-level path"},
		{"EmmyLua.spoon/annotations/file.lua", true, "should ignore files under multi-level path"},
		{"EmmyLua.spoon/init.lua", false, "should not ignore sibling files"},
		{"deep/nested/path", true, "should ignore nested directory"},
		{"deep/nested/path/file.txt", true, "should ignore files under nested directory"},
		{"deep/nested/other.txt", false, "should not ignore files in parent directory"},

		// Substring matching for multi-level patterns
		{"prefix/EmmyLua.spoon/annotations", true, "should ignore multi-level path anywhere in hierarchy"},
		{"some/prefix/EmmyLua.spoon/annotations/file.lua", true, "should ignore files under substring-matched path"},
		{"other/deep/nested/path", true, "should ignore nested directory anywhere in hierarchy"},
		{"prefix/deep/nested/path/file.txt", true, "should ignore files under substring-matched nested path"},

		// Standard glob patterns
		{"file.tmp", true, "should ignore files matching glob pattern"},
		{"data/file.tmp", true, "should ignore files matching glob pattern in subdirectory"},
		{"file.txt", false, "should not ignore files not matching glob pattern"},

		// Default ignore patterns
		{".git", true, "should ignore git files"},
		{".gitignore", true, "should ignore git files"},
		{"README.md", true, "should ignore README files"},
		{"LICENSE", true, "should ignore LICENSE files"},
		{"COPYING", true, "should ignore COPYING files"},

		// Files that should NOT be ignored
		{"normal.txt", false, "should not ignore normal files"},
		{"EmmyLua.spoon", false, "should not ignore directory itself"},
		{"deep/nested", false, "should not ignore parent directory"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			result := config.ShouldIgnore(tt.path)
			assert.Equal(t, tt.expected, result, "ShouldIgnore(%q) = %v, want %v", tt.path, result, tt.expected)
		})
	}
}

func TestMatchesPath(t *testing.T) {
	config := &Config{}

	tests := []struct {
		pattern  string
		path     string
		expected bool
		desc     string
	}{
		// Direct matches
		{"file.txt", "file.txt", true, "should match exact filename"},
		{"dir/file.txt", "dir/file.txt", true, "should match exact path"},

		// Glob patterns
		{"*.txt", "file.txt", true, "should match glob pattern"},
		{"test*", "test_file.txt", true, "should match glob pattern with prefix"},
		{"*.tmp", "backup.tmp", true, "should match glob pattern with suffix"},

		// Multi-level patterns
		{"EmmyLua.spoon/annotations", "EmmyLua.spoon/annotations", true, "should match multi-level path exactly"},
		{"EmmyLua.spoon/annotations", "EmmyLua.spoon/annotations/file.lua", true, "should match files under multi-level path"},
		{"deep/nested/path", "deep/nested/path", true, "should match nested directory"},
		{"deep/nested/path", "deep/nested/path/file.txt", true, "should match files under nested directory"},

		// Path hierarchy matching
		{"app/data", "app/data/cache/file.txt", true, "should match files in subdirectories"},
		{"app/*/logs", "app/prod/logs", true, "should match with wildcard in middle"},
		{"app/*/logs", "app/prod/logs/app.log", true, "should match files under wildcard pattern"},

		// Substring matching for multi-level patterns
		{"spoon/annotations", "EmmyLua.spoon/annotations", true, "should match multi-level pattern anywhere"},
		{"spoon/annotations", "prefix/EmmyLua.spoon/annotations", true, "should match multi-level pattern with prefix"},
		{"spoon/annotations", "EmmyLua.spoon/annotations/file.lua", true, "should match files under substring-matched pattern"},
		{"nested/path", "deep/nested/path", true, "should match nested pattern anywhere"},
		{"nested/path", "prefix/deep/nested/path/file.txt", true, "should match files under nested substring pattern"},

		// Single-part substring matching
		{"annotations", "EmmyLua.spoon/annotations", true, "should match single pattern anywhere in path"},
		{"annotations", "some/other/annotations/file.lua", true, "should match single pattern in deep path"},
		{"cache", "app/data/cache", true, "should match single directory anywhere"},
		{"cache", "app/data/cache/file.txt", true, "should match files under single pattern anywhere"},

		// Negative cases
		{"file.txt", "other.txt", false, "should not match different filename"},
		{"EmmyLua.spoon/annotations", "EmmyLua.spoon/init.lua", false, "should not match sibling files"},
		{"deep/nested/path", "deep/nested/other.txt", false, "should not match files in parent directory"},
		{"*.tmp", "file.txt", false, "should not match different extension"},
		{"app/data", "app/config", false, "should not match sibling directories"},
		{"app/data", "other/data", false, "should not match different parent"},

		// Edge cases
		{"", "file.txt", false, "empty pattern should not match"},
		{"file.txt", "", false, "should not match empty path"},
		{"", "", true, "empty pattern should match empty path"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			result := config.matchesPath(tt.pattern, tt.path)
			assert.Equal(t, tt.expected, result, "matchesPath(%q, %q) = %v, want %v", tt.pattern, tt.path, result, tt.expected)
		})
	}
}

func TestSubstringIgnorePatterns(t *testing.T) {
	configYAML := `
ignore:
  - "annotations"
  - "spoon/annotations"
  - "path"
  - "nested"
packages:
  - source: ./test
    targets:
      - ./target
`
	tmpFile, err := os.CreateTemp("", "test-substring-*.yaml")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(configYAML)
	require.NoError(t, err)
	tmpFile.Close()

	config, err := Load(tmpFile.Name())
	require.NoError(t, err)

	tests := []struct {
		path     string
		expected bool
		desc     string
	}{
		// Single-part substring matching
		{"annotations", true, "should ignore 'annotations' directory at root"},
		{"some/annotations", true, "should ignore 'annotations' directory anywhere"},
		{"EmmyLua.spoon/annotations", true, "should ignore 'annotations' directory in nested path"},
		{"some/deep/annotations/file.txt", true, "should ignore files under 'annotations' anywhere"},

		{"path", true, "should ignore 'path' directory at root"},
		{"prefix/path", true, "should ignore 'path' directory anywhere"},
		{"deep/nested/path", true, "should ignore 'path' directory in nested location"},

		{"nested", true, "should ignore 'nested' directory at root"},
		{"some/nested", true, "should ignore 'nested' directory anywhere"},
		{"deep/nested/other", true, "should ignore 'nested' directory in path"},

		// Multi-part substring matching
		{"spoon/annotations", true, "should ignore multi-part pattern at root"},
		{"EmmyLua.spoon/annotations", true, "should ignore multi-part pattern anywhere"},
		{"prefix/spoon/annotations", true, "should ignore multi-part pattern with prefix"},
		{"EmmyLua.spoon/annotations/file.lua", true, "should ignore files under multi-part pattern"},

		// Should NOT match
		{"annotation", false, "should not match partial word"},
		{"annotationss", false, "should not match word with suffix"},
		{"spoon/annotation", false, "should not match incomplete multi-part pattern"},
		{"other/file.txt", false, "should not match unrelated files"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			result := config.ShouldIgnore(tt.path)
			assert.Equal(t, tt.expected, result, "ShouldIgnore(%q) = %v, want %v", tt.path, result, tt.expected)
		})
	}
}

func TestConfigIgnoreWithComplexPatterns(t *testing.T) {
	configYAML := `
ignore:
  - "node_modules"
  - "*.log"
  - "build/temp"
  - "src/*/generated"
  - "docs/api/v*/internal"
packages:
  - source: ./project
    targets:
      - ~/.config/project
`
	tmpFile, err := os.CreateTemp("", "test-complex-*.yaml")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(configYAML)
	require.NoError(t, err)
	tmpFile.Close()

	config, err := Load(tmpFile.Name())
	require.NoError(t, err)

	tests := []struct {
		path     string
		expected bool
		desc     string
	}{
		// Directory ignores
		{"node_modules", true, "should ignore node_modules directory"},
		{"node_modules/package/index.js", true, "should ignore files in node_modules"},
		{"src/node_modules", true, "should ignore node_modules directory anywhere"},

		// Glob patterns
		{"app.log", true, "should ignore log files"},
		{"error.log", true, "should ignore log files"},
		{"logs/app.log", true, "should ignore log files in subdirectories"},
		{"app.txt", false, "should not ignore non-log files"},

		// Multi-level patterns
		{"build/temp", true, "should ignore build/temp directory"},
		{"build/temp/cache.dat", true, "should ignore files in build/temp"},
		{"build/output", false, "should not ignore other build directories"},
		{"temp", false, "should not ignore temp at root level"},

		// Wildcard patterns
		{"src/components/generated", true, "should ignore generated in any src subdirectory"},
		{"src/utils/generated", true, "should ignore generated in any src subdirectory"},
		{"src/generated", false, "should not match direct src/generated"},
		{"src/components/generated/types.ts", true, "should ignore files in generated directories"},

		// Complex nested patterns
		{"docs/api/v1/internal", true, "should ignore versioned internal docs"},
		{"docs/api/v2/internal", true, "should ignore versioned internal docs"},
		{"docs/api/v1/internal/secret.md", true, "should ignore files in versioned internal docs"},
		{"docs/api/v1/public", false, "should not ignore public docs"},
		{"docs/api/internal", false, "should not ignore non-versioned internal"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			result := config.ShouldIgnore(tt.path)
			assert.Equal(t, tt.expected, result, "ShouldIgnore(%q) = %v, want %v", tt.path, result, tt.expected)
		})
	}
}
