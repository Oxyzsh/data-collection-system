package domain

// SelectorType 常量定义外部 selector 能请求的几类值。
// 这是一个“类型标号”，不是 Go 的 interface 类型。
//
// 裁剪说明：告警模块已移除，因此不再提供“设备/测点当前告警数”类型。
const (
	SelectorTypeCurrentValue = 1
	SelectorTypeMappedValue  = 2
)
