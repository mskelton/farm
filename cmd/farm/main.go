package main

import (
	"fmt"
	"os"
	"strings"

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
	environment  string
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
	Use:   "link [environment]",
	Short: "Create symlinks",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Get environment from args if provided
		if len(args) > 0 {
			environment = args[0]
		}

		cfg, err := config.Load(configPath)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		// Filter packages for the specified environment
		packages := cfg.GetPackagesForEnvironment(environment)
		if len(packages) == 0 {
			if environment != "" {
				cmd.Printf("No packages found for environment '%s'\n", environment)
				available := cfg.GetAvailableEnvironments()
				if len(available) > 0 {
					cmd.Printf("Available environments: %v\n", available)
				}
				return nil
			}
		}

		// Create a temporary config with filtered packages
		filteredConfig := &config.Config{
			Packages:    packages,
			Ignore:      cfg.Ignore,
			IgnoreGlobs: cfg.IgnoreGlobs,
		}

		lock, err := lockfile.Load(lockfilePath)
		if err != nil {
			return fmt.Errorf("failed to load lockfile: %w", err)
		}

		l := linker.New(filteredConfig, lock, dryRun)
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
			envMsg := ""
			if environment != "" {
				envMsg = fmt.Sprintf(" for environment '%s'", environment)
			}
			cmd.Printf("✓ Linked %d files, removed %d dead links%s\n", len(result.Created), len(result.Removed), envMsg)
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
	Use:   "unlink [environment]",
	Short: "Remove symlinks",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Get environment from args if provided
		if len(args) > 0 {
			environment = args[0]
		}

		cfg, err := config.Load(configPath)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		// Filter packages for the specified environment
		packages := cfg.GetPackagesForEnvironment(environment)
		if len(packages) == 0 {
			if environment != "" {
				cmd.Printf("No packages found for environment '%s'\n", environment)
				available := cfg.GetAvailableEnvironments()
				if len(available) > 0 {
					cmd.Printf("Available environments: %v\n", available)
				}
				return nil
			}
		}

		// Create a temporary config with filtered packages
		filteredConfig := &config.Config{
			Packages:    packages,
			Ignore:      cfg.Ignore,
			IgnoreGlobs: cfg.IgnoreGlobs,
		}

		lock, err := lockfile.Load(lockfilePath)
		if err != nil {
			return fmt.Errorf("failed to load lockfile: %w", err)
		}

		l := linker.New(filteredConfig, lock, dryRun)
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
			envMsg := ""
			if environment != "" {
				envMsg = fmt.Sprintf(" for environment '%s'", environment)
			}
			cmd.Printf("✓ Removed %d symlinks%s\n", len(result.Removed), envMsg)
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
	Use:   "status [environment]",
	Short: "Show status of symlinks",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Get environment from args if provided
		if len(args) > 0 {
			environment = args[0]
		}

		lock, err := lockfile.Load(lockfilePath)
		if err != nil {
			return fmt.Errorf("failed to load lockfile: %w", err)
		}

		if len(lock.Symlinks) == 0 {
			cmd.Println("No symlinks tracked")
			return nil
		}

		// If environment is specified, filter symlinks based on config
		var relevantSymlinks []lockfile.Symlink
		if environment != "" {
			cfg, err := config.Load(configPath)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			packages := cfg.GetPackagesForEnvironment(environment)
			if len(packages) == 0 {
				cmd.Printf("No packages found for environment '%s'\n", environment)
				available := cfg.GetAvailableEnvironments()
				if len(available) > 0 {
					cmd.Printf("Available environments: %v\n", available)
				}
				return nil
			}

			// Get all source paths for the environment
			sourcePaths := make(map[string]bool)
			for _, pkg := range packages {
				sourcePaths[pkg.Source] = true
			}

			// Filter symlinks that belong to this environment
			for _, link := range lock.Symlinks.Sorted() {
				for sourcePath := range sourcePaths {
					if strings.HasPrefix(link.Source, sourcePath) {
						relevantSymlinks = append(relevantSymlinks, link)
						break
					}
				}
			}
		} else {
			relevantSymlinks = lock.Symlinks.Sorted()
		}

		if len(relevantSymlinks) == 0 {
			envMsg := ""
			if environment != "" {
				envMsg = fmt.Sprintf(" for environment '%s'", environment)
			}
			cmd.Printf("No symlinks tracked%s\n", envMsg)
			return nil
		}

		if verbose {
			envMsg := ""
			if environment != "" {
				envMsg = fmt.Sprintf(" for environment '%s'", environment)
			}
			cmd.Printf("Tracking %d symlinks%s:\n\n", len(relevantSymlinks), envMsg)

			for _, link := range relevantSymlinks {
				cmd.Printf("  %s -> %s", link.Target, link.Source)
				if link.IsFolded {
					cmd.Print(" [folded]")
				}
				cmd.Println()
			}
		} else {
			envMsg := ""
			if environment != "" {
				envMsg = fmt.Sprintf(" for environment '%s'", environment)
			}
			cmd.Printf("Tracking %d symlinks%s\n", len(relevantSymlinks), envMsg)
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
			envMsg := ""
			if environment != "" {
				envMsg = fmt.Sprintf(" %s", environment)
			}
			cmd.Printf("\nRun 'farm link%s' to clean up dead symlinks\n", envMsg)
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
