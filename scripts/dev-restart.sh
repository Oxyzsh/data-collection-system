#!/usr/bin/env bash
# 本地开发：构建（前端 + 后端）并重启本地服务，最后做健康检查。
#
# 用法：./scripts/dev-restart.sh
#   - 自动执行 ./build.sh local（构建前端并嵌入二进制）
#   - 停掉正在运行的本地实例（pkill），后台启动新实例
#   - 输出健康检查结果与日志路径
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "${SCRIPT_DIR}")"
LOG_FILE="${ROOT_DIR}/build/data-collect.log"
PORT="$(grep -E '^\s*port:' "${ROOT_DIR}/config/server.yaml" | head -1 | awk '{print $2}')"
PORT="${PORT:-8090}"

cd "${ROOT_DIR}"

echo "==> 1/4 构建（./build.sh local）"
./build.sh local

echo "==> 2/4 停止旧实例"
pkill -f 'build/data-collect' 2>/dev/null || true
sleep 1

echo "==> 3/4 启动新实例（日志：${LOG_FILE}）"
mkdir -p "${ROOT_DIR}/build"
nohup "${ROOT_DIR}/build/data-collect" >> "${LOG_FILE}" 2>&1 &
sleep 2

echo "==> 4/4 健康检查"
if curl -sf -m 5 "http://127.0.0.1:${PORT}/healthz"; then
  echo
  echo "✔ 本地服务已重启：http://localhost:${PORT}（日志 ${LOG_FILE}）"
else
  echo "✘ 健康检查失败，最近日志：" >&2
  tail -n 20 "${LOG_FILE}" >&2
  exit 1
fi
