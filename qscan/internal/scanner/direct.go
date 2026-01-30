package scanner

import (
	"context"
	"fmt"
	"os"
	"os/exec"
)

func RunDirect(ctx context.Context, subcommand string, args []string, opts ScanOptions) error {
	cmdArgs := []string{}

	if opts.Pod != "" {
		cmdArgs = append(cmdArgs, "--pod", opts.Pod)
	}

	if opts.ScanTypes != "" {
		cmdArgs = append(cmdArgs, "--scan-types", opts.ScanTypes)
	}

	if opts.Mode != "" {
		cmdArgs = append(cmdArgs, "--mode", opts.Mode)
	}

	if opts.Format != "" {
		cmdArgs = append(cmdArgs, "--format", opts.Format)
	}

	if opts.OutputDir != "" {
		cmdArgs = append(cmdArgs, "--output-dir", opts.OutputDir)
	}

	cmdArgs = append(cmdArgs, subcommand)
	cmdArgs = append(cmdArgs, args...)

	cmd := exec.CommandContext(ctx, opts.QScannerPath, cmdArgs...)
	cmd.Env = append(os.Environ(), fmt.Sprintf("QUALYS_ACCESS_TOKEN=%s", opts.Token))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	return cmd.Run()
}
