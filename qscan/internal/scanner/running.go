package scanner

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/qualys/qscan/internal/container"
)

func ScanRunning(ctx context.Context, target string, opts ScanOptions) (*ScanResult, error) {
	result := &ScanResult{
		Target:    target,
		Type:      "running",
		StartTime: time.Now(),
		Reports:   make(map[string]string),
	}

	pid, err := resolvePID(target)
	if err != nil {
		return result, err
	}

	rootfs := fmt.Sprintf("/proc/%d/root", pid)

	if _, err := os.Stat(rootfs); os.IsNotExist(err) {
		return result, fmt.Errorf("process %d does not exist", pid)
	}

	testPath := filepath.Join(rootfs, "etc")
	if _, err := os.Stat(testPath); err != nil {
		return result, fmt.Errorf("cannot access %s: permission denied (try running as root or same user)", rootfs)
	}

	if !opts.Quiet {
		fmt.Printf("  Scanning running container (PID: %d)\n", pid)
	}

	osInfo := getOSInfo(rootfs)
	result.OSInfo = osInfo

	if !opts.Quiet {
		fmt.Printf("  OS: %s\n", osInfo)
	}

	outputDir := opts.OutputDir
	if outputDir == "" {
		outputDir = "./reports"
	}
	outputDir = filepath.Join(outputDir, fmt.Sprintf("pid-%d", pid))

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return result, fmt.Errorf("failed to create output directory: %w", err)
	}

	if !opts.Quiet {
		fmt.Printf("  Running vulnerability scan...\n")
	}

	args := buildQScannerArgs(opts, osInfo, outputDir)
	args = append(args, "--exclude-dirs", "/proc,/sys,/dev,/run")
	args = append(args, "rootfs", rootfs)

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

func resolvePID(target string) (int, error) {
	if pid, err := strconv.Atoi(target); err == nil {
		if _, err := os.Stat(fmt.Sprintf("/proc/%d", pid)); err == nil {
			return pid, nil
		}
		return 0, fmt.Errorf("process %d does not exist", pid)
	}

	containers, err := container.FindContainersByName(target)
	if err != nil {
		return 0, err
	}

	if len(containers) == 0 {
		return 0, fmt.Errorf("no running container found matching: %s", target)
	}

	if len(containers) > 1 {
		fmt.Printf("Multiple containers found matching '%s':\n", target)
		for _, c := range containers {
			fmt.Printf("  PID %d: %s\n", c.PID, c.Command)
		}
		fmt.Printf("Using first match (PID %d)\n", containers[0].PID)
	}

	return containers[0].PID, nil
}

func getOSInfo(rootfs string) string {
	osReleasePath := filepath.Join(rootfs, "etc", "os-release")
	data, err := os.ReadFile(osReleasePath)
	if err != nil {
		return "Linux unknown"
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "PRETTY_NAME=") {
			value := strings.TrimPrefix(line, "PRETTY_NAME=")
			value = strings.Trim(value, "\"")
			return "Linux " + value
		}
	}

	for _, line := range lines {
		if strings.HasPrefix(line, "NAME=") {
			value := strings.TrimPrefix(line, "NAME=")
			value = strings.Trim(value, "\"")
			return "Linux " + value
		}
	}

	return "Linux unknown"
}
