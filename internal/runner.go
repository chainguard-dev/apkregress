package internal

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sync/semaphore"
)

type TestResult struct {
	Package  string
	WithRepo bool
	Success  bool
	Error    error
}

type RegressionTestRunner struct {
	packageName    string
	apkRepo        string
	repoPath       string
	repoType       string
	concurrency    int
	verbose        bool
	logDir         string
	apkrane        *ApkraneClient
	melange        *MelangeClient
	completedTests int64
	totalTests     int64
	startTime      time.Time
}

func (r *RegressionTestRunner) updateProgress() {
	if r.verbose {
		return // Don't show progress in verbose mode
	}

	completed := atomic.AddInt64(&r.completedTests, 1)
	total := r.totalTests

	if completed > total {
		return // Safety check
	}

	// Calculate progress percentage
	progress := float64(completed) / float64(total) * 100

	// Calculate elapsed time and estimate remaining time
	elapsed := time.Since(r.startTime)
	var eta time.Duration
	if completed > 0 {
		avgTimePerTest := elapsed / time.Duration(completed)
		remaining := total - completed
		eta = avgTimePerTest * time.Duration(remaining)
	}

	// Format the progress update
	if eta > 0 {
		fmt.Printf("\rProgress: %d/%d (%.1f%%) - ETA: %v", completed, total, progress, eta.Round(time.Second))
	} else {
		fmt.Printf("\rProgress: %d/%d (%.1f%%)", completed, total, progress)
	}

	// Print newline when complete
	if completed == total {
		fmt.Println()
	}
}

func NewRegressionTestRunner(packageName, apkRepo, repoPath, repoType string, concurrency int, verbose bool) *RegressionTestRunner {
	// Create log directory with timestamp
	timestamp := time.Now().Format("20060102-150405")
	logDir := filepath.Join("logs", fmt.Sprintf("regression-test-%s-%s", packageName, timestamp))

	return &RegressionTestRunner{
		packageName: packageName,
		apkRepo:     apkRepo,
		repoPath:    repoPath,
		repoType:    repoType,
		concurrency: concurrency,
		verbose:     verbose,
		logDir:      logDir,
		apkrane:     NewApkraneClient(verbose, repoType),
		melange:     NewMelangeClient(repoPath, verbose, logDir),
	}
}

func (r *RegressionTestRunner) Run() error {
	// Create log directory
	if err := os.MkdirAll(r.logDir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory %s: %w", r.logDir, err)
	}

	reverseDeps, err := r.apkrane.GetReverseDependencies(r.packageName)
	if err != nil {
		return fmt.Errorf("failed to get reverse dependencies: %w", err)
	}

	if len(reverseDeps) == 0 {
		fmt.Printf("No reverse dependencies found for package: %s\n", r.packageName)
		return nil
	}

	fmt.Printf("Testing %d reverse dependencies with concurrency %d\n", len(reverseDeps), r.concurrency)
	fmt.Printf("Logs will be saved to: %s\n", r.logDir)

	// Initialize progress tracking
	r.totalTests = int64(len(reverseDeps))
	r.startTime = time.Now()

	results := make(chan TestResult, len(reverseDeps)*2)
	ctx := context.Background()
	sem := semaphore.NewWeighted(int64(r.concurrency))
	var wg sync.WaitGroup
	var skippedCount int64

	for _, pkg := range reverseDeps {
		wg.Add(1)
		go func(packageName string) {
			defer wg.Done()
			sem.Acquire(ctx, 1)
			defer sem.Release(1)

			// First test with repo
			err := r.melange.TestPackage(packageName, true, r.apkRepo)

			// Skip package if YAML file not found
			if errors.Is(err, ErrPackageYAMLNotFound) {
				atomic.AddInt64(&skippedCount, 1)
				r.updateProgress()
				return
			}

			withRepoResult := TestResult{
				Package:  packageName,
				WithRepo: true,
				Success:  err == nil,
				Error:    err,
			}
			results <- withRepoResult

			// Only test without repo if test with repo failed
			if !withRepoResult.Success {
				err := r.melange.TestPackage(packageName, false, r.apkRepo)

				// Skip if YAML file not found (shouldn't happen since we already checked, but for safety)
				if errors.Is(err, ErrPackageYAMLNotFound) {
					r.updateProgress()
					return
				}

				results <- TestResult{
					Package:  packageName,
					WithRepo: false,
					Success:  err == nil,
					Error:    err,
				}
			}

			// Update progress after completing all tests for this package
			r.updateProgress()
		}(pkg)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	return r.analyzeResults(results, len(reverseDeps), int(atomic.LoadInt64(&skippedCount)))
}

func (r *RegressionTestRunner) analyzeResults(results chan TestResult, expectedPackages int, skippedPackages int) error {
	packageResults := make(map[string]map[bool]TestResult)

	for result := range results {
		if packageResults[result.Package] == nil {
			packageResults[result.Package] = make(map[bool]TestResult)
		}
		packageResults[result.Package][result.WithRepo] = result
	}

	var regressions []string
	var successCount, failureCount int

	fmt.Println("\n=== Test Results ===")
	for pkg, results := range packageResults {
		withRepoResult, hasWithRepo := results[true]
		withoutRepoResult, hasWithoutRepo := results[false]

		if !hasWithRepo {
			fmt.Printf("⚠️  %s: Incomplete test results\n", pkg)
			continue
		}

		// If with-repo test passed, we didn't run without-repo test
		if withRepoResult.Success && !hasWithoutRepo {
			successCount++
			if r.verbose {
				fmt.Printf("✅ %s: PASS (with repo, without-repo test skipped)\n", pkg)
			}
		} else if !withRepoResult.Success && hasWithoutRepo {
			// Both tests were run because with-repo failed
			if withoutRepoResult.Success {
				regressions = append(regressions, pkg)
				fmt.Printf("🔴 %s: REGRESSION DETECTED (fails with repo, passes without)\n", pkg)
			} else {
				failureCount++
				if r.verbose {
					fmt.Printf("❌ %s: FAIL (both scenarios)\n", pkg)
				}
			}
		} else if !withRepoResult.Success && !hasWithoutRepo {
			fmt.Printf("⚠️  %s: Incomplete test results (with-repo failed but no without-repo test)\n", pkg)
			continue
		}
	}

	fmt.Printf("\n=== Summary ===\n")
	fmt.Printf("Total packages found: %d\n", expectedPackages)
	fmt.Printf("Packages skipped (no YAML): %d\n", skippedPackages)
	fmt.Printf("Packages tested: %d\n", len(packageResults))
	fmt.Printf("Regressions detected: %d\n", len(regressions))
	fmt.Printf("Successful packages: %d\n", successCount)
	fmt.Printf("Failed packages: %d\n", failureCount)

	if len(regressions) > 0 {
		fmt.Printf("\nPackages with regressions:\n")
		for _, pkg := range regressions {
			fmt.Printf("  - %s\n", pkg)
		}
		return fmt.Errorf("found %d regressions", len(regressions))
	}

	return nil
}
