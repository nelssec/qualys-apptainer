# Qualys QScanner for Apptainer/Singularity

Vulnerability scanning for HPC container environments using Qualys QScanner's `rootfs` command.

## Overview

This toolkit enables Qualys vulnerability scanning for Apptainer (formerly Singularity) workloads on HPC clusters. It leverages QScanner's filesystem scanning capability—powered by [Aqua Trivy](https://github.com/aquasecurity/trivy)—to analyze native SIF images and running containers without requiring Docker.

## Requirements

- Linux x86_64
- Apptainer or Singularity
- Qualys subscription with Container Security
- Network access to Qualys platform (or use `--mode inventory-only` for air-gapped)

## Quick Start

```bash
# 1. Extract binary
./build.sh

# 2. Set credentials
export QUALYS_ACCESS_TOKEN="your-token"
export QUALYS_POD="US2"

# 3. Scan a SIF file
./scan-sif.sh /path/to/image.sif

# 4. Scan with secrets detection
./scan-sif.sh /path/to/image.sif --scan-types pkg,fileinsight,secret
```

## How It Works

```
┌─────────────────┐     ┌──────────────────┐     ┌─────────────────┐
│   SIF File      │────▶│ Extract rootfs   │────▶│ QScanner rootfs │
│   or Running    │     │ + capture uname  │     │ command         │
│   Container     │     └──────────────────┘     └────────┬────────┘
└─────────────────┘                                       │
                                                          ▼
┌─────────────────┐     ┌──────────────────┐     ┌─────────────────┐
│ Local Reports   │◀────│ Qualys Platform  │◀────│ SBOM + Inventory│
│ (SPDX, SARIF)   │     │ (Vuln Matching)  │     │ Upload          │
└─────────────────┘     └──────────────────┘     └─────────────────┘
```

1. **Extract** - SIF filesystem extracted via `apptainer exec tar`
2. **Identify** - OS detected from `/etc/os-release` + `uname` output
3. **Scan** - QScanner analyzes packages, binaries, and secrets
4. **Report** - Results uploaded to Qualys, reports generated locally

## Scan Types

| Type | Flag | Description |
|------|------|-------------|
| Package | `pkg` | OS packages (dpkg, rpm, apk) + language packages (default) |
| FileInsight | `fileinsight` | Non-package file metadata collection (default) |
| Secret | `secret` | Credential and secret detection |

```bash
# Default scan (pkg + fileinsight)
./scan-sif.sh image.sif

# Full scan with secrets
./scan-sif.sh image.sif --scan-types pkg,fileinsight,secret

# Secrets only
./scan-sif.sh image.sif --scan-types secret
```

## Scan Modes

| Mode | Flag | Description |
|------|------|-------------|
| Get Report | `--mode get-report` | Scan + upload + fetch vuln report (default) |
| Scan Only | `--mode scan-only` | Scan + upload, no report fetch |
| Inventory | `--mode inventory-only` | Local only, no backend communication |
| Policy | `--mode evaluate-policy` | Scan + evaluate against Qualys policies |

```bash
# Air-gapped: generate SBOM locally
./scan-sif.sh image.sif --mode inventory-only

# CI/CD: scan and upload only
./scan-sif.sh image.sif --mode scan-only
```

## Language Support (SCA)

QScanner detects vulnerabilities in installed packages:

| Language | Artifacts Scanned |
|----------|-------------------|
| Java | JAR, WAR, EAR files |
| Python | egg-info, dist-info, conda packages |
| Node.js | node_modules/package.json |
| Go | Compiled binaries (embedded module info) |
| Rust | Binaries with cargo-auditable |
| Ruby | .gemspec files |
| .NET | deps.json, packages.config |
| PHP | composer installed.json |

## Scripts

### scan-sif.sh

Scan Apptainer/Singularity SIF files:

```bash
./scan-sif.sh <sif-file> [qscanner-options]

# Examples
./scan-sif.sh ubuntu.sif
./scan-sif.sh ubuntu.sif --scan-types pkg,secret
./scan-sif.sh ubuntu.sif --mode inventory-only --format spdx
```

### scan-running.sh

Scan running Apptainer containers:

```bash
# List running containers
./scan-running.sh --list

# Scan by PID
./scan-running.sh 12345

# Scan by name pattern
./scan-running.sh myimage.sif
```

### Direct qscanner

```bash
# Scan any filesystem
./qscanner --pod US2 rootfs /path/to/rootfs

# Scan registry image (no Docker needed)
./qscanner --pod US2 image docker.io/nginx:latest

# Scan source code
./qscanner --pod US2 --scan-types pkg repo /path/to/code

# Full options
./qscanner --pod US2 \
    --scan-types pkg,fileinsight,secret \
    --mode get-report \
    --format json,spdx,cyclonedx \
    --report-format table,sarif \
    --output-dir ./reports \
    rootfs /tmp/extracted-sif
```

## Slurm Integration

### Single Job

```bash
sbatch --export=SCAN_TARGET=/shared/images/app.sif slurm-scan.sh
```

### Batch Scanning

```bash
# Edit targets.txt with SIF paths or image names
vim targets.txt

# Submit array job (5 concurrent max)
sbatch --array=1-$(grep -cv '^#' targets.txt)%5 slurm-batch.sh
```

**targets.txt format:**
```
# SIF files (auto-detected)
/shared/containers/ubuntu.sif
/shared/containers/python-ml.sif

# Registry images
docker.io/library/nginx:latest

# Explicit type
sif:/path/to/image.img
image:quay.io/repo/app:tag
```

## Output Formats

| Format | Flag | Description |
|--------|------|-------------|
| JSON | `--format json` | Detailed inventory |
| SPDX | `--format spdx` | SPDX 2.3 SBOM (uploaded to Qualys) |
| CycloneDX | `--format cyclonedx` | CycloneDX SBOM |
| SARIF | `--report-format sarif` | For CI/CD integration |
| Table | `--report-format table` | Console output |

## Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `QUALYS_ACCESS_TOKEN` | Yes* | Qualys API token |
| `QUALYS_POD` | Yes* | Platform: US1, US2, EU1, EU2, etc. |
| `SCAN_TYPES` | No | Default: `pkg,fileinsight` |
| `OUTPUT_DIR` | No | Report output directory |

*Not required for `--mode inventory-only`

## Architecture Support

| Architecture | Status | Binary |
|--------------|--------|--------|
| x86_64 (Intel/AMD) | ✅ Supported | `qscanner-linux-amd64.gz` |
| ARM64 | Requires binary | `qscanner-linux-arm64.gz` |
| IBM Power (ppc64le) | Contact Qualys | Not commonly available |

## Air-Gapped Environments

```bash
# Generate SBOM without network
./qscanner --mode inventory-only \
    --offline-scan \
    --format spdx,cyclonedx \
    rootfs /path/to/rootfs

# Transfer and import elsewhere
scp reports/*.spdx.json secure-system:/import/
```

## Performance

Recommended Slurm resources:
```bash
#SBATCH --mem=4G
#SBATCH --cpus-per-task=2
#SBATCH --time=00:30:00
#SBATCH --tmp=10G  # For SIF extraction
```

## Files

```
qualys-apptainer/
├── qscanner-linux-amd64.gz   # Compressed scanner binary
├── qscanner                   # Extracted binary (after build.sh)
├── build.sh                   # Setup script
├── scan-sif.sh               # SIF scanner
├── scan-running.sh           # Running container scanner
├── slurm-scan.sh             # Single Slurm job
├── slurm-batch.sh            # Batch Slurm jobs
├── targets.txt               # Batch target list
├── qscanner.def              # Apptainer definition (optional)
├── README.md                 # This file
└── blog-post.md              # Technical deep-dive
```

## License

QScanner is proprietary software requiring a Qualys subscription.
