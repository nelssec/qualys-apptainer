#!/bin/bash
#SBATCH --job-name=qscanner-batch
#SBATCH --output=logs/qscanner-%A_%a.out
#SBATCH --error=logs/qscanner-%A_%a.err
#SBATCH --time=00:30:00
#SBATCH --mem=4G
#SBATCH --cpus-per-task=2
#SBATCH --partition=standard
#SBATCH --array=1-10%5

# =============================================================================
# Qualys QScanner Batch Scan - Slurm Array Job
# =============================================================================
#
# Scans multiple targets from a list file using Slurm array jobs.
# Supports SIF files, registry images, and mixed targets.
#
# Usage:
#   1. Create targets.txt with one target per line:
#      /path/to/image1.sif
#      /path/to/image2.sif
#      docker.io/nginx:latest
#
#   2. Submit array job:
#      sbatch --array=1-$(wc -l < targets.txt) slurm-batch.sh
#
#   3. Or with throttling (5 concurrent):
#      sbatch --array=1-$(wc -l < targets.txt)%5 slurm-batch.sh
#
# Required:
#   QUALYS_ACCESS_TOKEN, QUALYS_POD environment variables
#   targets.txt (or TARGETS_FILE) with list of targets
#
# Target Format:
#   - SIF files: /path/to/image.sif (auto-detected by .sif extension)
#   - Registry images: docker.io/nginx:latest (anything without .sif)
#   - Force type: sif:/path/to/file or image:nginx:latest
#
# =============================================================================

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
QSCANNER="${QSCANNER:-${SCRIPT_DIR}/qscanner}"
TARGETS_FILE="${TARGETS_FILE:-${SCRIPT_DIR}/targets.txt}"
SCAN_TYPES="${SCAN_TYPES:-pkg,fileinsight}"
BASE_OUTPUT_DIR="${OUTPUT_DIR:-${SCRIPT_DIR}/reports}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info()  { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn()  { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

# Get the target for this array task
if [[ ! -f "${TARGETS_FILE}" ]]; then
    log_error "Targets file not found: ${TARGETS_FILE}"
    exit 1
fi

TARGET=$(sed -n "${SLURM_ARRAY_TASK_ID}p" "${TARGETS_FILE}")

if [[ -z "${TARGET}" ]]; then
    log_error "No target found at line ${SLURM_ARRAY_TASK_ID}"
    exit 1
fi

# Determine scan type from target
if [[ "${TARGET}" == sif:* ]]; then
    SCAN_TYPE="sif"
    TARGET="${TARGET#sif:}"
elif [[ "${TARGET}" == image:* ]]; then
    SCAN_TYPE="image"
    TARGET="${TARGET#image:}"
elif [[ "${TARGET}" == repo:* ]]; then
    SCAN_TYPE="repo"
    TARGET="${TARGET#repo:}"
elif [[ "${TARGET}" == *.sif ]]; then
    SCAN_TYPE="sif"
else
    SCAN_TYPE="image"
fi

# Create unique output directory for this target
SAFE_NAME=$(echo "${TARGET}" | tr '/:' '_' | sed 's/^_//')
OUTPUT_DIR="${BASE_OUTPUT_DIR}/${SLURM_ARRAY_JOB_ID}/${SAFE_NAME}"
mkdir -p "${OUTPUT_DIR}"
mkdir -p "${SCRIPT_DIR}/logs"

echo "============================================================"
echo "Batch Scan - Task ${SLURM_ARRAY_TASK_ID}/${SLURM_ARRAY_TASK_COUNT:-?}"
echo "============================================================"
echo "Array Job ID:  ${SLURM_ARRAY_JOB_ID}"
echo "Task ID:       ${SLURM_ARRAY_TASK_ID}"
echo "Scan Type:     ${SCAN_TYPE}"
echo "Target:        ${TARGET}"
echo "Output:        ${OUTPUT_DIR}"
echo "============================================================"

# Validate
if [[ -z "${QUALYS_ACCESS_TOKEN}" ]] || [[ -z "${QUALYS_POD}" ]]; then
    log_error "QUALYS_ACCESS_TOKEN and QUALYS_POD required"
    exit 1
fi

if [[ ! -x "${QSCANNER}" ]]; then
    log_error "qscanner binary not found: ${QSCANNER}"
    exit 1
fi

# Run scan based on type
case "${SCAN_TYPE}" in
    sif)
        if [[ ! -f "${TARGET}" ]]; then
            log_error "SIF file not found: ${TARGET}"
            exit 1
        fi

        TEMP_ROOTFS=$(mktemp -d)
        trap "rm -rf ${TEMP_ROOTFS}" EXIT

        # Find container command
        if command -v apptainer &>/dev/null; then
            CONTAINER_CMD="apptainer"
        elif command -v singularity &>/dev/null; then
            CONTAINER_CMD="singularity"
        else
            log_error "Neither apptainer nor singularity found"
            exit 1
        fi

        log_info "Extracting SIF filesystem..."
        ${CONTAINER_CMD} exec "${TARGET}" tar -cf - \
            --exclude=/proc --exclude=/sys --exclude=/dev --exclude=/run \
            / 2>/dev/null | tar -xf - -C "${TEMP_ROOTFS}" 2>/dev/null || true

        UNAME_OUTPUT=$(${CONTAINER_CMD} exec "${TARGET}" uname -a 2>/dev/null || echo "Linux unknown")

        log_info "Scanning rootfs..."
        "${QSCANNER}" \
            --pod "${QUALYS_POD}" \
            --scan-types "${SCAN_TYPES}" \
            --shell-commands "uname -a=${UNAME_OUTPUT}" \
            --output-dir "${OUTPUT_DIR}" \
            --format json,spdx,sarif \
            rootfs "${TEMP_ROOTFS}"
        ;;

    image)
        log_info "Scanning image..."
        "${QSCANNER}" \
            --pod "${QUALYS_POD}" \
            --scan-types "${SCAN_TYPES}" \
            --output-dir "${OUTPUT_DIR}" \
            --format json,spdx,sarif \
            image "${TARGET}"
        ;;

    repo)
        log_info "Scanning repository..."
        "${QSCANNER}" \
            --pod "${QUALYS_POD}" \
            --scan-types "${SCAN_TYPES}" \
            --output-dir "${OUTPUT_DIR}" \
            --format json,spdx,sarif \
            repo "${TARGET}"
        ;;
esac

EXIT_CODE=$?

echo ""
log_info "Task ${SLURM_ARRAY_TASK_ID} complete: ${TARGET} (exit: ${EXIT_CODE})"

exit ${EXIT_CODE}
