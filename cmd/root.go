package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/javacruft/apk-regression-test-runner/internal"
	"github.com/spf13/cobra"
)

var (
	packageName string
	apkRepo     string
	repoPath    string
	repoType    string
	concurrency int
	verbose     bool
	hangTimeout time.Duration
)

var rootCmd = &cobra.Command{
	Use:   "apk-regression-test-runner",
	Short: "Test reverse dependencies of a package for regressions",
	Long: `A tool that uses apkrane to find reverse dependencies of a package
and melange to test each reverse dependency against a provided APK repository.
Tests are run with and without the APK repository to detect regressions.
Supports wolfi-dev/os, chainguard-dev/enterprise-packages, and chainguard-dev/extra-packages repositories.`,
	RunE: runRegressionTest,
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&packageName, "package", "p", "", "Package name to find reverse dependencies for (required)")
	rootCmd.PersistentFlags().StringVarP(&apkRepo, "repo", "r", "", "APK repository URL to test against (required)")
	rootCmd.PersistentFlags().StringVarP(&repoPath, "repo-path", "w", "", "Path to package repository (wolfi-dev/os, chainguard-dev/enterprise-packages, or chainguard-dev/extra-packages) (required)")
	rootCmd.PersistentFlags().StringVarP(&repoType, "repo-type", "t", "wolfi", "Repository type: wolfi, enterprise, or extras")
	rootCmd.PersistentFlags().IntVarP(&concurrency, "concurrency", "c", 4, "Number of concurrent test jobs")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")
	rootCmd.PersistentFlags().DurationVar(&hangTimeout, "hang-timeout", 10*time.Minute, "Timeout for hung tests (default: 10m)")

	rootCmd.MarkPersistentFlagRequired("package")
	rootCmd.MarkPersistentFlagRequired("repo")
	rootCmd.MarkPersistentFlagRequired("repo-path")
}

func runRegressionTest(cmd *cobra.Command, args []string) error {
	if !filepath.IsAbs(repoPath) {
		absPath, err := filepath.Abs(repoPath)
		if err != nil {
			return fmt.Errorf("failed to resolve repository path: %w", err)
		}
		repoPath = absPath
	}

	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		return fmt.Errorf("repository path does not exist: %s", repoPath)
	}

	// Validate repository type
	if repoType != "wolfi" && repoType != "enterprise" && repoType != "extras" {
		return fmt.Errorf("invalid repository type: %s (must be wolfi, enterprise, or extras)", repoType)
	}

	runner := internal.NewRegressionTestRunner(packageName, apkRepo, repoPath, repoType, concurrency, verbose, hangTimeout)
	return runner.Run()
}
