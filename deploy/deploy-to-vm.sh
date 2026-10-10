#!/usr/bin/env bash
# 在 macOS 宿主机执行：重新构建发布包并部署/更新到 Parallels Ubuntu 虚拟机。
#
# 用法：
#   ./deploy/deploy-to-vm.sh            # 默认端口 8091
#   ./deploy/deploy-to-vm.sh 8095       # 指定端口（仅首次安装生效）
#
# 依赖：prlctl（Parallels Desktop）、虚拟机已开机、共享目录已开启（/Users/zsh -> /media/psf/Home）
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "${SCRIPT_DIR}")"
VM_UUID="${VM_UUID:-38a1cdc4-6d41-4d39-b331-e7cef3af8745}"
PORT="${1:-8091}"

# /Users/zsh -> /media/psf/Home 的路径映射
VM_ROOT="/media/psf/Home${ROOT_DIR#/Users/zsh}"

command -v prlctl >/dev/null 2>&1 || { echo "未找到 prlctl（需要 Parallels Desktop）" >&2; exit 1; }

echo "==> 1/2 构建 arm64 发布包"
cd "${ROOT_DIR}"
./build.sh package

echo "==> 2/2 部署到虚拟机 ${VM_UUID}（端口 ${PORT}）"
prlctl exec "${VM_UUID}" /bin/bash "${VM_ROOT}/deploy/vm-install.sh" "${PORT}"
