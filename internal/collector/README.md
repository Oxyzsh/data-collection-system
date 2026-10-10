# collector

采集域：统一采集器生命周期编排与协议采集实现。

## 模块作用
- 负责采集器生命周期编排、协议采集驱动装配和运行态采集执行。
- 将“编排”和“驱动实现”拆开，便于新增协议或设备类型。

## 路径边界
- `application/`：采集器启动/停止/重启/状态查询抽象。
- `adapters/runtime/`：连接设备运行态管理的适配层。
- `adapters/driver/`：具体采集器实现（modbus/snmp/system/simulator）。
- `infrastructure/`：采集编排所需配置读取与数据访问实现。
- `runtime/`：采集运行编排核心（设备采集线程、采集链路执行、在线状态维护）。
- `services.go`：采集服务对外统一入口。

## package引用关系
- 主要引用 `device`、`template`、`runtime`、`protocol`、`valueobj`、`shared`、`database`。
- `adapters/driver` 还会直接引用各协议实现和部分第三方采集库。

## package被引用关系
- 主要被 `device`、`metric`、`template`、`alarm`、`history` 引用。
- `adapters/runtime` 和 `runtime` 会被设备运行态、告警运行态和模板运行态共同复用。

## 文件树
```text
backend/internal/collector/
├── README.md                           # 模块说明
├── adapters/
│   ├── driver/
│   │   ├── codec.go                    # 采集结果编码/解码
│   │   ├── codec_test.go               # 编码测试
│   │   ├── factory_registry_test.go    # 工厂注册测试
│   │   ├── interface.go                # driver 接口
│   │   ├── metric_collector.go         # 指标采集器
│   │   ├── modbus_collector.go         # Modbus 采集器
│   │   ├── modbus_collector_test.go    # Modbus 采集器测试
│   │   ├── simulator_collector.go      # 模拟采集器
│   │   ├── simulator_collector_test.go # 模拟采集器测试
│   │   ├── snmp_collector.go           # SNMP 采集器
│   │   ├── snmp_collector_test.go      # SNMP 采集器测试
│   │   ├── system_collector.go         # 系统采集器
│   │   └── system_collector_test.go    # 系统采集器测试
│   └── runtime/
│       └── manager.go                  # 运行态适配管理器
├── application/
│   └── service.go                      # 采集用例服务
├── infrastructure/
│   ├── device_config_loader.go         # 设备配置加载器
│   └── lifecycle_repository.go         # 生命周期仓储
├── runtime/
│   ├── collector_manager.go            # 采集管理器
│   └── collector_manager_test.go       # 采集管理器测试
└── services.go                         # 服务装配入口
```
