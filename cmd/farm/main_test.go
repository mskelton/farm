package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCLIIntegration(t *testing.T) {
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)

	require.NoError(t, os.Chdir(tmpDir))

	// Reset flags to defaults
	configPath = "farm.yaml"
	lockfilePath = "farm.lock"
	dryRun = false
	verbose = false

	dotfilesDir := filepath.Join(tmpDir, "dotfiles")
	vimDir := filepath.Join(dotfilesDir, "vim")
	vscodeDir := filepath.Join(dotfilesDir, "vscode")
	require.NoError(t, os.MkdirAll(vimDir, 0755))
	require.NoError(t, os.MkdirAll(vscodeDir, 0755))

	require.NoError(t, os.WriteFile(filepath.Join(vimDir, ".vimrc"), []byte("vim config"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(vscodeDir, "settings.json"), []byte(`{"editor": {}}`), 0644))

	configContent := `packages:
  vim:
    source: ./dotfiles/vim
    targets:
      - ./home/.vim
  vscode:
    source: ./dotfiles/vscode
    targets:
      - ./home/.config/Code/User
      - ./home/.config/Cursor/User
`
	require.NoError(t, os.WriteFile("farm.yaml", []byte(configContent), 0644))

	t.Run("status with no links", func(t *testing.T) {
		rootCmd.SetArgs([]string{"status"})
		err := rootCmd.Execute()
		assert.NoError(t, err)
	})

	t.Run("link all packages", func(t *testing.T) {
		rootCmd.SetArgs([]string{"link", "-v"})
		err := rootCmd.Execute()
		assert.NoError(t, err)

		assert.FileExists(t, "./home/.vim/.vimrc")
		assert.FileExists(t, "./home/.config/Code/User/settings.json")
		assert.FileExists(t, "./home/.config/Cursor/User/settings.json")

		content, _ := os.ReadFile("./home/.vim/.vimrc")
		assert.Equal(t, "vim config", string(content))
	})

	t.Run("status after linking", func(t *testing.T) {
		rootCmd.SetArgs([]string{"status", "-v"})
		err := rootCmd.Execute()
		assert.NoError(t, err)
	})

	t.Run("unlink package", func(t *testing.T) {
		rootCmd.SetArgs([]string{"unlink", "vim"})
		err := rootCmd.Execute()
		assert.NoError(t, err)

		_, err = os.Stat("./home/.vim/.vimrc")
		assert.True(t, os.IsNotExist(err))
	})

	t.Run("dry run", func(t *testing.T) {
		os.Remove("./home/.config/Code/User/settings.json")

		rootCmd.SetArgs([]string{"link", "-n", "-v"})
		err := rootCmd.Execute()
		assert.NoError(t, err)

		_, err = os.Stat("./home/.config/Code/User/settings.json")
		assert.True(t, os.IsNotExist(err))
	})
}

func TestCLIFoldingBehavior(t *testing.T) {
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)

	require.NoError(t, os.Chdir(tmpDir))

	// Reset flags to defaults
	configPath = "farm.yaml"
	lockfilePath = "farm.lock"
	dryRun = false
	verbose = false

	sourceDir := filepath.Join(tmpDir, "source")
	foldDir := filepath.Join(sourceDir, "fold-me")
	noFoldDir := filepath.Join(sourceDir, "no-fold")
	require.NoError(t, os.MkdirAll(foldDir, 0755))
	require.NoError(t, os.MkdirAll(noFoldDir, 0755))

	require.NoError(t, os.WriteFile(filepath.Join(foldDir, "file1.txt"), []byte("fold"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(noFoldDir, "file2.txt"), []byte("no fold"), 0644))

	configContent := `packages:
  test:
    source: ./source
    targets:
      - ./target
    default_fold: false
    fold:
      - fold-me
`
	require.NoError(t, os.WriteFile("farm.yaml", []byte(configContent), 0644))

	rootCmd.SetArgs([]string{"link"})
	err := rootCmd.Execute()
	assert.NoError(t, err)

	info, err := os.Lstat("./target/fold-me")
	require.NoError(t, err)
	assert.True(t, info.Mode()&os.ModeSymlink != 0)

	info, err = os.Lstat("./target/no-fold/file2.txt")
	require.NoError(t, err)
	assert.True(t, info.Mode()&os.ModeSymlink != 0)
}

func TestCLIDeadLinkCleanup(t *testing.T) {
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)

	require.NoError(t, os.Chdir(tmpDir))

	// Reset flags to defaults
	configPath = "farm.yaml"
	lockfilePath = "farm.lock"
	dryRun = false
	verbose = false

	sourceDir := filepath.Join(tmpDir, "source")
	require.NoError(t, os.MkdirAll(sourceDir, 0755))

	deadFile := filepath.Join(sourceDir, "dead.txt")
	require.NoError(t, os.WriteFile(deadFile, []byte("will be deleted"), 0644))

	configContent := `packages:
  test:
    source: ./source
    targets:
      - ./target
`
	require.NoError(t, os.WriteFile("farm.yaml", []byte(configContent), 0644))

	rootCmd.SetArgs([]string{"link"})
	err := rootCmd.Execute()
	assert.NoError(t, err)

	require.NoError(t, os.Remove(deadFile))

	// Ensure lockfile exists
	_, err = os.Stat("farm.lock")
	require.NoError(t, err)

	rootCmd.SetArgs([]string{"status"})
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	err = rootCmd.Execute()
	assert.NoError(t, err)
	output := buf.String()
	t.Logf("Status output: %s", output)
	assert.Contains(t, output, "dead")

	rootCmd.SetArgs([]string{"link"})
	err = rootCmd.Execute()
	assert.NoError(t, err)

	_, err = os.Lstat("./target/dead.txt")
	assert.True(t, os.IsNotExist(err))
}
