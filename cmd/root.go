package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/chainguard-dev/apk-regression-test-runner/internal"
)

var (
	packageName    string
	apkRepo        string
	wolfiOSPath    string
	concurrency    int
	verbose        bool
)

var rootCmd = &cobra.Command{
	Use:   "apk-regression-test-runner",
	Short: "Test reverse dependencies of a package for regressions",
	Long: `A tool that uses apkrane to find reverse dependencies of a package
and melange to test each reverse dependency against a provided APK repository.
Tests are run with and without the APK repository to detect regressions.`,
	RunE: runRegressionTest,
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&packageName, "package", "p", "", "Package name to find reverse dependencies for (required)")
	rootCmd.PersistentFlags().StringVarP(&apkRepo, "repo", "r", "", "APK repository URL to test against (required)")
	rootCmd.PersistentFlags().StringVarP(&wolfiOSPath, "wolfi-os", "w", "", "Path to wolfi-dev/os repository (required)")
	rootCmd.PersistentFlags().IntVarP(&concurrency, "concurrency", "c", 4, "Number of concurrent test jobs")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")

	rootCmd.MarkPersistentFlagRequired("package")
	rootCmd.MarkPersistentFlagRequired("repo")
	rootCmd.MarkPersistentFlagRequired("wolfi-os")
}

func runRegressionTest(cmd *cobra.Command, args []string) error {
	if !filepath.IsAbs(wolfiOSPath) {
		absPath, err := filepath.Abs(wolfiOSPath)
		if err != nil {
			return fmt.Errorf("failed to resolve wolfi-os path: %w", err)
		}
		wolfiOSPath = absPath
	}

	if _, err := os.Stat(wolfiOSPath); os.IsNotExist(err) {
		return fmt.Errorf("wolfi-os path does not exist: %s", wolfiOSPath)
	}

	runner := internal.NewRegressionTestRunner(packageName, apkRepo, wolfiOSPath, concurrency, verbose)
	return runner.Run()
}