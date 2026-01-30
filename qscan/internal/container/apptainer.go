package container

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

type ContainerRuntime interface {
	Name() string
	Exec(image string, cmd []string) ([]byte, error)
	ExtractFilesystem(image string, destDir string) error
}

type apptainerRuntime struct {
	binary string
}

func DetectRuntime() (ContainerRuntime, error) {
	if path, err := exec.LookPath("apptainer"); err == nil {
		return &apptainerRuntime{binary: path}, nil
	}

	if path, err := exec.LookPath("singularity"); err == nil {
		return &apptainerRuntime{binary: path}, nil
	}

	return nil, fmt.Errorf("neither apptainer nor singularity found in PATH")
}

func (r *apptainerRuntime) Name() string {
	return filepath.Base(r.binary)
}

func (r *apptainerRuntime) Exec(image string, cmd []string) ([]byte, error) {
	args := append([]string{"exec", image}, cmd...)
	c := exec.Command(r.binary, args...)

	var stdout, stderr bytes.Buffer
	c.Stdout = &stdout
	c.Stderr = &stderr

	err := c.Run()
	if err != nil {
		return nil, fmt.Errorf("exec failed: %w (stderr: %s)", err, stderr.String())
	}

	return stdout.Bytes(), nil
}

func (r *apptainerRuntime) ExtractFilesystem(image string, destDir string) error {
	tarCmd := exec.Command(r.binary, "exec", image, "tar", "-cf", "-",
		"--exclude=/proc",
		"--exclude=/sys",
		"--exclude=/dev",
		"--exclude=/run",
		"--exclude=/tmp",
		"/",
	)

	extractCmd := exec.Command("tar", "-xf", "-", "-C", destDir)

	pipe, err := tarCmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create pipe: %w", err)
	}

	extractCmd.Stdin = pipe

	var tarStderr, extractStderr bytes.Buffer
	tarCmd.Stderr = &tarStderr
	extractCmd.Stderr = &extractStderr

	if err := extractCmd.Start(); err != nil {
		return fmt.Errorf("failed to start extraction: %w", err)
	}

	if err := tarCmd.Start(); err != nil {
		extractCmd.Process.Kill()
		return fmt.Errorf("failed to start tar: %w", err)
	}

	tarCmd.Wait()
	extractCmd.Wait()

	entries, err := os.ReadDir(destDir)
	if err != nil {
		return fmt.Errorf("failed to read extracted directory: %w", err)
	}

	if len(entries) == 0 {
		return fmt.Errorf("no files extracted from image")
	}

	return nil
}
