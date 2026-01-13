#!/bin/bash
#SBATCH --job-name=qscanner
#SBATCH --output=qscanner-%j.out
#SBATCH --error=qscanner-%j.err
#SBATCH --time=01:00:00
#SBATCH --mem=4G
#SBATCH --cpus-per-task=2
#SBATCH --partition=standard

# =============================================================================
# Qualys QScanner Slurm Job Script
# =============================================================================
#
# Supports scanning:
#   - SIF files (native Apptainer images)
#   - Registry images (Docker Hub, etc.)
#   - Source code repositories
#
# Usage:
#   # Scan a SIF file
#   sbatch --export=SCAN_TARGET=/path/to/image.sif slurm-scan.sh
#
#   # Scan a registry image
#   sbatch --export=SCAN_TARGET=nginx:latest,SCAN_TYPE=image slurm-scan.sh
#
#   # Scan source code
#   sbatch --export=SCAN_TARGET=/path/to/repo,SCAN_TYPE=repo slurm-scan.sh
#
# Required environment variables (set in ~/.bashrc or via --export):
#   QUALYS_ACCESS_TOKEN  - Your Qualys API token
#   QUALYS_POD           - Qualys platform (e.g., US2)
#
# Optional:
#   SCAN_TYPE            - "sif" (default), "image", "repo"
#   SCAN_TARGET          - Path or image name to scan
#   SCAN_TYPES           - Scan types: pkg,fileinsight,secret (default: pkg,fileinsight)
#
# =============================================================================

set -e

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
QSCANNER="${QSCANNER:-${SCRIPT_DIR}/qscanner}"
SCAN_TYPE="${SCAN_TYPE:-sif}"
SCAN_TARGET="${SCAN_TARGET:-}"
SCAN_TYPES="${SCAN_TYPES:-pkg,fileinsight}"
OUTPUT_DIR="${OUTPUT_DIR:-${SCRIPT_DIR}/reports/${SLURM_JOB_ID}}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info()  { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn()  { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

# Print job info
echo "============================================================"
echo "Qualys QScanner - Slurm Job"
echo "============================================================"
echo "Job ID:        ${SLURM_JOB_ID}"
echo "Node:          ${SLURMD_NODENAME}"
echo "Start time:    $(date)"
echo "Scan type:     ${SCAN_TYPE}"
echo "Target:        ${SCAN_TARGET}"
echo "Scan types:    ${SCAN_TYPES}"
echo "Output:        ${OUTPUT_DIR}"
echo "============================================================"

# Validate requirements
if [[ -z "${QUALYS_ACCESS_TOKEN}" ]]; then
    log_error "QUALYS_ACCESS_TOKEN not set"
    exit 1
fi

if [[ -z "${QUALYS_POD}" ]]; then
    log_error "QUALYS_POD not set"
    exit 1
fi

if [[ ! -x "${QSCANNER}" ]]; then
    log_error "qscanner binary not found: ${QSCANNER}"
    log_error "Run ./build.sh first"
    exit 1
fi

if [[ -z "${SCAN_TARGET}" ]]; then
    log_error "SCAN_TARGET not set"
    exit 1
fi

# Create output directory
mkdir -p "${OUTPUT_DIR}"

# Determine scan command based on type
case "${SCAN_TYPE}" in
    sif)
        # Scanning a SIF file - extract and use rootfs
        if [[ ! -f "${SCAN_TARGET}" ]]; then
            log_error "SIF file not found: ${SCAN_TARGET}"
            exit 1
        fi

        SIF_NAME=$(basename "${SCAN_TARGET}" .sif)
        TEMP_ROOTFS=$(mktemp -d)

        log_info "Extracting SIF: ${SCAN_TARGET}"

        # Find apptainer/singularity
        if command -v apptainer &>/dev/null; then
            CONTAINER_CMD="apptainer"
        elif command -v singularity &>/dev/null; then
            CONTAINER_CMD="singularity"
        else
            log_error "Neither apptainer nor singularity found"
            exit 1
        fi

        # Extract filesystem
        ${CONTAINER_CMD} exec "${SCAN_TARGET}" tar -cf - \
            --exclude=/proc --exclude=/sys --exclude=/dev --exclude=/run \
            / 2>/dev/null | tar -xf - -C "${TEMP_ROOTFS}" 2>/dev/null || true

        # Get OS info
        UNAME_OUTPUT=$(${CONTAINER_CMD} exec "${SCAN_TARGET}" uname -a 2>/dev/null || echo "Linux unknown")

        log_info "Running rootfs scan..."
        "${QSCANNER}" \
            --pod "${QUALYS_POD}" \
            --scan-types "${SCAN_TYPES}" \
            --shell-commands "uname -a=${UNAME_OUTPUT}" \
            --output-dir "${OUTPUT_DIR}" \
            --format json,spdx,sarif \
            rootfs "${TEMP_ROOTFS}"

        EXIT_CODE=$?

        # Cleanup
        rm -rf "${TEMP_ROOTFS}"
        ;;

    image)
        # Scanning a registry image
        log_info "Running image scan..."
        "${QSCANNER}" \
            --pod "${QUALYS_POD}" \
            --scan-types "${SCAN_TYPES}" \
            --output-dir "${OUTPUT_DIR}" \
            --format json,spdx,sarif \
            image "${SCAN_TARGET}"

        EXIT_CODE=$?
        ;;

    repo)
        # Scanning source code
        if [[ ! -d "${SCAN_TARGET}" ]]; then
            log_error "Repository not found: ${SCAN_TARGET}"
            exit 1
        fi

        log_info "Running repository scan..."
        "${QSCANNER}" \
            --pod "${QUALYS_POD}" \
            --scan-types "${SCAN_TYPES}" \
            --output-dir "${OUTPUT_DIR}" \
            --format json,spdx,sarif \
            repo "${SCAN_TARGET}"

        EXIT_CODE=$?
        ;;

    *)
        log_error "Unknown scan type: ${SCAN_TYPE}"
        log_error "Valid types: sif, image, repo"
        exit 1
        ;;
esac

# Summary
echo ""
echo "============================================================"
echo "Scan Complete"
echo "============================================================"
echo "Exit code:     ${EXIT_CODE}"
echo "End time:      $(date)"
echo "Reports:       ${OUTPUT_DIR}"
echo ""

if [[ -d "${OUTPUT_DIR}" ]]; then
    log_info "Generated reports:"
    ls -la "${OUTPUT_DIR}"
fi

exit ${EXIT_CODE}
