# valueobj

跨域值对象定义。

## 模块作用
- 负责跨域共享的值对象和数据类型规范。
- 统一不同模块的值模型，减少转换成本。

## 路径边界
- `datatype.go`：数据类型常量与规范化/合法性判断。
- `value.go`：统一采集值模型（number/text/bool）。

## package引用关系
- 根目录本身只承载基础类型定义，不直接依赖业务模块。
- 子文件只使用标准库与自身相关的简单类型处理。

## package被引用关系
- 主要被 `collector`、`device`、`history`、`metric`、`protocol`、`runtime`、`template` 引用。
- 作为跨域值模型底座，不依赖业务模块。

## 文件树
```text
backend/internal/valueobj/
├── README.md                           # 模块说明
├── datatype.go                         # 数据类型常量与合法性判断
└── value.go                            # 统一采集值模型
```
