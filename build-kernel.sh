#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
KERNEL_DIR="${KERNEL_DIR:-${SCRIPT_DIR}/linux-amazon}"
KERNEL_TAG="${KERNEL_TAG:-microvm-kernel-6.1.167-27.319.amzn2023}"

log() {
    echo "==> $*"
}

install_build_deps() {
    log "Installing kernel build dependencies"
    sudo apt-get update -qq
    sudo apt-get install -y -qq \
        bc bison build-essential cpio flex kmod libelf-dev libssl-dev rsync tar xz-utils >/dev/null
}

prepare_sources() {
    if [ ! -d "${KERNEL_DIR}" ]; then
        log "Cloning Amazon Linux kernel ${KERNEL_TAG}"
        git clone --depth 1 --branch "${KERNEL_TAG}" https://github.com/amazonlinux/linux.git "${KERNEL_DIR}"
        return
    fi

    if [ ! -d "${KERNEL_DIR}/.git" ]; then
        echo "Error: ${KERNEL_DIR} exists but is not a git checkout." >&2
        exit 1
    fi

    local current_ref
    current_ref="$(git -C "${KERNEL_DIR}" describe --tags --exact-match 2>/dev/null || echo unknown)"
    if [ "${current_ref}" != "${KERNEL_TAG}" ]; then
        if [ -n "$(git -C "${KERNEL_DIR}" status --short --untracked-files=no)" ]; then
            echo "Error: ${KERNEL_DIR} has local changes and is on ${current_ref}, but ${KERNEL_TAG} is requested." >&2
            echo "Commit, stash, or discard changes before switching kernel tags." >&2
            exit 1
        fi

        log "Switching existing kernel checkout from ${current_ref} to ${KERNEL_TAG}"
        git -C "${KERNEL_DIR}" fetch --depth 1 origin "refs/tags/${KERNEL_TAG}:refs/tags/${KERNEL_TAG}"
        git -C "${KERNEL_DIR}" checkout -f "tags/${KERNEL_TAG}"
    fi
}

configure_kernel() {
    log "Configuring kernel for Firecracker + developer tooling"
    if [ -f .config ]; then
        echo "Using existing .config"
    else
        make x86_64_defconfig
    fi

    ./scripts/config --disable SYSTEM_TRUSTED_KEYRING
    ./scripts/config --disable SECONDARY_TRUSTED_KEYRING
    ./scripts/config --disable SYSTEM_REVOCATION_KEYS
    ./scripts/config --disable MODULE_SIG
    ./scripts/config --disable INTEGRITY
    ./scripts/config --disable IMA
    ./scripts/config --disable EVM
    ./scripts/config --set-str SYSTEM_TRUSTED_KEYS ""
    ./scripts/config --set-str SYSTEM_REVOCATION_KEYS ""

    ./scripts/config --enable VIRTIO_MMIO_CMDLINE_DEVICES
    ./scripts/config --enable VIRTIO
    ./scripts/config --enable VIRTIO_MMIO
    ./scripts/config --enable VIRTIO_NET
    ./scripts/config --disable BLK_DEV_INTEGRITY

    ./scripts/config --enable DEVTMPFS
    ./scripts/config --enable DEVTMPFS_MOUNT
    ./scripts/config --enable TMPFS

    ./scripts/config --enable IP_PNP
    ./scripts/config --enable IP_PNP_DHCP
    ./scripts/config --enable IP_PNP_BOOTP
    ./scripts/config --enable IP_PNP_RARP

    ./scripts/config --enable NFS_FS
    ./scripts/config --enable ROOT_NFS
    ./scripts/config --enable NFS_V3
    ./scripts/config --enable NFS_V3_ACL
    ./scripts/config --enable LOCKD
    ./scripts/config --enable LOCKD_V4

    ./scripts/config --enable BINFMT_MISC
    ./scripts/config --enable USER_NS
    ./scripts/config --enable PID_NS
    ./scripts/config --enable NET_NS
    ./scripts/config --enable IPC_NS
    ./scripts/config --enable UTS_NS
    ./scripts/config --enable CGROUPS
    ./scripts/config --enable CGROUP_PIDS
    ./scripts/config --enable CGROUP_FREEZER
    ./scripts/config --enable CPUSETS
    ./scripts/config --enable BPF_SYSCALL
    ./scripts/config --enable FANOTIFY
    ./scripts/config --enable INOTIFY_USER
    ./scripts/config --enable FUSE_FS
    ./scripts/config --enable OVERLAY_FS
    ./scripts/config --enable SECCOMP
    ./scripts/config --disable DEBUG_STACK_USAGE

    make olddefconfig
}

build_kernel() {
    log "Building vmlinux"
    make -j"$(nproc)" vmlinux CC="gcc -std=gnu11"
}

install_build_deps
prepare_sources

cd "${KERNEL_DIR}"
configure_kernel
build_kernel

cd "${SCRIPT_DIR}"
ln -sf "${KERNEL_DIR}/vmlinux" vmlinux
log "Done. Kernel available at ${KERNEL_DIR}/vmlinux"
