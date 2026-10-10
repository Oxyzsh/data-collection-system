#!/usr/bin/env bash
# 在目标虚拟机内以 root 运行：安装/升级 data-collect（supervisor 托管）。
#
# 用法（宿主机执行）：
#   prlctl exec <VM_UUID> /bin/bash /media/psf/Home/.../deploy/vm-install.sh [端口]
#
# 说明：
# - 发布包经 Parallels 共享目录（/media/psf/Home）读取；
# - 首次安装写入指定端口，升级时保留 /opt/data-collect/config 与 data/ 不动；
# - 安装完成后打印 supervisor 状态与健康检查结果。
set -euo pipefail

TARBALL="${TARBALL:-/media/psf/Home/workspace/go/src/data_collection_system/build/data-collect-arm64.tar.gz}"
PORT="${1:-8091}"

[ "$(id -u)" -eq 0 ] || { echo "需要 root 运行" >&2; exit 1; }
[ -f "${TARBALL}" ] || { echo "找不到发布包: ${TARBALL}" >&2; exit 1; }

WORK="$(mktemp -d /tmp/dc-install.XXXXXX)"
tar -xzf "${TARBALL}" -C "${WORK}" --strip-components=1

# 首次安装：把部署端口写入示例配置；已有现场配置则不覆盖。
if [ ! -f /opt/data-collect/config/server.yaml ]; then
  sed -i "s/^  port: .*/  port: ${PORT}/" "${WORK}/config/server.yaml"
  echo "首次安装，端口设置为 ${PORT}"
else
  echo "检测到已有配置，保留 /opt/data-collect/config 不变"
fi

bash "${WORK}/install.sh"

echo
sleep 2
echo "=== supervisor 状态 ==="
supervisorctl status data-collect || true
echo "=== 健康检查（127.0.0.1:${PORT}） ==="
curl -sS -m 5 "http://127.0.0.1:${PORT}/healthz" || echo "(健康检查未通过，请查看日志)"
echo
echo "=== 最近日志 ==="
tail -n 12 /var/log/data-collect/data-collect.out.log 2>/dev/null || true
