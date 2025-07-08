package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/user/farm/internal/config"
	"github.com/user/farm/internal/linker"
	"github.com/user/farm/internal/lockfile"
)

func main() {
	// Create temporary directories for testing
	tempDir, err := os.MkdirTemp("", "farm-test-*")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	sourceDir := filepath.Join(tempDir, "source")
	targetDir := filepath.Join(tempDir, "target")

	// Create source directory and file
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		log.Fatal(err)
	}
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		log.Fatal(err)
	}

	sourceFile := filepath.Join(sourceDir, "test.txt")
	if err := os.WriteFile(sourceFile, []byte("test content"), 0644); err != nil {
		log.Fatal(err)
	}

	// Create a symlink manually (simulating existing symlink)
	targetFile := filepath.Join(targetDir, "test.txt")
	relSource, _ := filepath.Rel(filepath.Dir(targetFile), sourceFile)
	if err := os.Symlink(relSource, targetFile); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Created existing symlink: %s -> %s\n", targetFile, sourceFile)

	// Create config
	cfg := &config.Config{
		Packages: []*config.Package{
			{
				Source:  sourceDir,
				Targets: []string{targetDir},
			},
		},
	}

	if err := cfg.Validate(); err != nil {
		log.Fatal(err)
	}

	// Create empty lockfile
	lock := lockfile.New()

	// Run linker
	l := linker.New(cfg, lock, false)
	result, err := l.Link()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Link result - Created: %d, Removed: %d, Errors: %d\n", 
		len(result.Created), len(result.Removed), len(result.Errors))

	// Check if symlink is now in lockfile
	if len(lock.Symlinks) > 0 {
		fmt.Printf("✓ Symlink added to lockfile: %v\n", lock.Symlinks)
	} else {
		fmt.Printf("✗ No symlinks in lockfile\n")
	}
}