# runtime

运行态域：统一设备/测点运行时数据访问。

## 模块作用
- 负责统一运行态数据访问和运行态基础能力。
- 当前以单一运行模型为主，避免多套 runtime 抽象并存。
- 历史落盘写入器（`HourlyHistoryWriter`）也在此模块：采集链路只调 `Add` 进内存，
  由后台调度在整点前窗口按设备错峰批量落盘到历史库（见下节）。

## 路径边界
- `infrastructure/`：运行态缓存、运行态持久化、数据管理器、整点错峰历史写入器。
- `bootstrap.go`：运行态初始化入口。

## 历史落盘（HourlyHistoryWriter）

- 采集值先入内存（每点位最新值 + 当日 min/max），采集线程不持数据库写锁；
  写入入口 `history/infrastructure/sql_history_writer.go` 的 `Add` 仅转发到此写入器。
- 落盘调度（`Start`，main.go 启动）每分钟检查：进入整点前窗口（长度 =
  `storage.history.flush_window_minutes`，默认 10 分钟）后，按 `deviceID % 窗口分钟数`
  映射槽位错峰，逐设备批量 UPSERT `device_data`（单语句分块 ≤200 行）；
  `collected_at` 取小时起点，同小时幂等覆盖——每小时每点位 1 行、值为该小时最新采集值。
- 失败不清「已落」标记，窗口内后续分钟与下一个窗口自动重试；关机钩子先 `Stop`
  再 `Flush` 全部未落值（main.go 注册，`database.CloseDB` 之前），设备停止走
  `FlushDevice`（manager.go 的 `UnregisterDevice` 调用）。
- 当日极值（`device_daily_extremum`）：极值刷新事件置 dirty，按
  `storage.history.extremum_flush_interval`（默认 5s）周期批量 UPSERT（MIN/MAX 合并语义），
  启动时 `LoadDailyExtremum` 装载当天行继续累加，重启不产生极值缺口。

## package引用关系
- 主要引用 `database`、`valueobj`。
- 各业务模块通过 `runtime/infrastructure` 读取运行态快照。

## package被引用关系
- 主要被 `alarm`、`collector`、`device`、`history`、`metric` 等运行链路引用。
- 也是运行态数据访问的统一入口。

## 文件树
```text
backend/internal/runtime/
├── README.md                           # 模块说明
├── bootstrap.go                        # 运行态初始化入口
└── infrastructure/
    ├── device_cache.go                 # 设备运行态缓存
    ├── hourly_history_writer.go        # 整点错峰历史写入器（内存汇总 + 当日极值 + 关机落盘）
    ├── manager.go                      # 运行态管理器
    └── persistence.go                  # 运行态持久化
```
