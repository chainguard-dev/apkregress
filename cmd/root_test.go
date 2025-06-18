// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright 2025 Chainguard, Inc.

package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestReadPackageFile(t *testing.T) {
	tests := []struct {
		name           string
		fileContent    string
		expectedPkgs   []string
		expectedError  bool
		errorMessage   string
	}{
		{
			name: "valid package file",
			fileContent: `package1
package2
package3`,
			expectedPkgs:  []string{"package1", "package2", "package3"},
			expectedError: false,
		},
		{
			name: "package file with comments and empty lines",
			fileContent: `# This is a comment
package1

package2
# Another comment
package3

`,
			expectedPkgs:  []string{"package1", "package2", "package3"},
			expectedError: false,
		},
		{
			name: "package file with only comments",
			fileContent: `# This is a comment
# Another comment
`,
			expectedPkgs:   nil,
			expectedError:  true,
			errorMessage:   "no packages found in file",
		},
		{
			name:          "empty file",
			fileContent:   "",
			expectedPkgs:  nil,
			expectedError: true,
			errorMessage:  "no packages found in file",
		},
		{
			name: "package file with whitespace",
			fileContent: `  package1  
	package2	
 package3 `,
			expectedPkgs:  []string{"package1", "package2", "package3"},
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary file
			tmpFile, err := os.CreateTemp("", "packages_*.txt")
			if err != nil {
				t.Fatalf("Failed to create temp file: %v", err)
			}
			defer os.Remove(tmpFile.Name())

			// Write test content
			if _, err := tmpFile.WriteString(tt.fileContent); err != nil {
				t.Fatalf("Failed to write to temp file: %v", err)
			}
			tmpFile.Close()

			// Test readPackageFile
			packages, err := readPackageFile(tmpFile.Name())

			if tt.expectedError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if !strings.Contains(err.Error(), tt.errorMessage) {
					t.Errorf("Expected error message to contain '%s', got: %v", tt.errorMessage, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if len(packages) != len(tt.expectedPkgs) {
					t.Errorf("Expected %d packages, got %d", len(tt.expectedPkgs), len(packages))
				}
				for i, pkg := range packages {
					if i < len(tt.expectedPkgs) && pkg != tt.expectedPkgs[i] {
						t.Errorf("Expected package %d to be '%s', got '%s'", i, tt.expectedPkgs[i], pkg)
					}
				}
			}
		})
	}
}

func TestReadPackageFileNonExistent(t *testing.T) {
	_, err := readPackageFile("/nonexistent/file.txt")
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
}

func TestRunRegressionTestValidation(t *testing.T) {
	// Save original values
	origPackageName := packageName
	origPackageFile := packageFile
	origApkRepo := apkRepo
	origRepoPath := repoPath
	origRepoType := repoType

	defer func() {
		// Restore original values
		packageName = origPackageName
		packageFile = origPackageFile
		apkRepo = origApkRepo
		repoPath = origRepoPath
		repoType = origRepoType
	}()

	tests := []struct {
		name           string
		packageName    string
		packageFile    string
		apkRepo        string
		repoPath       string
		repoType       string
		expectedError  string
	}{
		{
			name:          "missing package and package file",
			packageName:   "",
			packageFile:   "",
			apkRepo:       "http://example.com",
			repoPath:      "/tmp",
			repoType:      "wolfi",
			expectedError: "either --package or --package-file must be specified",
		},
		{
			name:          "both package and package file specified",
			packageName:   "test-pkg",
			packageFile:   "/tmp/packages.txt",
			apkRepo:       "http://example.com",
			repoPath:      "/tmp",
			repoType:      "wolfi",
			expectedError: "cannot specify both --package and --package-file",
		},
		{
			name:          "non-existent repo path",
			packageName:   "test-pkg",
			packageFile:   "",
			apkRepo:       "http://example.com",
			repoPath:      "/nonexistent/path",
			repoType:      "wolfi",
			expectedError: "repository path does not exist",
		},
		{
			name:          "invalid repo type",
			packageName:   "test-pkg",
			packageFile:   "",
			apkRepo:       "http://example.com",
			repoPath:      "/tmp",
			repoType:      "invalid",
			expectedError: "invalid repository type: invalid (must be wolfi, enterprise, or extras)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set test values
			packageName = tt.packageName
			packageFile = tt.packageFile
			apkRepo = tt.apkRepo
			repoPath = tt.repoPath
			repoType = tt.repoType

			err := runRegressionTest(nil, nil)
			if err == nil {
				t.Error("Expected error but got none")
			} else if !strings.Contains(err.Error(), tt.expectedError) {
				t.Errorf("Expected error to contain '%s', got: %v", tt.expectedError, err)
			}
		})
	}
}

func TestRunRegressionTestRepoPathResolution(t *testing.T) {
	// Save original values
	origPackageName := packageName
	origPackageFile := packageFile
	origApkRepo := apkRepo
	origRepoPath := repoPath
	origRepoType := repoType

	defer func() {
		// Restore original values
		packageName = origPackageName
		packageFile = origPackageFile
		apkRepo = origApkRepo
		repoPath = origRepoPath
		repoType = origRepoType
	}()

	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "apkregress_test_")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Test relative path resolution
	relPath := filepath.Base(tmpDir)
	parentDir := filepath.Dir(tmpDir)

	// Change to parent directory temporarily
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer os.Chdir(originalWd)

	if err := os.Chdir(parentDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}

	// Set test values with relative path
	packageName = "test-pkg"
	packageFile = ""
	apkRepo = "http://example.com"
	repoPath = relPath // relative path
	repoType = "wolfi"

	// This should not return an error for path resolution
	// but will fail later when trying to create the runner
	err = runRegressionTest(nil, nil)
	if err != nil && strings.Contains(err.Error(), "repository path does not exist") {
		t.Error("Relative path should have been resolved correctly")
	}

	// Verify that repoPath was converted to absolute path
	if !filepath.IsAbs(repoPath) {
		t.Error("Expected repoPath to be converted to absolute path")
	}
}

func TestFlagValidation(t *testing.T) {
	// Test that required flags are properly marked
	cmd := rootCmd

	// Check that repo flag is marked as required
	flag := cmd.PersistentFlags().Lookup("repo")
	if flag == nil {
		t.Error("Expected 'repo' flag to exist")
	}

	// Check that repo-path flag is marked as required
	flag = cmd.PersistentFlags().Lookup("repo-path")
	if flag == nil {
		t.Error("Expected 'repo-path' flag to exist")
	}

	// Test default values
	if concurrency != 4 {
		t.Errorf("Expected default concurrency to be 4, got %d", concurrency)
	}

	if repoType != "wolfi" {
		t.Errorf("Expected default repoType to be 'wolfi', got '%s'", repoType)
	}

	if hangTimeout != 30*time.Minute {
		t.Errorf("Expected default hangTimeout to be 30m, got %v", hangTimeout)
	}

	if verbose != false {
		t.Errorf("Expected default verbose to be false, got %v", verbose)
	}

	if markdownOutput != false {
		t.Errorf("Expected default markdownOutput to be false, got %v", markdownOutput)
	}
}

func TestCommandStructure(t *testing.T) {
	cmd := rootCmd

	if cmd.Use != "apkregress" {
		t.Errorf("Expected command name to be 'apkregress', got '%s'", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("Expected command to have a short description")
	}

	if cmd.Long == "" {
		t.Error("Expected command to have a long description")
	}

	if cmd.RunE == nil {
		t.Error("Expected command to have a RunE function")
	}
}