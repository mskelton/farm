package lockfile

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

type SymlinkMap map[string]Symlink

func (m SymlinkMap) Sorted() []Symlink {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	symlinks := make([]Symlink, 0, len(keys))
	for _, key := range keys {
		symlinks = append(symlinks, m[key])
	}

	return symlinks
}

type LockFile struct {
	Version  string     `json:"version"`
	Updated  time.Time  `json:"updated"`
	Symlinks SymlinkMap `json:"symlinks"`
}

type Symlink struct {
	Source   string    `json:"source"`
	Target   string    `json:"target"`
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
		lock.Symlinks = make(SymlinkMap)
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

func (l *LockFile) AddSymlink(target string, source string, isFolded bool) {
	l.Symlinks[target] = Symlink{
		Source:   source,
		Target:   target,
		Created:  time.Now(),
		IsFolded: isFolded,
	}
}

func (l *LockFile) RemoveSymlink(target string) {
	delete(l.Symlinks, target)
}

func (l *LockFile) GetDeadSymlinks() ([]string, error) {
	var dead []string

	for _, link := range l.Symlinks.Sorted() {
		targetInfo, err := os.Lstat(link.Target)
		if err != nil {
			if os.IsNotExist(err) {
				dead = append(dead, link.Target)
				continue
			}
			return nil, fmt.Errorf("failed to stat %s: %w", link.Target, err)
		}

		if targetInfo.Mode()&os.ModeSymlink == 0 {
			continue
		}

		linkDest, err := os.Readlink(link.Target)
		if err != nil {
			dead = append(dead, link.Target)
			continue
		}

		linkDestAbs := linkDest
		if !filepath.IsAbs(linkDest) {
			linkDestAbs = filepath.Join(filepath.Dir(link.Target), linkDest)
		}

		if _, err := os.Stat(linkDestAbs); os.IsNotExist(err) {
			dead = append(dead, link.Target)
		} else if linkDestAbs != link.Source {
			dead = append(dead, link.Target)
		}
	}

	return dead, nil
}
