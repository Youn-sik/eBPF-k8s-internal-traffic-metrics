#!/usr/bin/env bash
# Generate vmlinux.h and bpf2go bindings for tcp_connect.c.
# Must be run on a Linux host with BTF-enabled kernel (e.g., /sys/kernel/btf/vmlinux present),
# clang/llvm installed, and bpftool available in PATH.
#
# Usage:
#   ARCH=x86 ./gen_bpf.sh   # for x86_64 nodes
#   ARCH=arm64 ./gen_bpf.sh # for arm64 nodes
#
# Outputs:
#   ebpf/vmlinux.h
#   ebpf/tcp_connect_bpfel.go (little-endian)
#   ebpf/tcp_connect_bpfeb.go (big-endian; optional but included for portability)

set -euo pipefail

ARCH="${ARCH:-x86}"
PKG="${GOPACKAGE:-ebpf}"

if [[ "$(uname -s)" != "Linux" ]]; then
  echo "This script must run on Linux with BTF-enabled kernel." >&2
  exit 1
fi

if ! command -v bpftool >/dev/null 2>&1; then
  echo "bpftool not found in PATH; install it first." >&2
  exit 1
fi

if [[ ! -f "/sys/kernel/btf/vmlinux" ]]; then
  echo "/sys/kernel/btf/vmlinux not found; ensure kernel was built with BTF." >&2
  exit 1
fi

echo "[1/2] Dumping vmlinux.h from kernel BTF..."
bpftool btf dump file /sys/kernel/btf/vmlinux format c > "$(dirname "$0")/vmlinux.h"

if ! command -v bpf2go >/dev/null 2>&1; then
  echo "bpf2go not found; install via: go install github.com/cilium/ebpf/cmd/bpf2go@latest" >&2
  exit 1
fi

echo "[2/2] Running bpf2go (arch=${ARCH}, pkg=${PKG})..."
bpf2go -cc clang -target bpfel -go-package "${PKG}" -cflags "-g -O2 -D__TARGET_ARCH_${ARCH}" TcpConnect tcp_connect.c -- -I"$(dirname "$0")"
bpf2go -cc clang -target bpfeb -go-package "${PKG}" -cflags "-g -O2 -D__TARGET_ARCH_${ARCH}" TcpConnect tcp_connect.c -- -I"$(dirname "$0")"

echo "Done. Generated tcp_connect_bpfel.go / tcp_connect_bpfeb.go and vmlinux.h under $(dirname "$0")."
