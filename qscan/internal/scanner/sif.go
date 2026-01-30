package scanner

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/qualys/qscan/internal/container"
)

func ScanSIF(ctx context.Context, sifPath string, runtime container.ContainerRuntime, opts ScanOptions) (*ScanResult, error) {
	result := &ScanResult{
		Target:    sifPath,
		Type:      "sif",
		StartTime: time.Now(),
		Reports:   make(map[string]string),
	}

	absPath, err := filepath.Abs(sifPath)
	if err != nil {
		return result, fmt.Errorf("failed to resolve path: %w", err)
	}

	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return result, fmt.Errorf("SIF file not found: %s", absPath)
	}

	tempDir, err := os.MkdirTemp("", "qscan-rootfs-*")
	if err != nil {
		return result, fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	if !opts.Quiet {
		fmt.Printf("  Extracting filesystem from %s...\n", filepath.Base(sifPath))
	}

	if err := runtime.ExtractFilesystem(absPath, tempDir); err != nil {
		return result, fmt.Errorf("failed to extract filesystem: %w", err)
	}

	if !opts.Quiet {
		fileCount := countFiles(tempDir)
		fmt.Printf("  Extracted %d files\n", fileCount)
	}

	if !opts.Quiet {
		fmt.Printf("  Detecting OS information...\n")
	}

	unameOutput, err := runtime.Exec(absPath, []string{"uname", "-a"})
	if err != nil {
		unameOutput = []byte("Linux unknown")
	}
	result.OSInfo = strings.TrimSpace(string(unameOutput))

	if !opts.Quiet {
		fmt.Printf("  OS: %s\n", result.OSInfo)
	}

	sifName := strings.TrimSuffix(filepath.Base(sifPath), ".sif")
	outputDir := opts.OutputDir
	if outputDir == "" {
		outputDir = "./reports"
	}
	outputDir = filepath.Join(outputDir, sifName)

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return result, fmt.Errorf("failed to create output directory: %w", err)
	}

	if !opts.Quiet {
		fmt.Printf("  Running vulnerability scan...\n")
	}

	args := buildQScannerArgs(opts, result.OSInfo, outputDir)
	args = append(args, "rootfs", tempDir)

	cmd := exec.CommandContext(ctx, opts.QScannerPath, args...)
	cmd.Env = append(os.Environ(), fmt.Sprintf("QUALYS_ACCESS_TOKEN=%s", opts.Token))

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	result.EndTime = time.Now()
	result.DurationSeconds = result.EndTime.Sub(result.StartTime).Seconds()
	result.RawOutput = stdout.String() + stderr.String()

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode = 1
			result.Error = err.Error()
		}
	}

	result.Reports = findReports(outputDir)

	return result, nil
}

func buildQScannerArgs(opts ScanOptions, unameOutput, outputDir string) []string {
	args := []string{}

	if opts.Pod != "" {
		args = append(args, "--pod", opts.Pod)
	}

	if opts.ScanTypes != "" {
		args = append(args, "--scan-types", opts.ScanTypes)
	}

	if opts.Mode != "" {
		args = append(args, "--mode", opts.Mode)
	}

	if opts.Format != "" {
		args = append(args, "--format", opts.Format)
	}

	if unameOutput != "" {
		args = append(args, "--shell-commands", fmt.Sprintf("uname -a=%s", unameOutput))
	}

	if outputDir != "" {
		args = append(args, "--output-dir", outputDir)
	}

	return args
}

func countFiles(dir string) int {
	count := 0
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			count++
		}
		return nil
	})
	return count
}

func findReports(outputDir string) map[string]string {
	reports := make(map[string]string)

	patterns := map[string][]string{
		"json":      {"*.json"},
		"spdx":      {"*spdx*.json", "*.spdx"},
		"cyclonedx": {"*cyclonedx*.json", "*cdx*.json"},
		"sarif":     {"*.sarif", "*sarif*.json"},
	}

	for format, globs := range patterns {
		for _, pattern := range globs {
			matches, err := filepath.Glob(filepath.Join(outputDir, pattern))
			if err == nil && len(matches) > 0 {
				reports[format] = matches[0]
				break
			}
		}
	}

	return reports
}
