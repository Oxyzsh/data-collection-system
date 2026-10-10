# history

历史数据域：负责历史写入与历史查询。

## 模块作用
- 负责历史点值写入、历史查询与相关服务装配。
- 强调批量写入与查询性能，和实时运行态保持边界清晰。

## 路径边界
- `application/`：历史写入/查询用例。
- `infrastructure/`：历史 SQL 写入器与查询仓储。
- `interfaces/http/`：历史查询接口。
- `services.go` / `query_services.go` / `bootstrap.go`：服务装配入口。

## package引用关系
- 主要引用 `runtime/infrastructure`、`valueobj`、`database`、`shared`、`interfaces/httpx/response`。
- 写入和查询逻辑都围绕历史表与运行态快照展开。

## package被引用关系
- 主要被 `collector` 和 `interfaces` 引用。
- `runtime`、`valueobj` 也是历史链路里的高频被引用包。

## 文件树
```text
backend/internal/history/
├── README.md                           # 模块说明
├── application/
│   ├── query_service.go                # 历史查询用例
│   └── write_service.go                # 历史写入用例
├── bootstrap.go                        # 初始化入口
├── infrastructure/
│   ├── sql_history_writer.go           # 历史写入器
│   └── sql_query_repository.go         # 历史查询仓储
├── interfaces/
│   └── http/
│       ├── handler.go                  # 历史 HTTP 处理器
│       └── routes.go                   # 路由注册
├── query_services.go                   # 查询服务装配
└── services.go                         # 服务装配入口
```
