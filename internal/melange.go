package internal

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

type MelangeClient struct {
	wolfiOSPath string
	verbose     bool
	logDir      string
}

// ErrPackageYAMLNotFound indicates that the package YAML file doesn't exist
var ErrPackageYAMLNotFound = errors.New("package YAML file not found")

func NewMelangeClient(wolfiOSPath string, verbose bool, logDir string) *MelangeClient {
	return &MelangeClient{
		wolfiOSPath: wolfiOSPath,
		verbose:     verbose,
		logDir:      logDir,
	}
}

func (m *MelangeClient) TestPackage(packageName string, withRepo bool, apkRepo string) error {
	// Check if the package YAML file exists
	yamlFilePath := filepath.Join(m.wolfiOSPath, fmt.Sprintf("%s.yaml", packageName))
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

	cmd.Dir = m.wolfiOSPath
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("make test/%s failed: %w", packageName, err)
	}

	return nil
}
