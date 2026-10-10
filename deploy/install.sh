#!/usr/bin/env bash
# 数据采集系统安装脚本（在目标虚拟机/板卡上以 root 运行）。
#
# 用法：
#   ./install.sh                  # 使用脚本同目录下的 data-collect 二进制
#   ./install.sh /path/to/binary  # 指定二进制
#
# 动作：
#   1. 安装二进制到 /opt/data-collect（已存在配置/数据不覆盖）
#   2. 安装 supervisor 配置并注册为自启动服务
#   3. supervisorctl 启动服务并输出状态
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
APP_DIR="/opt/data-collect"
LOG_DIR="/var/log/data-collect"
SERVICE_NAME="data-collect"

BIN_SRC="${1:-${SCRIPT_DIR}/data-collect}"
CONF_SRC="${SCRIPT_DIR}/data-collect.conf"

[ "$(id -u)" -eq 0 ] || { echo "请用 root 运行（sudo ./install.sh）" >&2; exit 1; }
[ -f "${BIN_SRC}" ] || { echo "找不到二进制: ${BIN_SRC}" >&2; exit 1; }
[ -f "${CONF_SRC}" ] || { echo "找不到 supervisor 配置: ${CONF_SRC}" >&2; exit 1; }

# 1. 查找 supervisor 配置目录（Debian/Ubuntu 与 RHEL 系路径不同）
SUPERVISOR_DIR=""
for dir in /etc/supervisor/conf.d /etc/supervisord.d; do
  if [ -d "${dir}" ]; then SUPERVISOR_DIR="${dir}"; break; fi
done
[ -n "${SUPERVISOR_DIR}" ] || { echo "未找到 supervisor 配置目录，请先安装 supervisor" >&2; exit 1; }

# 2. 安装程序与配置（保留已有配置与数据）
mkdir -p "${APP_DIR}/config" "${LOG_DIR}"
install -m 0755 "${BIN_SRC}" "${APP_DIR}/data-collect"

# 查找发布包内的模板配置：优先 <包>/config/，其次 <包>/，最后仓库布局 ../config
find_config() {
  local name="$1"
  for candidate in "${SCRIPT_DIR}/config/${name}" "${SCRIPT_DIR}/${name}" "${SCRIPT_DIR}/../config/${name}"; do
    if [ -f "${candidate}" ]; then echo "${candidate}"; return 0; fi
  done
  return 1
}
if src="$(find_config server.yaml)"; then
  [ -f "${APP_DIR}/config/server.yaml" ] || cp "${src}" "${APP_DIR}/config/server.yaml"
fi
if src="$(find_config device.yaml)"; then
  [ -f "${APP_DIR}/config/device.yaml" ] || cp "${src}" "${APP_DIR}/config/device.yaml"
fi

install -m 0644 "${CONF_SRC}" "${SUPERVISOR_DIR}/${SERVICE_NAME}.conf"

# 3. 注册并启动
if command -v supervisorctl >/dev/null 2>&1; then
  supervisorctl reread
  supervisorctl update
  supervisorctl restart "${SERVICE_NAME}" || supervisorctl start "${SERVICE_NAME}" || true
  sleep 1
  supervisorctl status "${SERVICE_NAME}" || true
else
  echo "未找到 supervisorctl：配置已安装到 ${SUPERVISOR_DIR}/${SERVICE_NAME}.conf，请手动 reload supervisor" >&2
  exit 1
fi

echo
echo "安装完成："
echo "  程序: ${APP_DIR}/data-collect（配置: ${APP_DIR}/config/，数据: ${APP_DIR}/data/）"
echo "  端口: $(grep -E '^\s*port:' "${APP_DIR}/config/server.yaml" | head -1 | awk '{print $2}')"
echo "  日志: ${LOG_DIR}/data-collect.out.log"
echo "  管理: supervisorctl status|restart|stop ${SERVICE_NAME}"
