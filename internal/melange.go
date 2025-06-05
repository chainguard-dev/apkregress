package internal

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

type MelangeClient struct {
	repoPath    string
	verbose     bool
	logDir      string
	hangTimeout time.Duration
}

// ErrPackageYAMLNotFound indicates that the package YAML file doesn't exist
var ErrPackageYAMLNotFound = errors.New("package YAML file not found")

// ErrTestHung indicates that a test exceeded the timeout and was killed
var ErrTestHung = errors.New("test hung and was killed after timeout")

func NewMelangeClient(repoPath string, verbose bool, logDir string, hangTimeout time.Duration) *MelangeClient {
	return &MelangeClient{
		repoPath:    repoPath,
		verbose:     verbose,
		logDir:      logDir,
		hangTimeout: hangTimeout,
	}
}

func (m *MelangeClient) TestPackage(packageName string, withRepo bool, apkRepo string) error {
	// Check if the package YAML file exists
	yamlFilePath := filepath.Join(m.repoPath, fmt.Sprintf("%s.yaml", packageName))
	if _, err := os.Stat(yamlFilePath); os.IsNotExist(err) {
		if m.verbose {
			fmt.Printf("Skipping %s: YAML file not found at %s\n", packageName, yamlFilePath)
		}
		return ErrPackageYAMLNotFound
	}

	// Create temporary directory for build
	tempDir, err := os.MkdirTemp("/tmp", fmt.Sprintf("melange-build-%s-", packageName))
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	var cmd *exec.Cmd
	target := fmt.Sprintf("test/%s", packageName)

	// Create log file name
	logFileName := fmt.Sprintf("%s_%s.log", packageName, map[bool]string{true: "with_repo", false: "without_repo"}[withRepo])
	logFilePath := filepath.Join(m.logDir, logFileName)

	// Create and open log file
	logFile, err := os.Create(logFilePath)
	if err != nil {
		return fmt.Errorf("failed to create log file %s: %w", logFilePath, err)
	}
	defer logFile.Close()

	if withRepo {
		if m.verbose {
			fmt.Printf("Testing %s with APK repository: %s (temp: %s, log: %s)\n", packageName, apkRepo, tempDir, logFilePath)
		}
		cmd = exec.Command("make", target)
		extraOpts := fmt.Sprintf("--repository-append %s", apkRepo)
		cmd.Env = append(os.Environ(),
			fmt.Sprintf("MELANGE_EXTRA_OPTS=%s", extraOpts),
			fmt.Sprintf("TMPDIR=%s", tempDir))
	} else {
		if m.verbose {
			fmt.Printf("Testing %s without APK repository (temp: %s, log: %s)\n", packageName, tempDir, logFilePath)
		}
		cmd = exec.Command("make", target)
		cmd.Env = append(os.Environ(), fmt.Sprintf("TMPDIR=%s", tempDir))
	}

	cmd.Dir = m.repoPath
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	// Create context with configurable timeout
	ctx, cancel := context.WithTimeout(context.Background(), m.hangTimeout)
	defer cancel()

	// Start the command
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start make test/%s: %w", packageName, err)
	}

	// Channel to capture the result of cmd.Wait()
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	// Wait for either completion or timeout
	select {
	case err := <-done:
		if err != nil {
			return fmt.Errorf("make test/%s failed: %w", packageName, err)
		}
		return nil
	case <-ctx.Done():
		// Timeout occurred, kill the process
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
		// Wait for the process to actually exit
		<-done

		// Write timeout message to log
		fmt.Fprintf(logFile, "\n\n=== TEST HUNG - KILLED AFTER %v ===\n", m.hangTimeout)

		if m.verbose {
			fmt.Printf("Test %s hung and was killed after %v\n", packageName, m.hangTimeout)
		}

		return ErrTestHung
	}
}
