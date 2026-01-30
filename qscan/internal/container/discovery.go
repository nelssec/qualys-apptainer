package container

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/shirou/gopsutil/v3/process"
)

type ContainerInfo struct {
	PID        int
	User       string
	Command    string
	Accessible bool
	RootFS     string
}

func ListContainers() error {
	containers, err := findContainers()
	if err != nil {
		return err
	}

	if len(containers) == 0 {
		fmt.Println("No running Apptainer/Singularity containers found")
		return nil
	}

	fmt.Println()
	fmt.Println("Running Apptainer/Singularity Containers")
	fmt.Println("=========================================")
	fmt.Println()

	for _, c := range containers {
		fmt.Printf("PID: %d\n", c.PID)
		fmt.Printf("  User:    %s\n", c.User)
		fmt.Printf("  Command: %s\n", truncateString(c.Command, 80))
		if c.Accessible {
			fmt.Printf("  RootFS:  %s (accessible)\n", c.RootFS)
		} else {
			fmt.Printf("  RootFS:  %s (permission denied)\n", c.RootFS)
		}
		fmt.Println()
	}

	return nil
}

func FindContainersByName(pattern string) ([]ContainerInfo, error) {
	all, err := findContainers()
	if err != nil {
		return nil, err
	}

	var matches []ContainerInfo
	pattern = strings.ToLower(pattern)

	for _, c := range all {
		if strings.Contains(strings.ToLower(c.Command), pattern) {
			matches = append(matches, c)
		}
	}

	return matches, nil
}

func findContainers() ([]ContainerInfo, error) {
	if runtime.GOOS != "linux" {
		return nil, fmt.Errorf("container discovery only works on Linux")
	}

	procs, err := process.Processes()
	if err != nil {
		return nil, fmt.Errorf("failed to list processes: %w", err)
	}

	var containers []ContainerInfo

	for _, p := range procs {
		cmdline, err := p.Cmdline()
		if err != nil {
			continue
		}

		if !isApptainerProcess(cmdline) {
			continue
		}

		username := "unknown"
		if uids, err := p.Uids(); err == nil && len(uids) > 0 {
			if u, err := user.LookupId(fmt.Sprintf("%d", uids[0])); err == nil {
				username = u.Username
			}
		}

		rootfs := fmt.Sprintf("/proc/%d/root", p.Pid)
		accessible := isAccessible(rootfs)

		containers = append(containers, ContainerInfo{
			PID:        int(p.Pid),
			User:       username,
			Command:    truncateString(cmdline, 200),
			Accessible: accessible,
			RootFS:     rootfs,
		})
	}

	return containers, nil
}

func isApptainerProcess(cmdline string) bool {
	lower := strings.ToLower(cmdline)

	if strings.Contains(lower, "apptainer") || strings.Contains(lower, "singularity") {
		if strings.Contains(lower, " run ") ||
			strings.Contains(lower, " exec ") ||
			strings.Contains(lower, " shell ") ||
			strings.Contains(lower, " instance ") {
			return true
		}
	}

	if strings.Contains(lower, "starter-suid") || strings.Contains(lower, "starter") {
		if strings.Contains(cmdline, ".sif") || strings.Contains(lower, "singularity") || strings.Contains(lower, "apptainer") {
			return true
		}
	}

	return false
}

func isAccessible(rootfs string) bool {
	testPath := filepath.Join(rootfs, "etc")
	_, err := os.Stat(testPath)
	return err == nil
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
