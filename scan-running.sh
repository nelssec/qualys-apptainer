#!/bin/bash
# =============================================================================
# Qualys QScanner - Running Container Scanner
# =============================================================================
# Scans running Apptainer/Singularity containers by accessing their filesystem.
#
# Usage:
#   ./scan-running.sh <pid|name> [options]
#
# Examples:
#   ./scan-running.sh 12345                    # Scan by PID
#   ./scan-running.sh myimage.sif              # Scan by image name (finds PID)
#   ./scan-running.sh --list                   # List running containers
#
# Required Environment Variables:
#   QUALYS_ACCESS_TOKEN  - Qualys API token
#   QUALYS_POD           - Qualys platform (US1, US2, EU1, etc.)
#
# Note: Requires appropriate permissions to access /proc/<pid>/root
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
Usage: $(basename "$0") <pid|name|--list> [qscanner-options]

Scan a running Apptainer/Singularity container for vulnerabilities.

Arguments:
  pid                   Process ID of the running container
  name                  Image name pattern to find (e.g., "myimage.sif")
  --list                List all running Apptainer/Singularity containers

QScanner Options (passed through):
  --scan-types          Scan types: pkg,fileinsight,secret (default: pkg,fileinsight)
  --mode                Mode: inventory-only, scan-only, get-report
  --format              Output format: json,spdx,table,cyclonedx
  --output-dir          Output directory for reports

Environment Variables:
  QUALYS_ACCESS_TOKEN   Qualys API token (required for scanning)
  QUALYS_POD            Qualys platform identifier (required for scanning)

Examples:
  # List running containers
  ./scan-running.sh --list

  # Scan by PID
  ./scan-running.sh 12345

  # Scan by image name
  ./scan-running.sh myapp.sif

  # Scan with secrets detection
  ./scan-running.sh 12345 --scan-types pkg,fileinsight,secret

Notes:
  - Requires root or same user permissions to access /proc/<pid>/root
  - On HPC systems, you may need to run this on the same node as the container
EOF
    exit 1
}

list_containers() {
    echo ""
    echo "Running Apptainer/Singularity Containers"
    echo "========================================="
    echo ""

    # Find apptainer/singularity processes
    local found=0

    while IFS= read -r line; do
        if [[ -n "$line" ]]; then
            found=1
            pid=$(echo "$line" | awk '{print $2}')
            user=$(echo "$line" | awk '{print $1}')
            cmd=$(echo "$line" | awk '{for(i=11;i<=NF;i++) printf $i" "; print ""}')

            echo "PID: ${pid}"
            echo "  User:    ${user}"
            echo "  Command: ${cmd}"

            # Try to get more info about the container
            if [[ -r "/proc/${pid}/root" ]]; then
                echo "  RootFS:  /proc/${pid}/root (accessible)"
            else
                echo "  RootFS:  /proc/${pid}/root (permission denied)"
            fi
            echo ""
        fi
    done < <(ps aux | grep -E "(apptainer|singularity|Singularity)" | grep -v grep | grep -v "scan-running.sh")

    if [[ $found -eq 0 ]]; then
        log_warn "No running Apptainer/Singularity containers found"
    fi

    exit 0
}

find_pid_by_name() {
    local name="$1"
    local pids

    pids=$(pgrep -f "${name}" 2>/dev/null | head -1)

    if [[ -z "$pids" ]]; then
        return 1
    fi

    echo "$pids"
}

# Check arguments
if [[ $# -lt 1 ]] || [[ "$1" == "-h" ]] || [[ "$1" == "--help" ]]; then
    usage
fi

# Handle --list
if [[ "$1" == "--list" ]]; then
    list_containers
fi

TARGET="$1"
shift  # Remove target from args, rest are passed to qscanner

# Determine if target is PID or name
if [[ "$TARGET" =~ ^[0-9]+$ ]]; then
    PID="$TARGET"
else
    log_step "Searching for container matching: ${TARGET}"
    PID=$(find_pid_by_name "$TARGET")
    if [[ -z "$PID" ]]; then
        log_error "No running container found matching: ${TARGET}"
        log_info "Use --list to see running containers"
        exit 1
    fi
    log_info "Found container PID: ${PID}"
fi

# Validate PID exists
if [[ ! -d "/proc/${PID}" ]]; then
    log_error "Process ${PID} does not exist"
    exit 1
fi

# Check if we can access the rootfs
ROOTFS="/proc/${PID}/root"
if [[ ! -r "${ROOTFS}" ]]; then
    log_error "Cannot access ${ROOTFS}"
    log_error "You may need root privileges or to be the same user as the container"
    exit 1
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
SCAN_TYPES="${SCAN_TYPES:-pkg,fileinsight}"
OUTPUT_DIR="${OUTPUT_DIR:-${SCRIPT_DIR}/reports/pid-${PID}}"

# Get process info
PROC_CMD=$(cat /proc/${PID}/cmdline 2>/dev/null | tr '\0' ' ' | head -c 100)
PROC_USER=$(stat -c '%U' /proc/${PID} 2>/dev/null || stat -f '%Su' /proc/${PID} 2>/dev/null || echo "unknown")

# Print scan info
echo ""
echo "============================================================"
echo "Qualys QScanner - Running Container Scan"
echo "============================================================"
echo "PID:           ${PID}"
echo "User:          ${PROC_USER}"
echo "Command:       ${PROC_CMD}..."
echo "RootFS:        ${ROOTFS}"
echo "Output Dir:    ${OUTPUT_DIR}"
echo "POD:           ${QUALYS_POD}"
echo "============================================================"
echo ""

# Get OS information from the container
log_step "Detecting OS information..."

# Try to read os-release from container's rootfs
UNAME_OUTPUT="Linux unknown"
if [[ -f "${ROOTFS}/etc/os-release" ]]; then
    . "${ROOTFS}/etc/os-release" 2>/dev/null || true
    UNAME_OUTPUT="Linux ${PRETTY_NAME:-unknown}"
fi

log_info "OS: ${UNAME_OUTPUT}"

# Create output directory
mkdir -p "${OUTPUT_DIR}"

# Run qscanner
log_step "Running Qualys vulnerability scan..."

"${QSCANNER}" \
    --pod "${QUALYS_POD}" \
    --scan-types "${SCAN_TYPES}" \
    --shell-commands "uname -a=${UNAME_OUTPUT}" \
    --output-dir "${OUTPUT_DIR}" \
    --exclude-dirs /proc,/sys,/dev,/run \
    "$@" \
    rootfs "${ROOTFS}"

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
