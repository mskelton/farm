package lockfile

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLockFile(t *testing.T) {
	lock := New()
	assert.Equal(t, CurrentVersion, lock.Version)
	assert.NotNil(t, lock.Symlinks)
	assert.Empty(t, lock.Symlinks)
	assert.WithinDuration(t, time.Now(), lock.Updated, 1*time.Second)
}

func TestSaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	lockPath := filepath.Join(tmpDir, "test.lock")

	original := New()
	original.AddSymlink("/home/user/.vimrc", "/home/user/dotfiles/vim/.vimrc", "vim", false)
	original.AddSymlink("/home/user/.config/nvim", "/home/user/dotfiles/nvim", "nvim", true)

	err := original.Save(lockPath)
	require.NoError(t, err)

	loaded, err := Load(lockPath)
	require.NoError(t, err)

	assert.Equal(t, original.Version, loaded.Version)
	assert.Len(t, loaded.Symlinks, 2)

	vimLink := loaded.Symlinks["/home/user/.vimrc"]
	assert.Equal(t, "/home/user/dotfiles/vim/.vimrc", vimLink.Source)
	assert.Equal(t, "vim", vimLink.Package)
	assert.False(t, vimLink.IsFolded)

	nvimLink := loaded.Symlinks["/home/user/.config/nvim"]
	assert.Equal(t, "/home/user/dotfiles/nvim", nvimLink.Source)
	assert.Equal(t, "nvim", nvimLink.Package)
	assert.True(t, nvimLink.IsFolded)
}

func TestLoadNonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	lockPath := filepath.Join(tmpDir, "nonexistent.lock")

	lock, err := Load(lockPath)
	require.NoError(t, err)
	assert.Equal(t, CurrentVersion, lock.Version)
	assert.Empty(t, lock.Symlinks)
}

func TestLoadInvalidJSON(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "invalid-*.lock")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString("invalid json")
	require.NoError(t, err)
	tmpFile.Close()

	_, err = Load(tmpFile.Name())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse lockfile")
}

func TestLoadWrongVersion(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "wrong-version-*.lock")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(`{"version": "0.1", "symlinks": {}}`)
	require.NoError(t, err)
	tmpFile.Close()

	_, err = Load(tmpFile.Name())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported lockfile version")
}

func TestAddRemoveSymlink(t *testing.T) {
	lock := New()

	lock.AddSymlink("/home/user/.vimrc", "/home/user/dotfiles/vim/.vimrc", "vim", false)
	assert.Len(t, lock.Symlinks, 1)

	link := lock.Symlinks["/home/user/.vimrc"]
	assert.Equal(t, "/home/user/dotfiles/vim/.vimrc", link.Source)
	assert.Equal(t, "vim", link.Package)
	assert.False(t, link.IsFolded)

	lock.RemoveSymlink("/home/user/.vimrc")
	assert.Empty(t, lock.Symlinks)
}

func TestGetDeadSymlinks(t *testing.T) {
	tmpDir := t.TempDir()

	sourceFile := filepath.Join(tmpDir, "source.txt")
	err := os.WriteFile(sourceFile, []byte("test"), 0644)
	require.NoError(t, err)

	goodLink := filepath.Join(tmpDir, "good-link")
	err = os.Symlink(sourceFile, goodLink)
	require.NoError(t, err)

	deadSourceFile := filepath.Join(tmpDir, "dead-source.txt")
	err = os.WriteFile(deadSourceFile, []byte("test"), 0644)
	require.NoError(t, err)

	deadLink := filepath.Join(tmpDir, "dead-link")
	err = os.Symlink(deadSourceFile, deadLink)
	require.NoError(t, err)

	err = os.Remove(deadSourceFile)
	require.NoError(t, err)

	nonExistentLink := filepath.Join(tmpDir, "non-existent")

	lock := New()
	lock.AddSymlink(goodLink, sourceFile, "test", false)
	lock.AddSymlink(deadLink, deadSourceFile, "test", false)
	lock.AddSymlink(nonExistentLink, sourceFile, "test", false)

	dead, err := lock.GetDeadSymlinks()
	require.NoError(t, err)

	assert.Contains(t, dead, deadLink)
	assert.Contains(t, dead, nonExistentLink)
	assert.NotContains(t, dead, goodLink)
}

func TestGetSymlinksForPackage(t *testing.T) {
	lock := New()

	lock.AddSymlink("/home/user/.vimrc", "/home/user/dotfiles/vim/.vimrc", "vim", false)
	lock.AddSymlink("/home/user/.vim", "/home/user/dotfiles/vim", "vim", true)
	lock.AddSymlink("/home/user/.config/nvim", "/home/user/dotfiles/nvim", "nvim", true)

	vimLinks := lock.GetSymlinksForPackage("vim")
	assert.Len(t, vimLinks, 2)

	nvimLinks := lock.GetSymlinksForPackage("nvim")
	assert.Len(t, nvimLinks, 1)

	noLinks := lock.GetSymlinksForPackage("nonexistent")
	assert.Empty(t, noLinks)
}
