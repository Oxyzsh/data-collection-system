# template

模板域：模板配置、模板解析、模板缓存。

## 模块作用
- 负责模板配置、模板解析、模板缓存与对外查询接口。
- 模板作为设备配置源，重点是缓存一致性和替换流程可控。

## 路径边界
- `application/`：模板增删改查、替换和预检查用例。
- `domain/`：模板模型、归一化、序列化规则。
- `infrastructure/`：模板仓储、缓存、解析器适配。
- `infrastructure/parsers/`：模板文件解析实现。
- `runtime/`：运行链路读取模板配置的轻量访问层。
- `interfaces/http/`：模板接口。
- `bootstrap.go` / `facade.go` / `services.go`：启动预热与对外入口。

## package引用关系
- 主要引用 `alarm`、`collector`、`device`、`protocol`、`runtime`、`valueobj`、`database`、`shared`。
- `infrastructure/parsers` 负责模板文件解析，`runtime` 负责高频读取。

## package被引用关系
- 主要被 `collector`、`device`、`alarm`、`metric`、`interfaces/http` 引用。
- 模板是设备、告警和采集链路的共同配置源。

## 文件树
```text
backend/internal/template/
├── README.md                           # 模块说明
├── application/
│   ├── config_service.go               # 模板配置服务
│   ├── parser_service.go               # 模板解析服务
│   └── query_service.go                # 模板查询服务
├── bootstrap.go                        # 初始化入口
├── domain/
│   ├── cache.go                        # 模板缓存模型
│   ├── config.go                       # 模板配置模型
│   ├── entity.go                       # 模板实体
│   ├── normalizer.go                   # 模板归一化
│   ├── point.go                        # 模板点位模型
│   ├── serializer.go                   # 序列化规则
│   └── validation.go                   # 校验规则
├── facade.go                           # 对外门面
├── infrastructure/
│   ├── bootstrap.go                    # 基础设施装配
│   ├── bootstrap_test.go               # 装配测试
│   ├── cache_adapter.go                # 缓存适配器
│   ├── cache_store.go                  # 缓存存储
│   ├── parser_adapter.go               # 解析器适配器
│   ├── parser_provider.go              # 解析器提供器
│   ├── parsers/
│   │   ├── parser.go                   # 解析器接口
│   │   └── xlsx.go                     # XLSX 解析实现
│   └── sql_config_repository.go        # 模板配置仓储
├── interfaces/
│   └── http/
│       ├── handler.go                  # 模板 HTTP 处理器
│       └── routes.go                   # 路由注册
├── runtime/
│   └── provider.go                     # 模板运行态提供器
└── services.go                         # 服务入口
```
