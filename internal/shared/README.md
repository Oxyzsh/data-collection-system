# shared

跨域共享基础能力。

## 模块作用
- 负责跨域共享的基础能力，供多个业务模块复用。
- 这里不放业务流程，只放稳定、可复用的公共能力。

## 路径边界
- `buildinfo/`：构建信息读取。
- `clock/`：时钟抽象。
- `errors/`：统一错误模型。
- `i18n/`：语言常量、字典和 key 渲染逻辑。
- `logger/`：日志抽象。
- `security/jwt/`：JWT 生成与解析。
- `tx/`：事务执行器封装。
- `ws/`：WebSocket 通用能力。

## package引用关系
- 根目录本身不直接依赖业务模块，更多是各子包自洽承载对应基础能力。
- 子包内部通常只依赖标准库和少量第三方库。

## package被引用关系
- 主要被 `auth`、`alarm`、`collector`、`device`、`history`、`interfaces`、`logging`、`maintenance`、`metric`、`system`、`template` 引用。
- `shared/security/jwt`、`shared/i18n`、`shared/errors`、`shared/ws` 是使用频率最高的公共包。

## 文件树
```text
backend/internal/shared/
├── README.md                           # 模块说明
├── buildinfo/
│   └── buildinfo.go                    # 构建信息读取
├── clock/
│   └── clock.go                        # 时钟抽象
├── errors/
│   └── app_error.go                    # 统一错误模型
├── i18n/
│   ├── locale.go                       # 语言常量与上下文
│   ├── locale_test.go                  # 语言相关测试
│   ├── messages.go                     # 文案字典
│   └── messages_test.go                # 文案渲染测试
├── logger/
│   └── logger.go                       # 日志抽象
├── security/
│   └── jwt/
│       ├── config.go                   # JWT 配置
│       └── jwt.go                      # JWT 生成与解析
├── tx/
│   └── tx.go                           # 事务执行器封装
└── ws/
    ├── selector_protocol.go           # 选择器 WebSocket 协议
    └── websocket.go                   # WebSocket 通用能力
```
