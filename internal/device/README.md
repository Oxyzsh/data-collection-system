# device

设备域：设备配置与设备生命周期管理（不承载采集运行线程）。

## 模块作用
- 负责设备配置、设备启停、点位配置与查询。
- 不承载采集运行线程，运行态交给 `collector/runtime` 与 `runtime/`。

## 路径边界
- `application/`：设备增删改、启停、点位配置、查询用例。
- `domain/`：设备领域模型与约束。
- `infrastructure/`：设备相关 SQL 仓储实现。
- `interfaces/http/`：设备与协议相关 HTTP 接口。
- `bootstrap.go` / `runtime_facade.go` / `services.go`：模块初始化、运行态门面和服务装配。

## package引用关系
- 主要引用 `collector`、`protocol`、`runtime`、`template`、`alarm`、`database`、`shared`、`valueobj`、`interfaces`。
- `application` 负责配置态编排，`infrastructure` 负责持久化与运行态读取。

## package被引用关系
- 主要被 `collector`、`metric`、`interfaces`、`shared` 引用。
- 设备是采集、告警、模板等模块共同依赖的资源中心。

## 文件树
```text
backend/internal/device/
├── README.md                           # 模块说明
├── application/
│   ├── config_service.go               # 设备配置用例
│   ├── lifecycle_service.go            # 生命周期用例
│   ├── point_config_service.go         # 点位配置用例
│   ├── query_service.go                # 查询用例
│   ├── selector_realtime_service.go    # 选择器实时数据服务
│   └── selector_service.go             # 选择器用例
├── bootstrap.go                        # 初始化入口
├── domain/
│   ├── config.go                       # 设备配置模型
│   ├── entity.go                       # 设备实体
│   ├── lifecycle.go                    # 生命周期规则
│   ├── point_config.go                 # 点位配置模型
│   └── selector_type.go                # 选择器类型
├── infrastructure/
│   ├── selector_provider.go            # 选择器数据提供器
│   ├── sql_config_repository.go        # 配置仓储
│   ├── sql_lifecycle_repository.go     # 生命周期仓储
│   ├── sql_point_config_repository.go  # 点位配置仓储
│   └── sql_query_repository.go         # 查询仓储
├── interfaces/
│   └── http/
│       ├── device_handler.go           # 设备接口
│       ├── protocol_handler.go         # 协议接口
│       ├── routes.go                   # 路由注册
│       └── selector_handler.go         # 选择器接口
├── runtime_facade.go                   # 运行态门面
└── services.go                         # 服务装配入口
```
