#!/usr/bin/env bash
# 数据采集系统端到端冒烟测试（无鉴权，直接调用开放 API）：
#   1. 健康检查；
#   2. 查询设备接入目录（device.yaml）；
#   3. 导入设备模板（xlsx：预览 → 确认）；
#   4. 按导入的模板添加 Modbus 设备并核对测点；
#   5. 添加一台内置「模拟温湿度」设备，启动采集并核对实时值。
#
# 前置：服务已在本机启动（默认 http://127.0.0.1:8090）。
# 用法：BASE=http://127.0.0.1:8090 ./scripts/e2e_smoke.sh
set -euo pipefail

BASE="${BASE:-http://127.0.0.1:8090}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "${SCRIPT_DIR}")"
TEMPLATE_XLSX="${ROOT_DIR}/examples/templates/tp_modbus_温湿度.xlsx"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "${TMP_DIR}"' EXIT

fail() { echo "FAIL: $*" >&2; exit 1; }
step() { echo; echo "==> $*"; }

jget() { # jget <file> <python 表达式>
  python3 - "$1" "$2" <<'PY'
import json, sys
d = json.load(open(sys.argv[1], encoding='utf-8'))
print(eval(sys.argv[2]))  # noqa: S307 - 冒烟脚本内部使用
PY
}

[ -f "${TEMPLATE_XLSX}" ] || fail "缺少示例模板 ${TEMPLATE_XLSX}"

step "1/6 健康检查"
curl -sS -m 10 "${BASE}/healthz" > "${TMP_DIR}/health.json"
jget "${TMP_DIR}/health.json" "('  服务状态={} 版本={}'.format(d['status'], d['version']))"

step "2/6 查询设备接入目录（device.yaml）"
curl -sS -m 10 "${BASE}/api/device-catalog?selector=rs485_rtu" > "${TMP_DIR}/catalog.json"
DEVICE_REF="$(jget "${TMP_DIR}/catalog.json" 'd["data"]["array"][0]["ref"]')"
DEVICE_LABEL="$(jget "${TMP_DIR}/catalog.json" 'd["data"]["array"][0]["label"]')"
echo "  串口端点: ${DEVICE_LABEL} (${DEVICE_REF})"

step "3/6 模板导入预览（xlsx）"
curl -sS -m 20 -X POST "${BASE}/api/devices/templates/import/preview" \
  -F "file=@${TEMPLATE_XLSX}" \
  -F "template_type=modbus" > "${TMP_DIR}/preview.json"
python3 - "${TMP_DIR}/preview.json" "${TMP_DIR}/confirm.json" <<'PY'
import json, sys
preview = json.load(open(sys.argv[1], encoding='utf-8'))
if preview.get("code") != 200:
    raise SystemExit(f"预览失败: {preview}")
data = preview["data"]
payload = {
    "template_name": "冒烟测试-温湿度",
    "description": data.get("description", "端到端冒烟导入"),
    "template_type": data.get("template_type") or "modbus",
    "points": data.get("points") or [],
    "alarms": data.get("alarms") or [],
}
print(f"  解析完成: 测点 {len(payload['points'])} 个, 告警 {len(payload['alarms'])} 个")
json.dump(payload, open(sys.argv[2], "w", encoding="utf-8"), ensure_ascii=False)
PY

step "4/6 模板导入确认"
curl -sS -m 20 -X POST "${BASE}/api/devices/templates/import/confirm" \
  -H 'Content-Type: application/json' \
  --data-binary "@${TMP_DIR}/confirm.json" > "${TMP_DIR}/confirm_resp.json"
TEMPLATE_ID="$(jget "${TMP_DIR}/confirm_resp.json" 'd["data"]["id"]')"
echo "  模板已入库: template_id=${TEMPLATE_ID}"

step "5/6 按模板添加设备（Modbus 串口）"
curl -sS -m 20 -X POST "${BASE}/api/devices" \
  -H 'Content-Type: application/json' \
  -d "{\"name\":\"冒烟测试-温湿度设备\",\"template_id\":${TEMPLATE_ID},\"protocol_type\":\"RS485_RTU\",\"device_ref\":\"${DEVICE_REF}\",\"description\":\"按导入模板添加\",\"params\":{\"slave_id\":\"1\"}}" \
  > "${TMP_DIR}/add_resp.json"
DEVICE_ID="$(jget "${TMP_DIR}/add_resp.json" 'd["data"]["id"]')"
echo "  设备已添加: device_id=${DEVICE_ID}"

curl -sS -m 10 "${BASE}/api/devices/${DEVICE_ID}" > "${TMP_DIR}/detail.json"
jget "${TMP_DIR}/detail.json" "('  设备详情: name={} template={} status={}'.format(d['data']['name'], d['data']['template_id'], d['data']['status']))"
curl -sS -m 10 "${BASE}/api/devices/${DEVICE_ID}/points" > "${TMP_DIR}/points.json"
jget "${TMP_DIR}/points.json" "('  设备测点: 数量={} 运行中={}'.format(len(d['data']), d.get('running')))"

step "6/6 添加内置模拟设备并验证采集运行"
curl -sS -m 20 -X POST "${BASE}/api/devices" \
  -H 'Content-Type: application/json' \
  -d '{"name":"冒烟测试-模拟设备","template_id":2,"description":"验证采集运行"}' > "${TMP_DIR}/sim_add.json"
SIM_ID="$(jget "${TMP_DIR}/sim_add.json" 'd["data"]["id"]')"
curl -sS -m 10 -X POST "${BASE}/api/devices/${SIM_ID}/start" > "${TMP_DIR}/sim_start.json"
echo "  模拟设备 ID=${SIM_ID}, 启动响应: $(cat "${TMP_DIR}/sim_start.json")"
sleep 3
curl -sS -m 10 "${BASE}/api/devices/${SIM_ID}/points" > "${TMP_DIR}/sim_points.json"
jget "${TMP_DIR}/sim_points.json" "('  运行中={} 首测点={}:{} 状态={}'.format(d.get('running'), d['data'][0]['point_id'], d['data'][0].get('value'), d['data'][0].get('status')))"

echo
echo "SMOKE TEST PASSED"
