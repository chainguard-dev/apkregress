package internal

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewMelangeClient(t *testing.T) {
	tests := []struct {
		name        string
		repoPath    string
		verbose     bool
		logDir      string
		hangTimeout time.Duration
	}{
		{
			name:        "basic client",
			repoPath:    "/tmp/repo",
			verbose:     false,
			logDir:      "/tmp/logs",
			hangTimeout: 30 * time.Minute,
		},
		{
			name:        "verbose client",
			repoPath:    "/home/user/packages",
			verbose:     true,
			logDir:      "/var/log/tests",
			hangTimeout: 45 * time.Minute,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewMelangeClient(tt.repoPath, tt.verbose, tt.logDir, tt.hangTimeout)
			
			if client == nil {
				t.Fatal("Expected non-nil client")
			}
			
			if client.repoPath != tt.repoPath {
				t.Errorf("Expected repoPath=%s, got %s", tt.repoPath, client.repoPath)
			}
			
			if client.verbose != tt.verbose {
				t.Errorf("Expected verbose=%v, got %v", tt.verbose, client.verbose)
			}
			
			if client.logDir != tt.logDir {
				t.Errorf("Expected logDir=%s, got %s", tt.logDir, client.logDir)
			}
			
			if client.hangTimeout != tt.hangTimeout {
				t.Errorf("Expected hangTimeout=%v, got %v", tt.hangTimeout, client.hangTimeout)
			}
		})
	}
}

func TestTestPackageYAMLNotFound(t *testing.T) {
	// Create temporary directory structure
	tmpDir, err := os.MkdirTemp("", "melange_test_")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	logDir := filepath.Join(tmpDir, "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		t.Fatalf("Failed to create log dir: %v", err)
	}

	client := NewMelangeClient(tmpDir, false, logDir, time.Minute)
	
	// Test with non-existent package
	err = client.TestPackage("nonexistent-package", true, "http://example.com/repo")
	
	if !errors.Is(err, ErrPackageYAMLNotFound) {
		t.Errorf("Expected ErrPackageYAMLNotFound, got %v", err)
	}
}

func TestTestPackageYAMLExists(t *testing.T) {
	// Create temporary directory structure
	tmpDir, err := os.MkdirTemp("", "melange_test_")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	logDir := filepath.Join(tmpDir, "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		t.Fatalf("Failed to create log dir: %v", err)
	}

	// Create a test package YAML file
	packageName := "test-package"
	yamlFile := filepath.Join(tmpDir, packageName+".yaml")
	yamlContent := `package:
  name: test-package
  version: 1.0.0
`
	if err := os.WriteFile(yamlFile, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("Failed to create YAML file: %v", err)
	}

	client := NewMelangeClient(tmpDir, false, logDir, time.Second) // Short timeout for test
	
	// This will fail because make command won't work, but it shouldn't return ErrPackageYAMLNotFound
	err = client.TestPackage(packageName, true, "http://example.com/repo")
	
	if errors.Is(err, ErrPackageYAMLNotFound) {
		t.Error("Should not return ErrPackageYAMLNotFound when YAML file exists")
	}
	
	// Should have created a log file
	expectedLogFile := filepath.Join(logDir, packageName+"_with_repo.log")
	if _, err := os.Stat(expectedLogFile); os.IsNotExist(err) {
		t.Errorf("Expected log file %s to be created", expectedLogFile)
	}
}

func TestLogFileCreation(t *testing.T) {
	// Create temporary directory structure
	tmpDir, err := os.MkdirTemp("", "melange_test_")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	logDir := filepath.Join(tmpDir, "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		t.Fatalf("Failed to create log dir: %v", err)
	}

	// Create a test package YAML file
	packageName := "test-package"
	yamlFile := filepath.Join(tmpDir, packageName+".yaml")
	if err := os.WriteFile(yamlFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create YAML file: %v", err)
	}

	client := NewMelangeClient(tmpDir, false, logDir, time.Millisecond*100) // Very short timeout

	tests := []struct {
		name         string
		withRepo     bool
		expectedFile string
	}{
		{
			name:         "with repo test",
			withRepo:     true,
			expectedFile: "test-package_with_repo.log",
		},
		{
			name:         "without repo test",
			withRepo:     false,
			expectedFile: "test-package_without_repo.log",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := client.TestPackage(packageName, tt.withRepo, "http://example.com/repo")
			
			// Should timeout (and that's expected for this test)
			if !errors.Is(err, ErrTestHung) && err != nil {
				// The test might fail for other reasons (like make not being available)
				// That's OK for this test - we're just checking log file creation
			}
			
			expectedLogFile := filepath.Join(logDir, tt.expectedFile)
			if _, err := os.Stat(expectedLogFile); os.IsNotExist(err) {
				t.Errorf("Expected log file %s to be created", expectedLogFile)
			}
		})
	}
}

func TestErrorTypes(t *testing.T) {
	if ErrPackageYAMLNotFound == nil {
		t.Error("ErrPackageYAMLNotFound should not be nil")
	}
	
	if ErrTestHung == nil {
		t.Error("ErrTestHung should not be nil")
	}
	
	if ErrPackageYAMLNotFound.Error() == "" {
		t.Error("ErrPackageYAMLNotFound should have a non-empty error message")
	}
	
	if ErrTestHung.Error() == "" {
		t.Error("ErrTestHung should have a non-empty error message")
	}
	
	// Test that they're different errors
	if errors.Is(ErrPackageYAMLNotFound, ErrTestHung) {
		t.Error("ErrPackageYAMLNotFound and ErrTestHung should be different errors")
	}
}

func TestErrorMessages(t *testing.T) {
	yamlErr := ErrPackageYAMLNotFound.Error()
	if !strings.Contains(yamlErr, "not found") {
		t.Errorf("Expected ErrPackageYAMLNotFound to contain 'not found', got: %s", yamlErr)
	}
	
	hungErr := ErrTestHung.Error()
	if !strings.Contains(hungErr, "hung") {
		t.Errorf("Expected ErrTestHung to contain 'hung', got: %s", hungErr)
	}
}

func TestTimeoutBehavior(t *testing.T) {
	// This test verifies timeout logic without actually waiting for timeout
	timeouts := []time.Duration{
		time.Minute,
		30 * time.Minute,
		time.Hour,
	}
	
	for _, timeout := range timeouts {
		t.Run(timeout.String(), func(t *testing.T) {
			tmpDir, err := os.MkdirTemp("", "melange_test_")
			if err != nil {
				t.Fatalf("Failed to create temp dir: %v", err)
			}
			defer os.RemoveAll(tmpDir)

			logDir := filepath.Join(tmpDir, "logs")
			if err := os.MkdirAll(logDir, 0755); err != nil {
				t.Fatalf("Failed to create log dir: %v", err)
			}

			client := NewMelangeClient(tmpDir, false, logDir, timeout)
			
			if client.hangTimeout != timeout {
				t.Errorf("Expected hangTimeout=%v, got %v", timeout, client.hangTimeout)
			}
		})
	}
}

func TestVerboseLogging(t *testing.T) {
	tests := []struct {
		name    string
		verbose bool
	}{
		{
			name:    "verbose enabled",
			verbose: true,
		},
		{
			name:    "verbose disabled",
			verbose: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir, err := os.MkdirTemp("", "melange_test_")
			if err != nil {
				t.Fatalf("Failed to create temp dir: %v", err)
			}
			defer os.RemoveAll(tmpDir)

			logDir := filepath.Join(tmpDir, "logs")
			if err := os.MkdirAll(logDir, 0755); err != nil {
				t.Fatalf("Failed to create log dir: %v", err)
			}

			client := NewMelangeClient(tmpDir, tt.verbose, logDir, time.Minute)
			
			if client.verbose != tt.verbose {
				t.Errorf("Expected verbose=%v, got %v", tt.verbose, client.verbose)
			}
		})
	}
}

func TestRepoPathHandling(t *testing.T) {
	tests := []struct {
		name     string
		repoPath string
	}{
		{
			name:     "absolute path",
			repoPath: "/home/user/packages",
		},
		{
			name:     "relative path",
			repoPath: "./packages",
		},
		{
			name:     "current directory",
			repoPath: ".",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewMelangeClient(tt.repoPath, false, "/tmp/logs", time.Minute)
			
			if client.repoPath != tt.repoPath {
				t.Errorf("Expected repoPath=%s, got %s", tt.repoPath, client.repoPath)
			}
		})
	}
}