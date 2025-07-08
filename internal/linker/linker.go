package linker

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/user/farm/internal/config"
	"github.com/user/farm/internal/lockfile"
)

type Linker struct {
	config   *config.Config
	lockFile *lockfile.LockFile
	dryRun   bool
}

type LinkResult struct {
	Created []string
	Removed []string
	Errors  []error
}

func New(cfg *config.Config, lock *lockfile.LockFile, dryRun bool) *Linker {
	return &Linker{
		config:   cfg,
		lockFile: lock,
		dryRun:   dryRun,
	}
}

func (l *Linker) Link() (*LinkResult, error) {
	result := &LinkResult{
		Created: []string{},
		Removed: []string{},
		Errors:  []error{},
	}

	deadLinks, err := l.lockFile.GetDeadSymlinks()
	if err != nil {
		return nil, fmt.Errorf("failed to get dead symlinks: %w", err)
	}

	for _, dead := range deadLinks {
		if !l.dryRun {
			if err := os.Remove(dead); err != nil && !os.IsNotExist(err) {
				result.Errors = append(result.Errors, fmt.Errorf("failed to remove dead link %s: %w", dead, err))
				continue
			}
		}
		l.lockFile.RemoveSymlink(dead)
		result.Removed = append(result.Removed, dead)
	}

	for _, pkg := range l.config.Packages {
		for _, target := range pkg.Targets {
			if err := l.linkPackage(pkg, target, result); err != nil {
				result.Errors = append(result.Errors, err)
			}
		}
	}

	return result, nil
}

func (l *Linker) linkPackage(pkg *config.Package, targetBase string, result *LinkResult) error {
	return l.linkDirectory(pkg.Source, targetBase, pkg, result)
}

func (l *Linker) linkDirectory(source, target string, pkg *config.Package, result *LinkResult) error {
	entries, err := os.ReadDir(source)
	if err != nil {
		return fmt.Errorf("failed to read source directory %s: %w", source, err)
	}

	for _, entry := range entries {
		// Skip ignored files/directories
		if l.config.ShouldIgnore(entry.Name()) {
			continue
		}

		sourcePath := filepath.Join(source, entry.Name())
		targetPath := filepath.Join(target, entry.Name())

		if entry.IsDir() {
			if l.shouldFold(entry.Name(), source, pkg) {
				if err := l.createSymlink(sourcePath, targetPath, true, result); err != nil {
					return err
				}
			} else {
				if err := l.linkDirectory(sourcePath, targetPath, pkg, result); err != nil {
					return err
				}
			}
		} else {
			if err := l.createSymlink(sourcePath, targetPath, false, result); err != nil {
				return err
			}
		}
	}

	return nil
}

func (l *Linker) shouldFold(dirName, currentPath string, pkg *config.Package) bool {
	relativePath := strings.TrimPrefix(currentPath, pkg.Source)
	relativePath = strings.TrimPrefix(relativePath, "/")
	if relativePath != "" {
		relativePath = filepath.Join(relativePath, dirName)
	} else {
		relativePath = dirName
	}

	for _, noFoldPath := range pkg.NoFold {
		if matched, _ := filepath.Match(noFoldPath, relativePath); matched {
			return false
		}
		if strings.HasPrefix(relativePath, noFoldPath+"/") {
			return false
		}
	}

	for _, foldPath := range pkg.Fold {
		if matched, _ := filepath.Match(foldPath, relativePath); matched {
			return true
		}
		if strings.HasPrefix(relativePath, foldPath+"/") {
			return true
		}
	}

	return pkg.DefaultFold
}

func (l *Linker) createSymlink(source, target string, isFolded bool, result *LinkResult) error {
	targetDir := filepath.Dir(target)
	if !l.dryRun {
		if err := os.MkdirAll(targetDir, 0755); err != nil {
			return fmt.Errorf("failed to create target directory %s: %w", targetDir, err)
		}
	}

	if existingTarget, err := os.Lstat(target); err == nil {
		if existingTarget.Mode()&os.ModeSymlink != 0 {
			existingSource, _ := os.Readlink(target)
			existingSourceAbs := existingSource
			if !filepath.IsAbs(existingSource) {
				existingSourceAbs = filepath.Join(filepath.Dir(target), existingSource)
			}

			if existingSourceAbs == source {
				// Symlink already exists and points to correct source
				// Add it to lockfile if not already tracked
				l.lockFile.AddSymlink(target, source, isFolded)
				return nil
			}

			if !l.dryRun {
				if err := os.Remove(target); err != nil {
					return fmt.Errorf("failed to remove existing symlink %s: %w", target, err)
				}
			}
		} else {
			return fmt.Errorf("target %s already exists and is not a symlink", target)
		}
	}

	if !l.dryRun {
		relSource, err := filepath.Rel(filepath.Dir(target), source)
		if err != nil {
			return fmt.Errorf("failed to calculate relative path: %w", err)
		}

		if err := os.Symlink(relSource, target); err != nil {
			return fmt.Errorf("failed to create symlink %s -> %s: %w", target, source, err)
		}
	}

	l.lockFile.AddSymlink(target, source, isFolded)
	result.Created = append(result.Created, target)

	return nil
}

func (l *Linker) Unlink() (*LinkResult, error) {
	result := &LinkResult{
		Removed: []string{},
		Errors:  []error{},
	}

	for target := range l.lockFile.Symlinks {
		if !l.dryRun {
			if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
				result.Errors = append(result.Errors, fmt.Errorf("failed to remove symlink %s: %w", target, err))
				continue
			}
		}

		l.lockFile.RemoveSymlink(target)
		result.Removed = append(result.Removed, target)
	}

	return result, nil
}
