// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright 2025 Chainguard, Inc.

package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/javacruft/apkregress/internal"
	"github.com/spf13/cobra"
)

var (
	packageName    string
	packageFile    string
	apkRepo        string
	repoPath       string
	repoType       string
	concurrency    int
	verbose        bool
	hangTimeout    time.Duration
	markdownOutput bool
)

var rootCmd = &cobra.Command{
	Use:   "apkregress",
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
	rootCmd.PersistentFlags().StringVarP(&packageName, "package", "p", "", "Package name to find reverse dependencies for")
	rootCmd.PersistentFlags().StringVarP(&packageFile, "package-file", "f", "", "File containing list of package names (one per line)")
	rootCmd.PersistentFlags().StringVarP(&apkRepo, "repo", "r", "", "APK repository URL to test against (required)")
	rootCmd.PersistentFlags().StringVarP(&repoPath, "repo-path", "w", "", "Path to package repository (wolfi-dev/os, chainguard-dev/enterprise-packages, or chainguard-dev/extra-packages) (required)")
	rootCmd.PersistentFlags().StringVarP(&repoType, "repo-type", "t", "wolfi", "Repository type: wolfi, enterprise, or extras")
	rootCmd.PersistentFlags().IntVarP(&concurrency, "concurrency", "c", 4, "Number of concurrent test jobs")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")
	rootCmd.PersistentFlags().DurationVar(&hangTimeout, "hang-timeout", 30*time.Minute, "Timeout for hung tests (default: 30m)")
	rootCmd.PersistentFlags().BoolVarP(&markdownOutput, "markdown", "m", false, "Output test summary in markdown format for GitHub issues")

	rootCmd.MarkPersistentFlagRequired("repo")
	rootCmd.MarkPersistentFlagRequired("repo-path")
}

func runRegressionTest(cmd *cobra.Command, args []string) error {
	// Validate that either package or package-file is provided, but not both
	if packageName == "" && packageFile == "" {
		return fmt.Errorf("either --package or --package-file must be specified")
	}
	if packageName != "" && packageFile != "" {
		return fmt.Errorf("cannot specify both --package and --package-file")
	}

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

	if packageFile != "" {
		// Package file mode: test packages directly from file
		packages, err := readPackageFile(packageFile)
		if err != nil {
			return fmt.Errorf("failed to read package file: %w", err)
		}
		runner := internal.NewRegressionTestRunnerFromPackageList(packages, apkRepo, repoPath, repoType, concurrency, verbose, hangTimeout, markdownOutput)
		return runner.RunFromPackageList(packages)
	} else {
		// Single package mode: find reverse dependencies and test them
		runner := internal.NewRegressionTestRunner(packageName, apkRepo, repoPath, repoType, concurrency, verbose, hangTimeout, markdownOutput)
		return runner.Run()
	}
}

func readPackageFile(filename string) ([]string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var packages []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && !strings.HasPrefix(line, "#") {
			packages = append(packages, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	if len(packages) == 0 {
		return nil, fmt.Errorf("no packages found in file %s", filename)
	}

	return packages, nil
}
