package main

import (
	"fmt"
	"os"

	"github.com/mskelton/farm/internal/config"
	"github.com/mskelton/farm/internal/linker"
	"github.com/mskelton/farm/internal/lockfile"
	"github.com/spf13/cobra"
)

var (
	configPath   string
	lockfilePath string
	dryRun       bool
	verbose      bool
)

var rootCmd = &cobra.Command{
	Use:   "farm",
	Short: "A dotfile manager with advanced symlink management",
	Long: `Farm is a dotfile manager that creates symlinks with features like:
- Lockfile tracking of created symlinks
- Support for symlinking to multiple targets
- Granular folding/no-folding control
- Automatic cleanup of dead symlinks`,
}

var linkCmd = &cobra.Command{
	Use:   "link",
	Short: "Create symlinks",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(configPath)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		lock, err := lockfile.Load(lockfilePath)
		if err != nil {
			return fmt.Errorf("failed to load lockfile: %w", err)
		}

		l := linker.New(cfg, lock, dryRun)
		result, err := l.Link()
		if err != nil {
			return fmt.Errorf("failed to link: %w", err)
		}

		if verbose || dryRun {
			printResult(cmd, result, dryRun)
		}

		if !dryRun {
			if err := lock.Save(lockfilePath); err != nil {
				return fmt.Errorf("failed to save lockfile: %w", err)
			}
			cmd.Printf("✓ Linked %d files, removed %d dead links\n", len(result.Created), len(result.Removed))
		}

		if len(result.Errors) > 0 {
			cmd.Println("\nErrors:")
			for _, err := range result.Errors {
				cmd.Printf("  ✗ %v\n", err)
			}
			return fmt.Errorf("linking completed with %d errors", len(result.Errors))
		}

		return nil
	},
}

var unlinkCmd = &cobra.Command{
	Use:   "unlink",
	Short: "Remove symlinks",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(configPath)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		lock, err := lockfile.Load(lockfilePath)
		if err != nil {
			return fmt.Errorf("failed to load lockfile: %w", err)
		}

		l := linker.New(cfg, lock, dryRun)
		result, err := l.Unlink()
		if err != nil {
			return fmt.Errorf("failed to unlink: %w", err)
		}

		if verbose || dryRun {
			if dryRun {
				cmd.Printf("Will remove symlinks:\n")
			} else {
				cmd.Printf("Removed symlinks:\n")
			}
			for _, removed := range result.Removed {
				cmd.Printf("  - %s\n", removed)
			}
		}

		if !dryRun {
			if err := lock.Save(lockfilePath); err != nil {
				return fmt.Errorf("failed to save lockfile: %w", err)
			}
			cmd.Printf("✓ Removed %d symlinks\n", len(result.Removed))
		}

		if len(result.Errors) > 0 {
			cmd.Println("\nErrors:")
			for _, err := range result.Errors {
				cmd.Printf("  ✗ %v\n", err)
			}
			return fmt.Errorf("unlinking completed with %d errors", len(result.Errors))
		}

		return nil
	},
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show status of symlinks",
	RunE: func(cmd *cobra.Command, args []string) error {
		lock, err := lockfile.Load(lockfilePath)
		if err != nil {
			return fmt.Errorf("failed to load lockfile: %w", err)
		}

		if len(lock.Symlinks) == 0 {
			cmd.Println("No symlinks tracked")
			return nil
		}

		if verbose {
			cmd.Printf("Tracking %d symlinks:\n\n", len(lock.Symlinks))

			for _, link := range lock.Symlinks.Sorted() {
				cmd.Printf("  %s -> %s", link.Target, link.Source)
				if link.IsFolded {
					cmd.Print(" [folded]")
				}
				cmd.Println()
			}
		} else {
			cmd.Printf("Tracking %d symlinks\n", len(lock.Symlinks))
		}

		deadLinks, err := lock.GetDeadSymlinks()
		if err != nil {
			return fmt.Errorf("failed to check for dead symlinks: %w", err)
		}

		if len(deadLinks) > 0 {
			cmd.Printf("\n⚠ Found %d dead symlinks:\n", len(deadLinks))
			for _, dead := range deadLinks {
				cmd.Printf("  ✗ %s\n", dead)
			}
			cmd.Println("\nRun 'farm link' to clean up dead symlinks")
		}

		return nil
	},
}

func printResult(cmd *cobra.Command, result *linker.LinkResult, isDryRun bool) {
	if len(result.Created) > 0 {
		if isDryRun {
			cmd.Println("Will create symlinks:")
		} else {
			cmd.Println("Created symlinks:")
		}
		for _, created := range result.Created {
			cmd.Printf("  + %s\n", created)
		}
	}

	if len(result.Removed) > 0 {
		if isDryRun {
			cmd.Println("\nWill remove dead symlinks:")
		} else {
			cmd.Println("\nRemoved dead symlinks:")
		}
		for _, removed := range result.Removed {
			cmd.Printf("  - %s\n", removed)
		}
	}
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "farm.yaml", "config file path")
	rootCmd.PersistentFlags().StringVarP(&lockfilePath, "lockfile", "l", "farm.lock", "lockfile path")
	rootCmd.PersistentFlags().BoolVarP(&dryRun, "dry-run", "n", false, "perform a dry run")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")

	rootCmd.AddCommand(linkCmd)
	rootCmd.AddCommand(unlinkCmd)
	rootCmd.AddCommand(statusCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
