package linker

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/user/farm/internal/config"
	"github.com/user/farm/internal/lockfile"
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
				"parent": true,
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
