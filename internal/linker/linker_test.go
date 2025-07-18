package linker

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mskelton/farm/internal/config"
	"github.com/mskelton/farm/internal/lockfile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestEnvironment(t *testing.T) (string, string, string) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "source")
	targetDir := filepath.Join(tmpDir, "target")

	require.NoError(t, os.MkdirAll(sourceDir, 0755))
	require.NoError(t, os.MkdirAll(targetDir, 0755))

	return tmpDir, sourceDir, targetDir
}

func TestLinkSimpleFile(t *testing.T) {
	_, sourceDir, targetDir := setupTestEnvironment(t)

	testFile := filepath.Join(sourceDir, "test.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("test content"), 0644))

	cfg := &config.Config{
		Packages: []*config.Package{
			{
				Source:  sourceDir,
				Targets: []string{targetDir},
			},
		},
	}

	lock := lockfile.New()
	linker := New(cfg, lock, false)

	result, err := linker.Link()
	require.NoError(t, err)
	assert.Len(t, result.Created, 1)
	assert.Empty(t, result.Removed)
	assert.Empty(t, result.Errors)

	expectedLink := filepath.Join(targetDir, "test.txt")
	assert.Contains(t, result.Created, expectedLink)

	info, err := os.Lstat(expectedLink)
	require.NoError(t, err)
	assert.True(t, info.Mode()&os.ModeSymlink != 0)

	content, err := os.ReadFile(expectedLink)
	require.NoError(t, err)
	assert.Equal(t, "test content", string(content))

	assert.Contains(t, lock.Symlinks, expectedLink)
	assert.Equal(t, testFile, lock.Symlinks[expectedLink].Source)
	assert.Equal(t, expectedLink, lock.Symlinks[expectedLink].Target)
	assert.False(t, lock.Symlinks[expectedLink].IsFolded)
}

func TestLinkMultipleTargets(t *testing.T) {
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "source")
	target1Dir := filepath.Join(tmpDir, "target1")
	target2Dir := filepath.Join(tmpDir, "target2")

	require.NoError(t, os.MkdirAll(sourceDir, 0755))
	require.NoError(t, os.MkdirAll(target1Dir, 0755))
	require.NoError(t, os.MkdirAll(target2Dir, 0755))

	testFile := filepath.Join(sourceDir, "test.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("test content"), 0644))

	cfg := &config.Config{
		Packages: []*config.Package{
			{
				Source:  sourceDir,
				Targets: []string{target1Dir, target2Dir},
			},
		},
	}

	lock := lockfile.New()
	linker := New(cfg, lock, false)

	result, err := linker.Link()
	require.NoError(t, err)
	assert.Len(t, result.Created, 2)

	expectedLink1 := filepath.Join(target1Dir, "test.txt")
	expectedLink2 := filepath.Join(target2Dir, "test.txt")

	assert.Contains(t, result.Created, expectedLink1)
	assert.Contains(t, result.Created, expectedLink2)

	for _, link := range []string{expectedLink1, expectedLink2} {
		info, err := os.Lstat(link)
		require.NoError(t, err)
		assert.True(t, info.Mode()&os.ModeSymlink != 0)

		content, err := os.ReadFile(link)
		require.NoError(t, err)
		assert.Equal(t, "test content", string(content))
	}
}

func TestFoldingBehavior(t *testing.T) {
	_, sourceDir, targetDir := setupTestEnvironment(t)

	foldDir := filepath.Join(sourceDir, "fold-me")
	noFoldDir := filepath.Join(sourceDir, "no-fold-me")
	require.NoError(t, os.MkdirAll(foldDir, 0755))
	require.NoError(t, os.MkdirAll(noFoldDir, 0755))

	require.NoError(t, os.WriteFile(filepath.Join(foldDir, "file1.txt"), []byte("fold1"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(noFoldDir, "file2.txt"), []byte("nofold2"), 0644))

	cfg := &config.Config{
		Packages: []*config.Package{
			{
				Source:      sourceDir,
				Targets:     []string{targetDir},
				DefaultFold: false,
				Fold:        []string{"fold-me"},
				NoFold:      []string{"no-fold-me"},
			},
		},
	}

	lock := lockfile.New()
	linker := New(cfg, lock, false)

	_, err := linker.Link()
	require.NoError(t, err)

	foldedLink := filepath.Join(targetDir, "fold-me")
	info, err := os.Lstat(foldedLink)
	require.NoError(t, err)
	assert.True(t, info.Mode()&os.ModeSymlink != 0)

	unfoldedFile := filepath.Join(targetDir, "no-fold-me", "file2.txt")
	info, err = os.Lstat(unfoldedFile)
	require.NoError(t, err)
	assert.True(t, info.Mode()&os.ModeSymlink != 0)
	assert.False(t, info.IsDir())
}

func TestRemoveDeadLinks(t *testing.T) {
	_, sourceDir, targetDir := setupTestEnvironment(t)

	deadSource := filepath.Join(sourceDir, "dead.txt")
	require.NoError(t, os.WriteFile(deadSource, []byte("dead"), 0644))

	deadTarget := filepath.Join(targetDir, "dead.txt")
	require.NoError(t, os.Symlink(deadSource, deadTarget))

	lock := lockfile.New()
	lock.AddSymlink(deadTarget, deadSource, false)

	require.NoError(t, os.Remove(deadSource))

	cfg := &config.Config{
		Packages: []*config.Package{},
	}

	linker := New(cfg, lock, false)
	result, err := linker.Link()
	require.NoError(t, err)

	assert.Len(t, result.Removed, 1)
	assert.Contains(t, result.Removed, deadTarget)

	_, err = os.Lstat(deadTarget)
	assert.True(t, os.IsNotExist(err))
}

func TestDryRun(t *testing.T) {
	_, sourceDir, targetDir := setupTestEnvironment(t)

	testFile := filepath.Join(sourceDir, "test.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("test"), 0644))

	cfg := &config.Config{
		Packages: []*config.Package{
			{
				Source:  sourceDir,
				Targets: []string{targetDir},
			},
		},
	}

	lock := lockfile.New()
	linker := New(cfg, lock, true) // dry run

	result, err := linker.Link()
	require.NoError(t, err)
	assert.Len(t, result.Created, 1)

	expectedLink := filepath.Join(targetDir, "test.txt")
	_, err = os.Lstat(expectedLink)
	assert.True(t, os.IsNotExist(err))
}

func TestUnlink(t *testing.T) {
	_, sourceDir, targetDir := setupTestEnvironment(t)

	testFile := filepath.Join(sourceDir, "test.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("test"), 0644))

	targetFile := filepath.Join(targetDir, "test.txt")
	require.NoError(t, os.Symlink(testFile, targetFile))

	lock := lockfile.New()
	lock.AddSymlink(targetFile, testFile, false)

	cfg := &config.Config{
		Packages: []*config.Package{},
	}

	linker := New(cfg, lock, false)
	result, err := linker.Unlink()
	require.NoError(t, err)

	assert.Len(t, result.Removed, 1)
	assert.Contains(t, result.Removed, targetFile)

	_, err = os.Lstat(targetFile)
	assert.True(t, os.IsNotExist(err))
}

func TestReplaceExistingSymlink(t *testing.T) {
	_, sourceDir, targetDir := setupTestEnvironment(t)

	oldSource := filepath.Join(sourceDir, "old.txt")
	newSource := filepath.Join(sourceDir, "new.txt")
	require.NoError(t, os.WriteFile(oldSource, []byte("old"), 0644))
	require.NoError(t, os.WriteFile(newSource, []byte("new"), 0644))

	targetFile := filepath.Join(targetDir, "test.txt")
	require.NoError(t, os.Symlink(oldSource, targetFile))

	cfg := &config.Config{
		Packages: []*config.Package{
			{
				Source:  sourceDir,
				Targets: []string{targetDir},
			},
		},
	}

	lock := lockfile.New()
	linker := New(cfg, lock, false)

	require.NoError(t, os.Rename(newSource, filepath.Join(sourceDir, "test.txt")))

	result, err := linker.Link()
	require.NoError(t, err)
	assert.Len(t, result.Created, 2)

	content, err := os.ReadFile(targetFile)
	require.NoError(t, err)
	assert.Equal(t, "new", string(content))
}

func TestIgnorePatterns(t *testing.T) {
	_, sourceDir, targetDir := setupTestEnvironment(t)

	// Create files that should be ignored by default patterns
	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, ".git"), []byte("git"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, ".gitignore"), []byte("gitignore"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, "README.md"), []byte("readme"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, "LICENSE"), []byte("license"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, "COPYING"), []byte("copying"), 0644))
	// These should NOT be ignored with new patterns
	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, "backup.txt~"), []byte("backup"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, "normal.txt"), []byte("normal"), 0644))

	cfg := &config.Config{
		Packages: []*config.Package{
			{
				Source:  sourceDir,
				Targets: []string{targetDir},
			},
		},
	}

	// Validate config to compile ignore patterns
	err := cfg.Validate()
	require.NoError(t, err)

	lock := lockfile.New()
	linker := New(cfg, lock, false)

	result, err := linker.Link()
	require.NoError(t, err)

	// Both normal.txt and backup.txt~ should be linked (not ignored by default)
	assert.Len(t, result.Created, 2)
	assert.Contains(t, result.Created, filepath.Join(targetDir, "normal.txt"))
	assert.Contains(t, result.Created, filepath.Join(targetDir, "backup.txt~"))

	// Verify ignored files don't exist in target
	_, err = os.Lstat(filepath.Join(targetDir, ".git"))
	assert.True(t, os.IsNotExist(err))
	_, err = os.Lstat(filepath.Join(targetDir, ".gitignore"))
	assert.True(t, os.IsNotExist(err))
	_, err = os.Lstat(filepath.Join(targetDir, "README.md"))
	assert.True(t, os.IsNotExist(err))
	_, err = os.Lstat(filepath.Join(targetDir, "LICENSE"))
	assert.True(t, os.IsNotExist(err))
	_, err = os.Lstat(filepath.Join(targetDir, "COPYING"))
	assert.True(t, os.IsNotExist(err))
}

func TestCustomIgnorePatterns(t *testing.T) {
	_, sourceDir, targetDir := setupTestEnvironment(t)

	// Create test files
	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, "test_file.txt"), []byte("test"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, "data.bak"), []byte("backup"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, "keep.txt"), []byte("keep"), 0644))

	cfg := &config.Config{
		Ignore: []string{"test_*", "*.bak"},
		Packages: []*config.Package{
			{
				Source:  sourceDir,
				Targets: []string{targetDir},
			},
		},
	}

	// Validate config to compile ignore patterns
	err := cfg.Validate()
	require.NoError(t, err)

	lock := lockfile.New()
	linker := New(cfg, lock, false)

	result, err := linker.Link()
	require.NoError(t, err)

	// Only keep.txt should be linked
	assert.Len(t, result.Created, 1)
	assert.Contains(t, result.Created, filepath.Join(targetDir, "keep.txt"))
}

func TestExistingSymlinkAddedToLockfile(t *testing.T) {
	_, sourceDir, targetDir := setupTestEnvironment(t)

	// Create source file
	testFile := filepath.Join(sourceDir, "test.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("test content"), 0644))

	// Create symlink manually (simulating existing symlink)
	targetFile := filepath.Join(targetDir, "test.txt")
	relSource, err := filepath.Rel(filepath.Dir(targetFile), testFile)
	require.NoError(t, err)
	require.NoError(t, os.Symlink(relSource, targetFile))

	cfg := &config.Config{
		Packages: []*config.Package{
			{
				Source:  sourceDir,
				Targets: []string{targetDir},
			},
		},
	}

	err = cfg.Validate()
	require.NoError(t, err)

	lock := lockfile.New()
	linker := New(cfg, lock, false)

	result, err := linker.Link()
	require.NoError(t, err)

	// No new symlinks should be created since it already exists
	assert.Len(t, result.Created, 0)
	assert.Empty(t, result.Removed)
	assert.Empty(t, result.Errors)

	// But the existing symlink should be in the lockfile
	assert.Len(t, lock.Symlinks, 1)
	assert.Contains(t, lock.Symlinks, targetFile)
	assert.Equal(t, targetFile, lock.Symlinks[targetFile].Target)
	assert.Equal(t, testFile, lock.Symlinks[targetFile].Source)
	assert.False(t, lock.Symlinks[targetFile].IsFolded)
}

func TestNestedFolding(t *testing.T) {
	_, sourceDir, targetDir := setupTestEnvironment(t)

	nestedDir := filepath.Join(sourceDir, "parent", "child")
	require.NoError(t, os.MkdirAll(nestedDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(nestedDir, "file.txt"), []byte("nested"), 0644))

	tests := []struct {
		name         string
		defaultFold  bool
		fold         []string
		noFold       []string
		expectFolded map[string]bool
	}{
		{
			name:        "default no fold",
			defaultFold: false,
			fold:        []string{},
			noFold:      []string{},
			expectFolded: map[string]bool{
				"parent":       false,
				"parent/child": false,
			},
		},
		{
			name:        "fold parent",
			defaultFold: false,
			fold:        []string{"parent"},
			noFold:      []string{},
			expectFolded: map[string]bool{
				"parent": true,
			},
		},
		{
			name:        "fold child only",
			defaultFold: false,
			fold:        []string{"parent/child"},
			noFold:      []string{},
			expectFolded: map[string]bool{
				"parent":       false,
				"parent/child": true,
			},
		},
		{
			name:        "default fold with exception",
			defaultFold: true,
			fold:        []string{},
			noFold:      []string{"parent/child"},
			expectFolded: map[string]bool{
				"parent":       false, // parent cannot be folded because it contains no_fold subdirectory
				"parent/child": false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testTargetDir := filepath.Join(targetDir, tt.name)
			require.NoError(t, os.MkdirAll(testTargetDir, 0755))

			cfg := &config.Config{
				Packages: []*config.Package{
					{
						Source:      sourceDir,
						Targets:     []string{testTargetDir},
						DefaultFold: tt.defaultFold,
						Fold:        tt.fold,
						NoFold:      tt.noFold,
					},
				},
			}

			lock := lockfile.New()
			linker := New(cfg, lock, false)

			_, err := linker.Link()
			require.NoError(t, err)

			for path, shouldBeFolded := range tt.expectFolded {
				checkPath := filepath.Join(testTargetDir, path)
				info, err := os.Lstat(checkPath)
				require.NoError(t, err)

				if shouldBeFolded {
					assert.True(t, info.Mode()&os.ModeSymlink != 0, "%s should be a symlink", path)
				} else {
					if info.Mode()&os.ModeSymlink != 0 {
						t.Errorf("%s should not be a folded symlink", path)
					}
				}
			}
		})
	}
}

func TestMultiLevelIgnorePatterns(t *testing.T) {
	_, sourceDir, targetDir := setupTestEnvironment(t)

	// Create directories and files for multi-level testing
	emmyDir := filepath.Join(sourceDir, "EmmyLua.spoon")
	require.NoError(t, os.MkdirAll(emmyDir, 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(emmyDir, "annotations"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(emmyDir, "annotations", "file.lua"), []byte("annotation"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(emmyDir, "init.lua"), []byte("init"), 0644))

	// Create nested directories
	require.NoError(t, os.MkdirAll(filepath.Join(sourceDir, "deep", "nested", "path"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, "deep", "nested", "path", "file.txt"), []byte("deep file"), 0644))

	cfg := &config.Config{
		Ignore: []string{
			"EmmyLua.spoon/annotations", // Multi-level ignore pattern
			"nested/path",               // Another multi-level pattern
		},
		Packages: []*config.Package{
			{
				Source:  sourceDir,
				Targets: []string{targetDir},
			},
		},
	}

	err := cfg.Validate()
	require.NoError(t, err)

	lock := lockfile.New()
	linker := New(cfg, lock, false)

	result, err := linker.Link()
	require.NoError(t, err)

	// Check that ignored paths are not present in target
	_, err = os.Lstat(filepath.Join(targetDir, "EmmyLua.spoon", "annotations"))
	assert.True(t, os.IsNotExist(err), "EmmyLua.spoon/annotations should be ignored")

	_, err = os.Lstat(filepath.Join(targetDir, "deep", "nested", "path"))
	assert.True(t, os.IsNotExist(err), "nested/path should be ignored")

	// Verify that we don't have the ignored files in the result
	for _, created := range result.Created {
		assert.NotContains(t, created, "annotations/file.lua")
		assert.NotContains(t, created, "path/file.txt")
	}
}

func TestMultiLevelNoFoldPatterns(t *testing.T) {
	_, sourceDir, targetDir := setupTestEnvironment(t)

	// Create the directory structure from the example config
	claudeDir := filepath.Join(sourceDir, ".claude")
	require.NoError(t, os.MkdirAll(claudeDir, 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(claudeDir, "commands"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(claudeDir, "commands", "cmd.sh"), []byte("command"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(claudeDir, "config.json"), []byte("config"), 0644))

	configDir := filepath.Join(sourceDir, ".config")
	require.NoError(t, os.MkdirAll(configDir, 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(configDir, "nvim"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(configDir, "nvim", "init.vim"), []byte("vim config"), 0644))

	hammerspoonDir := filepath.Join(sourceDir, ".hammerspoon")
	require.NoError(t, os.MkdirAll(hammerspoonDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(hammerspoonDir, "init.lua"), []byte("hammerspoon"), 0644))

	// Create some other directories that should be folded by default
	require.NoError(t, os.MkdirAll(filepath.Join(sourceDir, "other-dir"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, "other-dir", "file.txt"), []byte("other"), 0644))

	cfg := &config.Config{
		Packages: []*config.Package{
			{
				Source:      sourceDir,
				Targets:     []string{targetDir},
				DefaultFold: true, // Fold by default
				NoFold: []string{
					".claude/commands", // Multi-level no_fold pattern
					".config/nvim",     // Another multi-level pattern
					".hammerspoon",     // Single-level pattern
				},
			},
		},
	}

	err := cfg.Validate()
	require.NoError(t, err)

	lock := lockfile.New()
	linker := New(cfg, lock, false)

	result, err := linker.Link()
	require.NoError(t, err)

	// Check that no_fold patterns are NOT folded (individual files should be linked)
	_, err = os.Lstat(filepath.Join(targetDir, ".claude", "commands", "cmd.sh"))
	assert.NoError(t, err, ".claude/commands/cmd.sh should be individually linked")

	_, err = os.Lstat(filepath.Join(targetDir, ".config", "nvim", "init.vim"))
	assert.NoError(t, err, ".config/nvim/init.vim should be individually linked")

	_, err = os.Lstat(filepath.Join(targetDir, ".hammerspoon", "init.lua"))
	assert.NoError(t, err, ".hammerspoon/init.lua should be individually linked")

	// Check that other directories are folded as expected
	info, err := os.Lstat(filepath.Join(targetDir, "other-dir"))
	require.NoError(t, err)
	assert.True(t, info.Mode()&os.ModeSymlink != 0, "other-dir should be folded (symlinked)")

	// Check that parent directories that should be folded are folded
	info, err = os.Lstat(filepath.Join(targetDir, ".claude"))
	require.NoError(t, err)
	assert.False(t, info.Mode()&os.ModeSymlink != 0, ".claude should not be folded because it contains no_fold subdirectory")

	// Verify file counts
	expectedIndividualFiles := 4 // cmd.sh, config.json, init.vim, init.lua
	expectedFoldedDirs := 1      // other-dir
	totalExpected := expectedIndividualFiles + expectedFoldedDirs

	assert.Equal(t, totalExpected, len(result.Created))
}

func TestMultiLevelFoldPatterns(t *testing.T) {
	_, sourceDir, targetDir := setupTestEnvironment(t)

	// Create nested structure
	require.NoError(t, os.MkdirAll(filepath.Join(sourceDir, "app", "data", "cache"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, "app", "data", "cache", "file1.txt"), []byte("cache1"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, "app", "data", "cache", "file2.txt"), []byte("cache2"), 0644))

	require.NoError(t, os.MkdirAll(filepath.Join(sourceDir, "app", "data", "logs"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, "app", "data", "logs", "app.log"), []byte("log"), 0644))

	require.NoError(t, os.MkdirAll(filepath.Join(sourceDir, "app", "src"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, "app", "src", "main.js"), []byte("main"), 0644))

	cfg := &config.Config{
		Packages: []*config.Package{
			{
				Source:      sourceDir,
				Targets:     []string{targetDir},
				DefaultFold: false, // Don't fold by default
				Fold: []string{
					"app/data/cache", // Multi-level fold pattern
					"app/data/logs",  // Another multi-level pattern
				},
			},
		},
	}

	err := cfg.Validate()
	require.NoError(t, err)

	lock := lockfile.New()
	linker := New(cfg, lock, false)

	result, err := linker.Link()
	require.NoError(t, err)

	// Check that fold patterns are folded
	info, err := os.Lstat(filepath.Join(targetDir, "app", "data", "cache"))
	require.NoError(t, err)
	assert.True(t, info.Mode()&os.ModeSymlink != 0, "app/data/cache should be folded")

	info, err = os.Lstat(filepath.Join(targetDir, "app", "data", "logs"))
	require.NoError(t, err)
	assert.True(t, info.Mode()&os.ModeSymlink != 0, "app/data/logs should be folded")

	// Check that non-fold patterns are NOT folded
	_, err = os.Lstat(filepath.Join(targetDir, "app", "src", "main.js"))
	assert.NoError(t, err, "app/src/main.js should be individually linked")

	// Verify we have the expected number of created symlinks
	expectedSymlinks := 3 // cache (folded), logs (folded), main.js (individual)
	assert.Equal(t, expectedSymlinks, len(result.Created))
}

func TestMixedMultiLevelPatterns(t *testing.T) {
	_, sourceDir, targetDir := setupTestEnvironment(t)

	// Create a complex directory structure
	require.NoError(t, os.MkdirAll(filepath.Join(sourceDir, "system", "bin"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, "system", "bin", "tool"), []byte("tool"), 0644))

	require.NoError(t, os.MkdirAll(filepath.Join(sourceDir, "system", "config", "app"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, "system", "config", "app", "settings.json"), []byte("settings"), 0644))

	require.NoError(t, os.MkdirAll(filepath.Join(sourceDir, "system", "ignore-me"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, "system", "ignore-me", "file.txt"), []byte("ignore"), 0644))

	cfg := &config.Config{
		Ignore: []string{
			"system/ignore-me", // Multi-level ignore
		},
		Packages: []*config.Package{
			{
				Source:      sourceDir,
				Targets:     []string{targetDir},
				DefaultFold: false,
				Fold: []string{
					"system/bin", // Multi-level fold
				},
				NoFold: []string{
					"system/config/app", // Multi-level no_fold
				},
			},
		},
	}

	err := cfg.Validate()
	require.NoError(t, err)

	lock := lockfile.New()
	linker := New(cfg, lock, false)

	result, err := linker.Link()
	require.NoError(t, err)

	// Check ignored directory doesn't exist
	_, err = os.Lstat(filepath.Join(targetDir, "system", "ignore-me"))
	assert.True(t, os.IsNotExist(err), "system/ignore-me should be ignored")

	// Check folded directory is a symlink
	info, err := os.Lstat(filepath.Join(targetDir, "system", "bin"))
	require.NoError(t, err)
	assert.True(t, info.Mode()&os.ModeSymlink != 0, "system/bin should be folded")

	// Check no_fold directory has individual files
	_, err = os.Lstat(filepath.Join(targetDir, "system", "config", "app", "settings.json"))
	assert.NoError(t, err, "system/config/app/settings.json should be individually linked")

	// Verify count (bin folded + settings.json individual)
	assert.Equal(t, 2, len(result.Created))
}
