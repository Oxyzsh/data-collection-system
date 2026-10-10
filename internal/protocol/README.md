# protocol

协议能力域：协议参数定义、校验、注册。

## 模块作用
- 负责协议参数定义、参数校验和协议注册。
- 作为设备与采集链路的协议基础包，不承载具体采集实现。

## 路径边界
- `common/`：协议通用参数与通用校验。
- `modbus/`：modbus 协议参数和校验。
- `snmp/`：snmp 协议参数和校验。
- `common.go` / `protocol.go` / `validator.go`：统一协议注册与校验入口。

## package引用关系
- 主要引用 `valueobj`。
- `modbus` 和 `snmp` 都复用 `common` 层的通用校验与参数定义。

## package被引用关系
- 主要被 `collector`、`device`、`template` 引用。
- 也是设备协议配置接口与采集驱动的共同依赖。

## 文件树
```text
backend/internal/protocol/
├── README.md                           # 模块说明
├── common.go                           # 通用协议入口
├── common/
│   ├── common.go                       # 通用参数定义
│   └── validator.go                    # 通用校验逻辑
├── modbus/
│   ├── modbus.go                       # Modbus 协议定义
│   ├── spec.go                         # Modbus 规格说明
│   └── validator.go                    # Modbus 参数校验
├── protocol.go                         # 协议注册入口
├── protocol_test.go                    # 协议注册测试
├── registry_test.go                    # 注册表测试
├── snmp/
│   ├── snmp.go                         # SNMP 协议定义
│   ├── types.go                        # SNMP 类型定义
│   ├── types_test.go                   # SNMP 类型测试
│   └── validator.go                    # SNMP 参数校验
└── validator.go                        # 统一校验入口
```
