# APK Regression Test Runner

A Go-based tool that uses apkrane to generate a list of reverse dependencies of a provided package and then uses melange to run the test makefile target for each package against a provided APK repository. If a package test fails, it repeats the test without the provided APK repository to detect regressions.

## Features

- Find reverse dependencies using apkrane
- Test packages using melange with configurable concurrency
- Regression detection by comparing results with and without APK repository
- Verbose output option for detailed logging

## Requirements

- Go 1.21+
- `apkrane` command-line tool
- `make` command
- Access to wolfi-dev/os repository

## Installation

```bash
go build -o apk-regression-test-runner .
```

## Usage

```bash
./apk-regression-test-runner \
  --package <package-name> \
  --repo <apk-repository-url> \
  --wolfi-os <path-to-wolfi-os-repo> \
  --concurrency 4 \
  --verbose
```

### Options

- `--package, -p`: Package name to find reverse dependencies for (required)
- `--repo, -r`: APK repository URL to test against (required)
- `--wolfi-os, -w`: Path to wolfi-dev/os repository (required)
- `--concurrency, -c`: Number of concurrent test jobs (default: 4)
- `--verbose, -v`: Enable verbose output

### Example

```bash
./apk-regression-test-runner \
  --package openssl \
  --repo https://packages.wolfi.dev/os/x86_64/APKINDEX.tar.gz \
  --wolfi-os /path/to/wolfi-dev/os \
  --concurrency 8 \
  --verbose
```

## How it works

1. Uses apkrane to query the Wolfi package index and find reverse dependencies
2. For each reverse dependency, runs two tests:
   - With the provided APK repository (using `MELANGE_EXTRA_OPTS`)
   - Without the provided APK repository
3. Compares results to detect regressions:
   - ✅ Pass: Both tests succeed or test improves with repository
   - ❌ Fail: Both tests fail (not a regression)
   - 🔴 Regression: Test fails with repository but passes without

## Output

The tool provides a summary showing:
- Total packages tested
- Number of regressions detected
- Successful and failed packages
- List of packages with regressions

Exit code 1 indicates regressions were found.