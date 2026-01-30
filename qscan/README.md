# qscan - Qualys QScanner Wrapper for Apptainer/Singularity

A Go binary that wraps the Qualys qscanner for scanning Apptainer/Singularity containers. The binary embeds qscanner internally for easy distribution.

## Features

- Scan SIF files by extracting their filesystem
- Scan running containers via `/proc/<pid>/root`
- List running Apptainer/Singularity containers
- Passthrough to qscanner's `image` and `repo` commands
- Configuration via CLI flags, environment variables, or config file
- JSON output for automation

## Installation

```bash
cd qscan
make build
./qscan version
```

## Usage

### Scan a SIF File

```bash
# Basic scan
qscan sif /path/to/app.sif

# Scan with secrets detection
qscan sif app.sif --scan-types pkg,fileinsight,secret

# Air-gapped mode (no credentials needed)
qscan sif app.sif --mode inventory-only

# JSON output for automation
qscan sif app.sif --json
```

### Scan Running Containers

```bash
# List running containers
qscan running --list

# Scan by PID
qscan running 12345

# Scan by name pattern
qscan running myapp.sif
```

### Registry Image (passthrough)

```bash
qscan image docker://alpine:latest
```

### Source Repository (passthrough)

```bash
qscan repo /path/to/source
```

## Configuration

### Priority Order

1. CLI flags (highest)
2. Environment variables
3. Config file (`~/.config/qscan/config.yaml`)
4. Defaults (lowest)

### Environment Variables

```bash
export QUALYS_ACCESS_TOKEN="your-token"
export QUALYS_POD="US2"
export SCAN_TYPES="pkg,fileinsight"
export OUTPUT_DIR="./reports"
```

### Config File

Create `~/.config/qscan/config.yaml`:

```yaml
qualys:
  token: "your-token"
  pod: "US2"

defaults:
  scan_types: "pkg,fileinsight"
  mode: "get-report"
  output_dir: "./reports"
```

## Global Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--token` | `-t` | Qualys access token |
| `--pod` | `-p` | Qualys POD (US1, US2, EU1, etc.) |
| `--scan-types` | | Scan types: pkg, fileinsight, secret |
| `--mode` | | Mode: get-report, scan-only, inventory-only, evaluate-policy |
| `--format` | | Output formats: json, spdx, cyclonedx, sarif |
| `--output-dir` | `-o` | Output directory for reports |
| `--config` | `-c` | Config file path |
| `--json` | | Output as JSON |
| `--quiet` | `-q` | Suppress progress output |
| `--verbose` | `-v` | Verbose logging |

## Output

### Table Output (default)

```
Qualys QScanner Results
=======================
Target:     /shared/containers/app.sif
Type:       sif
OS:         Linux Ubuntu 22.04
Duration:   45.2s
Exit Code:  0

Reports:
  FORMAT     PATH
  json       ./reports/app/inventory.json
  spdx       ./reports/app/inventory.spdx.json
```

### JSON Output (--json)

```json
{
  "target": "/shared/containers/app.sif",
  "type": "sif",
  "duration_seconds": 45.2,
  "exit_code": 0,
  "os_info": "Linux Ubuntu 22.04",
  "reports": {
    "json": "./reports/app/inventory.json",
    "spdx": "./reports/app/inventory.spdx.json"
  }
}
```

## Requirements

- Linux (for running qscanner)
- Apptainer or Singularity installed (for SIF scanning)
- Root or same-user permissions (for running container scanning)

## Building

```bash
# Build for current platform
make build

# Build for Linux
make build-linux

# Build for all platforms
make build-all
```
