# 数据采集系统（data-collection-system）

面向机房 / 工业现场场景的**轻量数据采集系统**：以「设备模板导入 → 按模板添加设备 →
采集运行 → 运行态 / 历史落库」为主线，提供开箱即用的采集能力与内置管理页面。

- **单文件部署**：Go + Gin + SQLite（纯 Go，无 CGO），前端构建产物内嵌，一个二进制即可运行；
- **开箱即用**：内置设备模板与管理页面，启动后浏览器打开即可导入模板、添加设备、查看实时值与历史数据；
- **采集协议**：Modbus（RTU/TCP）、SNMP、本地 GPIO、模拟设备、系统指标（CPU / 内存 / 磁盘）；
- **接口风格**：REST + JSON，默认不鉴权，适合内网机器间直连调用。

## 功能范围

### 功能

| 功能 | 说明 |
|------|------|
| 设备模板导入 | Excel（xlsx）解析：预览 → 确认入库；支持 Modbus / SNMP / 本地开关量模板；内置模板（采集器、模拟温湿度、开关量输入/输出） |
| 按模板添加设备 | 设备增删改查、协议参数校验、测点实例化、测点配置与启停、启动/停止采集 |
| 数据采集 | Modbus（RTU/TCP）、SNMP、本地 GPIO、模拟设备、系统指标（CPU/内存/磁盘） |
| 运行态数据 | 最新值（数值/文本/布尔）与设备在线状态，落 `runtime.db` |
| 历史数据 | 采集值内存缓冲 + 整点错峰批量落 `history.db`，并维护当日极值 |
| 数据查询 | 设备测点实时值接口、设备历史数据接口、selector 批量取值接口 |
| 三方子系统接口 | 按 [三方协议对接指南](docs/THIRD_PARTY_PROTOCOL_GUIDE.md) **对外提供** 4 个接口：健康检查、设备树、测点实时值、测点写值 |
| Web 页面 | 独立前端（Vue3 + Vite + Element Plus）：设备模板导入/查看、按模板添加设备、启用开关（启停采集）、测点实时值、**采集数据查询页**（按设备/测点/时间范围看历史数据与趋势图，支持导出 CSV） |

### 功能边界（不包含）

本系统专注数据采集与采集配置，以下能力不在范围内（如有需要，建议在上层平台实现）：

- 认证与权限：接口默认免鉴权，建议部署在内网，或由网关 / 反向代理补一层鉴权；
- 告警判定、告警通知与告警记录；
- 操作日志、系统运维（网络 / NTP / 在线升级）、视频、门禁、组态画面、联动、北向转发与存储治理调度。

> 说明：设备模板 Excel 中的「告警」页签会被解析并随模板保存（模板文件格式的一部分，
> 见 `internal/templatealarm`），但系统**不做告警判定与通知**。

## 快速开始

```bash
# 构建（自动构建前端并嵌入二进制）
./build.sh local     # 当前平台（开发调试）
./build.sh x86       # linux/amd64
./build.sh arm64     # linux/arm64
./build.sh package   # 两个架构的发布包（build/data-collect-{x86,arm64}.tar.gz）

# 从项目根目录启动（数据目录 data/，默认端口 8090）
./build/data-collect
# 浏览器打开 http://<主机>:8090 即为管理页面
```

配置文件：

- `config/server.yaml`：监听端口、数据目录、历史落盘窗口；
- `config/device.yaml`：串口端点与本地 DI/DO 通道（设备接入目录）。
  运行时会从 `<程序目录>/config/device.yaml` 或 `/etc/mmdc/device.yaml` 读取，
  `build.sh` 会自动把示例配置复制到二进制旁边的 `build/config/` 下。

常用环境变量：`GOLOOK_CONFIG_FILE`、`GOLOOK_DATA_DIR`、`SERVER_PORT`、`GOLOOK_LOG_LEVEL`。

## 接口一览

| 类别 | 接口 |
|------|------|
| 健康检查 | `GET /healthz` |
| 设备接入目录 | `GET /api/device-catalog?selector=rs485_rtu`（也支持 `rs232_rtu` / `local_io`） |
| 模板 | `GET /api/devices/templates`、`GET /api/devices/template-types`、`GET /api/devices/templates/:id`、`GET /api/devices/templates/:id/detail`、`POST /api/devices/templates/import/preview`、`POST /api/devices/templates/import/confirm`、`POST /api/devices/templates/:id/replace`、`POST /api/devices/templates/:id/review`、`DELETE /api/devices/templates/:id` |
| 设备 | `GET/POST /api/devices`、`GET/PUT/DELETE /api/devices/:id`、`POST /api/devices/:id/start`、`POST /api/devices/:id/stop`、`POST /api/devices/:id/copy`、`PUT /api/devices/:id/enabled` |
| 测点 | `GET /api/devices/:id/points`、`PUT /api/devices/:id/points/:point_id/config`、`DELETE /api/devices/:id/points/:point_id/config`、`POST /api/devices/:id/points/:point_id/write` |
| 历史 / 参数 | `GET /api/devices/:id/history?point_id=&limit=&start=&end=`（start/end 为可选时间范围，格式 `2006-01-02 15:04:05`；limit 默认 100、上限 5000）、`GET/PUT /api/devices/:id/params` |
| 协议 | `GET /api/devices/protocols/types`、`GET /api/devices/protocols/params` |
| selector | `GET /api/devices/:id/selector-points`、`POST /api/devices/selector-values` |

### 1）设备模板导入

```bash
# 第一步：上传 xlsx 预览解析（返回 template_name / points / alarms 等）
curl -F "file=@examples/templates/tp_modbus_温湿度.xlsx" -F "template_type=modbus" \
  http://localhost:8090/api/devices/templates/import/preview > preview.json

# 第二步：用预览结果组装请求体，确认入库
curl -X POST http://localhost:8090/api/devices/templates/import/confirm \
  -H 'Content-Type: application/json' -d @confirm.json
# → {"code":200,"data":{"id":1001},...}
```

### 2）按模板添加设备

```bash
# 查询可选设备端点（device.yaml 生成），拿到 device_ref
curl "http://localhost:8090/api/device-catalog?selector=rs485_rtu"

# 按模板添加设备（Modbus 串口示例）
curl -X POST http://localhost:8090/api/devices -H 'Content-Type: application/json' -d '{
  "name": "温湿度-1",
  "template_id": 1001,
  "protocol_type": "RS485_RTU",
  "device_ref": "serial:xxxxxxxxxxxx",
  "params": {"slave_id": "1"}
}'

# 启动采集并查看实时测点
curl -X POST http://localhost:8090/api/devices/1/start
curl http://localhost:8090/api/devices/1/points

# 查询历史数据
curl "http://localhost:8090/api/devices/1/history?point_id=1&limit=100"
```

一键冒烟（服务需已启动）：

```bash
./scripts/e2e_smoke.sh
```

脚本完整走一遍：健康检查 → 设备目录 → 模板导入（预览+确认）→ 按模板添加设备（核对测点）→
添加内置模拟设备并启动采集（核对实时值）。

## 目录结构

```text
cmd/server               进程入口：初始化 3 个 SQLite 库 → 模块初始化 → 采集恢复 + HTTP 服务
internal/template        设备模板：导入解析、缓存、查询、替换
internal/templatealarm   模板文件中的告警数据模型（仅解析存储，不含判定逻辑）
internal/device          设备配置、测点配置、selector 查询
internal/collector       采集编排与驱动：modbus / snmp / gpio / system / simulator
internal/protocol        Modbus / SNMP 参数定义与校验
internal/runtime         运行态：最新值（数值/文本/布尔）、设备在线状态
internal/history         采集历史：内存缓冲 + 整点错峰批量落盘 + 当日极值
internal/devicecatalog   device.yaml → 设备接入目录（device_ref 解析）
internal/bootstrap       启动配置与运行目录
internal/subsystem       对外提供三方子系统接口（health / deviceTree / realtime / set）
internal/shared          日志、i18n、错误、版本号
config/                  server.yaml / device.yaml
examples/templates/      示例模板（Modbus 温湿度、SNMP 主机）
scripts/e2e_smoke.sh     端到端冒烟脚本
data/                    运行时数据目录（git 忽略，首次启动自动创建）
```

## 对外接口：三方子系统协议

本系统按 [docs/THIRD_PARTY_PROTOCOL_GUIDE.md](docs/THIRD_PARTY_PROTOCOL_GUIDE.md) **对外提供**以下接口，
供外部 Sub-System 拉取设备与采集数据、下发控制命令：

| 接口 | 方法 | 说明 |
|------|------|------|
| `/health` | GET | 健康检查（`{"code":0,"message":"success"}`） |
| `/protocol/sub/system/api/v1/converter/deviceTree` | GET | 设备树：每个设备一个根节点，其测点（含内置在线状态点 `0`）为叶子节点 |
| `/protocol/sub/system/api/v1/converter/realtime?pointStr=` | GET | 测点实时值（`data.pointId / timestamp(ms) / value`） |
| `/api/set/{pointStr}` | POST | 设置测点值（body `{"pointId":"...","value":...}`），仅 `rw`/`w` 测点可写 |

**pointStr 约定**：`<deviceId>.<pointId>`（如 `1.2` 表示设备 1 的 2 号测点）。外部系统可直接使用设备树返回的
`pointId`；`point_id = 0` 是采集器维护的「设备在线状态」点。

**错误码**：`0` 成功；`400` 参数错误、`404` 测点不存在、`503` 暂无实时数据（设备未运行或尚未采集）。

```bash
curl http://localhost:8090/health
curl http://localhost:8090/protocol/sub/system/api/v1/converter/deviceTree
curl "http://localhost:8090/protocol/sub/system/api/v1/converter/realtime?pointStr=1.1"
curl -X POST http://localhost:8090/api/set/1.3 -H 'Content-Type: application/json' \
     -d '{"pointId":"1.3","value":28}'
```

## 前端页面（frontend/）

独立前端工程，技术栈 Vue 3 + Vite + Element Plus：

| 页面 | 功能 |
|------|------|
| 设备模板 | 模板列表/查看（测点、告警、引用设备）；xlsx 导入（上传 → 解析预览 → 确认入库）；删除非内置模板 |
| 设备管理 | 设备列表（运行状态/启用状态）；**添加设备**（选模板 → 协议/端点/参数 → 提交）；启用开关（启用即开始采集并持久化，重启自动恢复）；测点实时值（3s 自动刷新）；测点历史数据 |
| 采集数据 | 选择设备 → 查看各测点当前值；点击测点查看**历史采集数据**：时间范围（最近 1 小时 / 24 小时 / 今天 / 自定义）、条数上限、趋势图、明细表格、导出 CSV |

开发模式（前后端分离调试）：

```bash
# 终端 1：启动后端
./build/data-collect

# 终端 2：启动前端 dev server（3100 端口，/api 代理到 8090）
cd frontend && npm install && npm run dev
```

生产模式：`./build.sh` 会自动执行 `npm run build`，把产物复制到 `cmd/server/dist/` 并通过
`//go:embed` 打进二进制 —— 部署时只需一个可执行文件，页面路径与接口同源。

## 运行与部署

程序不绑定任何进程管理器：**一个二进制 + 一个 `config/` 目录**即可运行；日志输出到 stdout/stderr，
自启动与保活交给部署环境（systemd、容器编排或手动 `nohup` 均可）。

发布包（`./build.sh package`）内容：

```text
pkg-x86/                    # 或 pkg-arm64
  data-collect              # 二进制（已内嵌前端页面）
  config/server.yaml        # 端口 / 数据目录
  config/device.yaml        # 串口 / 本地 IO 端点
  README.md
```

直接运行：

```bash
tar -xzf data-collect-x86.tar.gz   # 按机器架构选择 x86 / arm64
cd pkg-x86
./data-collect                     # 前台运行（默认 http://<主机>:8090）

# 或后台运行
nohup ./data-collect >> data-collect.log 2>&1 &
```

用 systemd 托管（示例；假设程序安装在 `/opt/data-collect`，按需调整）：

```ini
# /etc/systemd/system/data-collect.service
[Unit]
Description=Data Collection System
After=network.target

[Service]
WorkingDirectory=/opt/data-collect
ExecStart=/opt/data-collect/data-collect
Restart=on-failure
RestartSec=3

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now data-collect
systemctl status data-collect
```

说明：

- 运行数据（SQLite 库、上传文件）默认落在 `<安装目录>/data/`，首次启动自动创建；
- 升级只替换二进制，`config/` 与 `data/` 原样保留；
- 从零开始：停止进程后删除 `data/` 目录，重新启动会自动建库并写入内置模板；
- 日志：程序输出到 stdout/stderr，由部署环境收集（systemd journal / 重定向到文件 / 容器日志）。

## 数据存储

| 库 | 表 | 用途 |
|----|----|------|
| `data/backend/db/config.db` | `device_templates` / `devices` / `device_params` / `cfg_dev_point` | 模板与设备配置 |
| `data/backend/db/runtime.db` | `device_latest_values_num/text/bool` / `device_online_status` | 最新值与在线状态 |
| `data/backend/db/history.db` | `device_data` / `device_daily_extremum` | 采集历史与当日极值 |

> 需要从零开始时：停止进程后删除 `data/` 目录，重新启动即可自动建库。

## 测试

```bash
go test ./...            # 单元测试（其中 2 个 Modbus 用例需要本机回环端口监听权限）
./scripts/e2e_smoke.sh   # 端到端冒烟（需先启动服务）
```

## License

本项目采用 [MIT License](LICENSE)。
