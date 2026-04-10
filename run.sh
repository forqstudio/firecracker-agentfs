#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

ROOTFS_DIR="${ROOTFS_DIR:-${SCRIPT_DIR}/rootfs}"
KERNEL_PATH="${KERNEL_PATH:-${SCRIPT_DIR}/linux-amazon/vmlinux}"
AGENTFS_DIR="${SCRIPT_DIR}/.agentfs"

BUILD_KERNEL_MODE="${BUILD_KERNEL_MODE:-auto}"
BUILD_ROOTFS_MODE="${BUILD_ROOTFS_MODE:-auto}"

usage() {
    cat <<'EOF'
Usage: ./run.sh [OPTIONS] [agent-id]

Builds the kernel and rootfs as needed, then starts Firecracker with AgentFS.

Options:
  --build-kernel    Force a kernel rebuild before launch
  --build-rootfs    Force a rootfs rebuild before launch
  --build-all       Force both kernel and rootfs rebuilds before launch
  -h, --help        Show this help

Environment:
  BUILD_KERNEL_MODE=auto|always|never
  BUILD_ROOTFS_MODE=auto|always|never

Notes:
  - In auto mode, missing artifacts are built automatically.
  - If you force a rootfs rebuild and an overlay already exists for the same
    agent id, use RESET_AGENTFS=1 or choose a new agent id before launch.
  - Default VM size is 4 vCPU / 8192 MiB unless overridden.
EOF
}

log() {
    echo "==> $*"
}

die() {
    echo "Error: $*" >&2
    exit 1
}

should_build() {
    local mode="$1"
    local path="$2"

    case "${mode}" in
        always) return 0 ;;
        never) return 1 ;;
        auto) [ ! -e "${path}" ] ;;
        *) die "Invalid build mode: ${mode}" ;;
    esac
}

AGENT_ID="firecracker-dev"

while [ "$#" -gt 0 ]; do
    case "$1" in
        --build-kernel)
            BUILD_KERNEL_MODE="always"
            ;;
        --build-rootfs)
            BUILD_ROOTFS_MODE="always"
            ;;
        --build-all)
            BUILD_KERNEL_MODE="always"
            BUILD_ROOTFS_MODE="always"
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        -*)
            die "Unknown option: $1"
            ;;
        *)
            AGENT_ID="$1"
            ;;
    esac
    shift
done

ROOTFS_SENTINEL="${ROOTFS_DIR}/etc/agentvm-release"
OVERLAY_DB="${AGENTFS_DIR}/${AGENT_ID}.db"

if should_build "${BUILD_KERNEL_MODE}" "${KERNEL_PATH}"; then
    log "Building kernel"
    "${SCRIPT_DIR}/build-kernel.sh"
else
    log "Reusing existing kernel at ${KERNEL_PATH}"
fi

ROOTFS_REBUILT=0
if should_build "${BUILD_ROOTFS_MODE}" "${ROOTFS_SENTINEL}"; then
    log "Building rootfs"
    "${SCRIPT_DIR}/build-rootfs.sh"
    ROOTFS_REBUILT=1
else
    log "Reusing existing rootfs at ${ROOTFS_DIR}"
fi

if [ "${ROOTFS_REBUILT}" = "1" ] && [ -f "${OVERLAY_DB}" ] && [ "${RESET_AGENTFS:-0}" != "1" ]; then
    die "Rootfs was rebuilt and overlay ${AGENT_ID} already exists. Re-run with RESET_AGENTFS=1 or use a new agent id."
fi

log "Starting Firecracker with AgentFS overlay ${AGENT_ID}"
exec "${SCRIPT_DIR}/firecracker.sh" "${AGENT_ID}"
