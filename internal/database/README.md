# database

数据库基础能力模块。

## 模块作用
- 负责数据库连接初始化、关闭和基础连接管理。
- 这里只保留数据库底座能力，不承载业务表创建逻辑。

## 路径边界
- `db.go`：数据库连接初始化与关闭。

## package引用关系
- 主要被各业务模块的 `infrastructure`、`runtime`、`application` 包引用。
- 这里只处理公共数据库连接，不直接承载业务仓储实现。

## package被引用关系
- 主要被 `alarm`、`auth`、`collector`、`device`、`export`、`history`、`logging`、`maintenance`、`metric`、`runtime`、`system`、`template` 引用。

## 文件树
```text
backend/internal/database/
├── README.md                           # 模块说明
└── db.go                               # 数据库连接初始化与关闭
```
