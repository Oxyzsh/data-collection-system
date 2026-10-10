#!/usr/bin/env bash
# 数据采集系统构建脚本（仅后端，无前端资源）。
#
# 用法：
#   ./build.sh             # 构建前端 + 两个 linux 架构
#   ./build.sh local       # 构建前端 + 当前平台（开发调试）
#   ./build.sh frontend    # 只构建前端（产物复制到 cmd/server/dist）
#   ./build.sh x86         # 编译 linux/amd64
#   ./build.sh arm64       # 编译 linux/arm64（RK3562 等板卡）
#   ./build.sh all         # x86 + arm64（默认）
#   ./build.sh clean       # 清理构建产物
#
# 环境变量：
#   SKIP_FRONTEND=1        # 跳过前端构建，直接使用 cmd/server/dist 里已有产物
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "${SCRIPT_DIR}"
OUTPUT_DIR="${SCRIPT_DIR}/build"
BUILD_VERSION="$(cat "${SCRIPT_DIR}/__version" 2>/dev/null || echo 0.1.0)"
LDFLAGS="-X data-collection-system/internal/shared/buildinfo.Version=${BUILD_VERSION}"

copy_device_config() {
  # device.yaml 的固定查找位置是 <程序目录>/config/device.yaml，
  # 因此构建后把示例配置复制到二进制旁边（与发布包布局一致）。
  [ -f "${SCRIPT_DIR}/config/device.yaml" ] || {
    echo "缺少 config/device.yaml" >&2
    exit 1
  }
  mkdir -p "${OUTPUT_DIR}/config"
  cp "${SCRIPT_DIR}/config/device.yaml" "${OUTPUT_DIR}/config/device.yaml"
}

# 构建前端（Vue3 + Vite），并把产物复制到后端 embed 目录。
build_frontend() {
  if [ "${SKIP_FRONTEND:-0}" = "1" ]; then
    echo "==> SKIP_FRONTEND=1，跳过前端构建"
    return 0
  fi
  if ! command -v npm >/dev/null 2>&1; then
    echo "==> 未找到 npm，跳过前端构建（使用 cmd/server/dist 现有产物）" >&2
    return 0
  fi

  echo "==> 构建前端（frontend/）"
  (
    cd "${SCRIPT_DIR}/frontend"
    if [ ! -d node_modules ]; then
      npm install --no-audit --no-fund
    fi
    npm run build
  )

  mkdir -p "${SCRIPT_DIR}/cmd/server/dist"
  if command -v rsync >/dev/null 2>&1; then
    rsync -a --delete --exclude='.keep' "${SCRIPT_DIR}/frontend/dist/" "${SCRIPT_DIR}/cmd/server/dist/"
  else
    cp -R "${SCRIPT_DIR}/frontend/dist/." "${SCRIPT_DIR}/cmd/server/dist/"
  fi
  echo "==> 前端产物已复制到 cmd/server/dist"
}

build() {
  local goos="$1" goarch="$2" name="$3"
  echo "==> 编译 ${goos}/${goarch} -> build/${name}"
  CGO_ENABLED=0 GOOS="${goos}" GOARCH="${goarch}" \
    go build -trimpath -ldflags="${LDFLAGS}" -o "${OUTPUT_DIR}/${name}" ./cmd/server
  copy_device_config
}

mkdir -p "${OUTPUT_DIR}"

case "${1:-all}" in
  frontend)
    build_frontend
    exit 0
    ;;
  local)
    build_frontend
    go build -trimpath -ldflags="${LDFLAGS}" -o "${OUTPUT_DIR}/data-collect" ./cmd/server
    copy_device_config
    echo "==> 构建完成: build/data-collect"
    ;;
  x86)
    build_frontend
    build linux amd64 data-collect-x86
    ;;
  arm64)
    build_frontend
    build linux arm64 data-collect-arm64
    ;;
  all)
    build_frontend
    build linux amd64 data-collect-x86
    build linux arm64 data-collect-arm64
    ;;
  package)
    build_frontend
    build linux amd64 data-collect-x86
    build linux arm64 data-collect-arm64

    # 打发布包：二进制 + 配置 + supervisor 安装脚本
    for item in "x86:data-collect-x86" "arm64:data-collect-arm64"; do
      arch="${item%%:*}"
      binname="${item##*:}"
      pkgdir="${OUTPUT_DIR}/pkg-${arch}"
      rm -rf "${pkgdir}"
      mkdir -p "${pkgdir}/config"
      cp "${OUTPUT_DIR}/${binname}" "${pkgdir}/data-collect"
      cp "${SCRIPT_DIR}/config/server.yaml" "${pkgdir}/config/server.yaml"
      cp "${SCRIPT_DIR}/config/device.yaml" "${pkgdir}/config/device.yaml"
      cp "${SCRIPT_DIR}/deploy/install.sh" "${pkgdir}/install.sh"
      cp "${SCRIPT_DIR}/deploy/supervisor/data-collect.conf" "${pkgdir}/data-collect.conf"
      tar -C "${OUTPUT_DIR}" -czf "${OUTPUT_DIR}/data-collect-${arch}.tar.gz" "pkg-${arch}"
      rm -rf "${pkgdir}"
      echo "==> 发布包: build/data-collect-${arch}.tar.gz"
    done
    ;;
  clean)
    rm -rf "${OUTPUT_DIR}"
    echo "==> 已清理 build/"
    ;;
  *)
    echo "用法: $0 {local|x86|arm64|all|clean}" >&2
    exit 1
    ;;
esac
