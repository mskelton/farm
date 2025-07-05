package lockfile

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type LockFile struct {
	Version  string             `json:"version"`
	Updated  time.Time          `json:"updated"`
	Symlinks map[string]Symlink `json:"symlinks"`
}

type Symlink struct {
	Source   string    `json:"source"`
	Target   string    `json:"target"`
	Package  string    `json:"package"`
	Created  time.Time `json:"created"`
	IsFolded bool      `json:"is_folded"`
}

const (
	CurrentVersion = "1.0"
	DefaultPath    = "farm.lock"
)

func New() *LockFile {
	return &LockFile{
		Version:  CurrentVersion,
		Updated:  time.Now(),
		Symlinks: make(map[string]Symlink),
	}
}

func Load(path string) (*LockFile, error) {
	if path == "" {
		path = DefaultPath
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return New(), nil
		}
		return nil, fmt.Errorf("failed to read lockfile: %w", err)
	}

	var lock LockFile
	if err := json.Unmarshal(data, &lock); err != nil {
		return nil, fmt.Errorf("failed to parse lockfile: %w", err)
	}

	if lock.Version != CurrentVersion {
		return nil, fmt.Errorf("unsupported lockfile version: %s", lock.Version)
	}

	if lock.Symlinks == nil {
		lock.Symlinks = make(map[string]Symlink)
	}

	return &lock, nil
}

func (l *LockFile) Save(path string) error {
	if path == "" {
		path = DefaultPath
	}

	l.Updated = time.Now()

	data, err := json.MarshalIndent(l, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal lockfile: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write lockfile: %w", err)
	}

	return nil
}

func (l *LockFile) AddSymlink(target, source, packageName string, isFolded bool) {
	l.Symlinks[target] = Symlink{
		Source:   source,
		Target:   target,
		Package:  packageName,
		Created:  time.Now(),
		IsFolded: isFolded,
	}
}

func (l *LockFile) RemoveSymlink(target string) {
	delete(l.Symlinks, target)
}

func (l *LockFile) GetDeadSymlinks() ([]string, error) {
	var dead []string

	for target, link := range l.Symlinks {
		targetInfo, err := os.Lstat(target)
		if err != nil {
			if os.IsNotExist(err) {
				dead = append(dead, target)
				continue
			}
			return nil, fmt.Errorf("failed to stat %s: %w", target, err)
		}

		if targetInfo.Mode()&os.ModeSymlink == 0 {
			continue
		}

		linkDest, err := os.Readlink(target)
		if err != nil {
			dead = append(dead, target)
			continue
		}

		linkDestAbs := linkDest
		if !filepath.IsAbs(linkDest) {
			linkDestAbs = filepath.Join(filepath.Dir(target), linkDest)
		}

		if _, err := os.Stat(linkDestAbs); os.IsNotExist(err) {
			dead = append(dead, target)
		} else if linkDestAbs != link.Source {
			dead = append(dead, target)
		}
	}

	return dead, nil
}

func (l *LockFile) GetSymlinksForPackage(packageName string) []Symlink {
	var symlinks []Symlink
	for _, link := range l.Symlinks {
		if link.Package == packageName {
			symlinks = append(symlinks, link)
		}
	}
	return symlinks
}
