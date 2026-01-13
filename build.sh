#!/bin/bash
# =============================================================================
# Qualys QScanner - Build/Setup Script
# =============================================================================
# Extracts qscanner binary for your architecture.
#
# Usage:
#   ./build.sh              # Extract binary for current architecture
#   ./build.sh --sif        # Also build Apptainer SIF image
#   ./build.sh --check      # Check system compatibility
#
# Supported Architectures:
#   - linux/amd64 (x86_64)  : Intel/AMD processors
#   - linux/arm64           : ARM64 processors (needs separate binary)
#   - linux/ppc64le         : IBM Power (needs separate binary)
#
# =============================================================================
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "${SCRIPT_DIR}"

# Configuration
QSCANNER_VERSION="4.8.0"

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

# Detect architecture
detect_arch() {
    local arch=$(uname -m)
    case "${arch}" in
        x86_64|amd64)
            echo "amd64"
            ;;
        aarch64|arm64)
            echo "arm64"
            ;;
        ppc64le)
            echo "ppc64le"
            ;;
        *)
            echo "unknown"
            ;;
    esac
}

# Check system compatibility
check_system() {
    echo ""
    echo "============================================================"
    echo "System Compatibility Check"
    echo "============================================================"
    echo ""

    local os=$(uname -s)
    local arch=$(uname -m)
    local detected=$(detect_arch)

    echo "Operating System: ${os}"
    echo "Architecture:     ${arch} (${detected})"
    echo ""

    if [[ "${os}" != "Linux" ]]; then
        log_warn "QScanner requires Linux. Current OS: ${os}"
        log_warn "For macOS/Windows, use Docker or a Linux VM"
        return 1
    fi

    case "${detected}" in
        amd64)
            if [[ -f "qscanner-linux-amd64.gz" ]]; then
                log_info "Binary available: qscanner-linux-amd64.gz"
                return 0
            else
                log_error "Missing binary: qscanner-linux-amd64.gz"
                return 1
            fi
            ;;
        arm64)
            if [[ -f "qscanner-linux-arm64.gz" ]]; then
                log_info "Binary available: qscanner-linux-arm64.gz"
                return 0
            else
                log_error "ARM64 binary not included"
                log_error "Download qscanner-linux-arm64.gz from Qualys"
                return 1
            fi
            ;;
        ppc64le)
            if [[ -f "qscanner-linux-ppc64le.gz" ]]; then
                log_info "Binary available: qscanner-linux-ppc64le.gz"
                return 0
            else
                log_error "IBM Power (ppc64le) binary not included"
                log_error "Contact Qualys for ppc64le build availability"
                log_error ""
                log_error "Alternative: Run on x86_64 login/service node and"
                log_error "mount shared filesystem to scan SIF files"
                return 1
            fi
            ;;
        *)
            log_error "Unsupported architecture: ${arch}"
            return 1
            ;;
    esac
}

# Main
echo ""
echo "============================================================"
echo "Qualys QScanner for Apptainer - Setup"
echo "============================================================"
echo "Version: ${QSCANNER_VERSION}"
echo ""

# Handle --check flag
if [[ "$1" == "--check" ]]; then
    check_system
    exit $?
fi

# Detect architecture
ARCH=$(detect_arch)
BINARY_GZ="qscanner-linux-${ARCH}.gz"

log_step "Detecting system architecture..."
log_info "Architecture: ${ARCH}"

# Check if binary exists for this architecture
if [[ ! -f "${BINARY_GZ}" ]]; then
    log_error "Binary not found for architecture: ${ARCH}"
    log_error "Expected file: ${BINARY_GZ}"
    echo ""

    case "${ARCH}" in
        ppc64le)
            log_warn "IBM Power (ppc64le) is not commonly supported by QScanner"
            log_warn "Options:"
            log_warn "  1. Contact Qualys about ppc64le build availability"
            log_warn "  2. Run scans from x86_64 login node with shared filesystem"
            log_warn "  3. Use cross-architecture container (if available)"
            ;;
        arm64)
            log_warn "Download the ARM64 binary from Qualys and rename to:"
            log_warn "  qscanner-linux-arm64.gz"
            ;;
        *)
            log_error "Unsupported architecture"
            ;;
    esac
    exit 1
fi

# Extract binary
log_step "Extracting qscanner binary..."

gunzip -k -f "${BINARY_GZ}"
mv "qscanner-linux-${ARCH}" qscanner 2>/dev/null || mv "${BINARY_GZ%.gz}" qscanner
chmod +x qscanner

# Verify binary
if ./qscanner --version &>/dev/null; then
    EXTRACTED_VERSION=$(./qscanner --version 2>&1 | head -1)
    log_info "Extracted: ${EXTRACTED_VERSION}"
else
    log_warn "Cannot verify binary version (may need to run on Linux)"
fi

log_info "Binary ready: ${SCRIPT_DIR}/qscanner"

# Build SIF image (optional)
if [[ "$1" == "--sif" ]]; then
    log_step "Building Apptainer SIF image..."

    if command -v apptainer &>/dev/null; then
        CONTAINER_CMD="apptainer"
    elif command -v singularity &>/dev/null; then
        CONTAINER_CMD="singularity"
    else
        log_error "Neither apptainer nor singularity found!"
        exit 1
    fi

    # Prepare binary for SIF
    cp "${BINARY_GZ}" qscanner-sif.gz

    sudo ${CONTAINER_CMD} build qscanner.sif qscanner.def

    rm -f qscanner-sif.gz
    log_info "SIF image built: ${SCRIPT_DIR}/qscanner.sif"
fi

# Summary
echo ""
echo "============================================================"
echo "Setup Complete"
echo "============================================================"
echo ""
echo "Quick Start:"
echo ""
echo "  1. Set credentials:"
echo "     export QUALYS_ACCESS_TOKEN='your-token'"
echo "     export QUALYS_POD='US2'"
echo ""
echo "  2. Scan a SIF file:"
echo "     ./scan-sif.sh /path/to/image.sif"
echo ""
echo "  3. Scan running containers:"
echo "     ./scan-running.sh --list"
echo "     ./scan-running.sh <pid>"
echo ""
echo "  4. Direct qscanner usage:"
echo "     ./qscanner rootfs /path/to/rootfs"
echo "     ./qscanner --help"
echo ""
echo "For Slurm/HPC:"
echo "  sbatch --export=SCAN_TARGET=/path/to/image.sif slurm-scan.sh"
echo ""
