package scanner

import (
	"time"
)

type ScanOptions struct {
	Token        string
	Pod          string
	ScanTypes    string
	Mode         string
	Format       string
	OutputDir    string
	QScannerPath string
	Quiet        bool
	ExcludeDirs  []string
}

type ScanResult struct {
	Target          string            `json:"target"`
	Type            string            `json:"type"`
	StartTime       time.Time         `json:"start_time"`
	EndTime         time.Time         `json:"end_time"`
	DurationSeconds float64           `json:"duration_seconds"`
	ExitCode        int               `json:"exit_code"`
	Error           string            `json:"error,omitempty"`
	OSInfo          string            `json:"os_info,omitempty"`
	Reports         map[string]string `json:"reports,omitempty"`
	RawOutput       string            `json:"-"`
}

type Vulnerability struct {
	Severity     string `json:"severity"`
	CVE          string `json:"cve"`
	Package      string `json:"package"`
	Version      string `json:"version"`
	FixedVersion string `json:"fixed_version,omitempty"`
}

type Inventory struct {
	Packages []Package `json:"packages"`
}

type Package struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Type    string `json:"type"`
}
