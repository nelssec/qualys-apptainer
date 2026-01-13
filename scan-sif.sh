#!/bin/bash
# =============================================================================
# Qualys QScanner - SIF File Scanner
# =============================================================================
# Scans Apptainer/Singularity SIF images using qscanner's rootfs command.
#
# Usage:
#   ./scan-sif.sh <path-to-sif-file> [options]
#
# Examples:
#   ./scan-sif.sh myimage.sif
#   ./scan-sif.sh myimage.sif --scan-types pkg,secret
#   ./scan-sif.sh /path/to/images/*.sif  # Scan multiple SIFs
#
# Required Environment Variables:
#   QUALYS_ACCESS_TOKEN  - Qualys API token
#   QUALYS_POD           - Qualys platform (US1, US2, EU1, etc.)
#
# =============================================================================
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
QSCANNER="${SCRIPT_DIR}/qscanner"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info()  { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn()  { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }
log_step()  { echo -e "${BLUE}[STEP]${NC} $1"; }

usage() {
    cat << EOF
Usage: $(basename "$0") <sif-file> [qscanner-options]

Scan an Apptainer/Singularity SIF image for vulnerabilities.

Arguments:
  sif-file              Path to the .sif file to scan

QScanner Options (passed through):
  --scan-types          Scan types: pkg,fileinsight,secret (default: pkg,fileinsight)
  --mode                Mode: inventory-only, scan-only, get-report (default: get-report)
  --format              Output format: json,spdx,table,cyclonedx (default: db,json,spdx)
  --output-dir          Output directory for reports
  --exclude-dirs        Directories to exclude from scan

Environment Variables:
  QUALYS_ACCESS_TOKEN   Qualys API token (required)
  QUALYS_POD            Qualys platform identifier (required)
  SCAN_TYPES            Default scan types (default: pkg,fileinsight)
  OUTPUT_DIR            Default output directory

Examples:
  # Basic scan
  ./scan-sif.sh ubuntu.sif

  # Scan with secrets detection
  ./scan-sif.sh myapp.sif --scan-types pkg,fileinsight,secret

  # Inventory only (no backend upload)
  ./scan-sif.sh myapp.sif --mode inventory-only --format json

  # Scan multiple SIF files
  for sif in /images/*.sif; do ./scan-sif.sh "\$sif"; done
EOF
    exit 1
}

# Check arguments
if [[ $# -lt 1 ]] || [[ "$1" == "-h" ]] || [[ "$1" == "--help" ]]; then
    usage
fi

SIF_FILE="$1"
shift  # Remove SIF file from args, rest are passed to qscanner

# Validate SIF file
if [[ ! -f "${SIF_FILE}" ]]; then
    log_error "SIF file not found: ${SIF_FILE}"
    exit 1
fi

if [[ ! "${SIF_FILE}" == *.sif ]]; then
    log_warn "File does not have .sif extension: ${SIF_FILE}"
fi

# Check qscanner binary
if [[ ! -x "${QSCANNER}" ]]; then
    log_error "qscanner binary not found or not executable: ${QSCANNER}"
    log_error "Run ./build.sh first to extract the binary"
    exit 1
fi

# Check required environment variables
if [[ -z "${QUALYS_ACCESS_TOKEN}" ]]; then
    log_error "QUALYS_ACCESS_TOKEN environment variable required"
    exit 1
fi

if [[ -z "${QUALYS_POD}" ]]; then
    log_error "QUALYS_POD environment variable required"
    exit 1
fi

# Configuration
SIF_NAME=$(basename "${SIF_FILE}" .sif)
TEMP_ROOTFS=$(mktemp -d)
SCAN_TYPES="${SCAN_TYPES:-pkg,fileinsight}"
OUTPUT_DIR="${OUTPUT_DIR:-${SCRIPT_DIR}/reports/${SIF_NAME}}"

# Cleanup function
cleanup() {
    if [[ -d "${TEMP_ROOTFS}" ]]; then
        log_step "Cleaning up temporary files..."
        rm -rf "${TEMP_ROOTFS}"
    fi
}
trap cleanup EXIT

# Print scan info
echo ""
echo "============================================================"
echo "Qualys QScanner - SIF Image Scan"
echo "============================================================"
echo "SIF File:      ${SIF_FILE}"
echo "Image Name:    ${SIF_NAME}"
echo "Temp RootFS:   ${TEMP_ROOTFS}"
echo "Output Dir:    ${OUTPUT_DIR}"
echo "POD:           ${QUALYS_POD}"
echo "============================================================"
echo ""

# Step 1: Extract SIF contents
log_step "Extracting SIF contents to temporary rootfs..."

if command -v apptainer &>/dev/null; then
    CONTAINER_CMD="apptainer"
elif command -v singularity &>/dev/null; then
    CONTAINER_CMD="singularity"
else
    log_error "Neither apptainer nor singularity found!"
    exit 1
fi

# Extract filesystem from SIF
# Using tar to capture the full filesystem, excluding virtual filesystems
${CONTAINER_CMD} exec "${SIF_FILE}" tar -cf - \
    --exclude=/proc \
    --exclude=/sys \
    --exclude=/dev \
    --exclude=/run \
    --exclude=/tmp \
    / 2>/dev/null | tar -xf - -C "${TEMP_ROOTFS}" 2>/dev/null || true

log_info "Extracted $(find "${TEMP_ROOTFS}" -type f 2>/dev/null | wc -l | tr -d ' ') files"

# Step 2: Get OS information from the SIF
log_step "Detecting OS information..."

UNAME_OUTPUT=$(${CONTAINER_CMD} exec "${SIF_FILE}" uname -a 2>/dev/null || echo "Linux unknown")
OS_RELEASE=""
if ${CONTAINER_CMD} exec "${SIF_FILE}" cat /etc/os-release &>/dev/null; then
    OS_RELEASE=$(${CONTAINER_CMD} exec "${SIF_FILE}" cat /etc/os-release 2>/dev/null | head -5)
fi

log_info "OS: ${UNAME_OUTPUT}"

# Step 3: Create output directory
mkdir -p "${OUTPUT_DIR}"

# Step 4: Run qscanner
log_step "Running Qualys vulnerability scan..."

"${QSCANNER}" \
    --pod "${QUALYS_POD}" \
    --scan-types "${SCAN_TYPES}" \
    --shell-commands "uname -a=${UNAME_OUTPUT}" \
    --output-dir "${OUTPUT_DIR}" \
    "$@" \
    rootfs "${TEMP_ROOTFS}"

SCAN_EXIT=$?

# Summary
echo ""
echo "============================================================"
echo "Scan Complete"
echo "============================================================"
echo "Exit Code:     ${SCAN_EXIT}"
echo "Reports:       ${OUTPUT_DIR}"
echo ""

if [[ -d "${OUTPUT_DIR}" ]]; then
    log_info "Generated reports:"
    ls -la "${OUTPUT_DIR}"
fi

exit ${SCAN_EXIT}
