package internal

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
	"sort"
	"strings"
)

type ApkraneClient struct {
	verbose bool
}

type Package struct {
	Origin       string   `json:"Origin"`
	Dependencies []string `json:"Dependencies"`
}

func NewApkraneClient(verbose bool) *ApkraneClient {
	return &ApkraneClient{
		verbose: verbose,
	}
}

func (a *ApkraneClient) GetReverseDependencies(packageName string) ([]string, error) {
	if a.verbose {
		fmt.Printf("Finding reverse dependencies for package: %s\n", packageName)
	}

	arch := runtime.GOARCH
	if arch == "amd64" {
		arch = "x86_64"
	}

	indexURL := fmt.Sprintf("https://packages.wolfi.dev/os/%s/APKINDEX.tar.gz", arch)
	
	cmd := exec.Command("apkrane", "ls", "--json", "--latest", indexURL)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to run apkrane ls for %s: %w", indexURL, err)
	}

	var packages []Package
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		
		var pkg Package
		if err := json.Unmarshal([]byte(line), &pkg); err != nil {
			if a.verbose {
				fmt.Printf("Warning: failed to parse JSON line: %s\n", err)
			}
			continue
		}
		packages = append(packages, pkg)
	}
	
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read apkrane output: %w", err)
	}

	originSet := make(map[string]bool)
	for _, pkg := range packages {
		if pkg.Dependencies == nil {
			continue
		}
		
		for _, dep := range pkg.Dependencies {
			if strings.Contains(dep, packageName) {
				if pkg.Origin != "" {
					originSet[pkg.Origin] = true
				}
			}
		}
	}

	var origins []string
	for origin := range originSet {
		origins = append(origins, origin)
	}
	sort.Strings(origins)

	if a.verbose {
		fmt.Printf("Found %d reverse dependencies\n", len(origins))
	}

	return origins, nil
}