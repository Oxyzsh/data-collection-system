package valueobj

import "strings"

// 系统最终值类型（模板 DataType）。
// 解析、换算、表达式处理后的值，统一按这四类进入缓存、历史、告警和展示链路。
const (
	DataTypeINT64   = "INT64"
	DataTypeFLOAT64 = "FLOAT64"
	DataTypeBOOL    = "BOOL"
	DataTypeSTRING  = "STRING"
)

// 协议原始解析类型常量。
// 这些类型仅用于 OriginalType，不应用于模板 DataType。
const (
	RawTypeINT16   = "INT16"
	RawTypeUINT16  = "UINT16"
	RawTypeINT32   = "INT32"
	RawTypeUINT32  = "UINT32"
	RawTypeINT64   = "INT64"
	RawTypeUINT64  = "UINT64"
	RawTypeFLOAT32 = "FLOAT32"
	RawTypeFLOAT64 = "FLOAT64"
	RawTypeBOOL    = "BOOL"
	RawTypeSTRING  = "STRING"
)

func IsValidDataType(s string) bool {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case DataTypeINT64, DataTypeFLOAT64, DataTypeBOOL, DataTypeSTRING:
		return true
	default:
		return false
	}
}

func NormalizeDataType(s string) string {
	upper := strings.ToUpper(strings.TrimSpace(s))
	if upper == "" {
		return ""
	}
	if IsValidDataType(upper) {
		return upper
	}
	return ""
}

func IsValidRawType(s string) bool {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case RawTypeINT16, RawTypeUINT16, RawTypeINT32, RawTypeUINT32,
		RawTypeINT64, RawTypeUINT64, RawTypeFLOAT32, RawTypeFLOAT64,
		RawTypeBOOL, RawTypeSTRING:
		return true
	default:
		return false
	}
}

func NormalizeRawType(s string) string {
	upper := strings.ToUpper(strings.TrimSpace(s))
	if upper == "" {
		return ""
	}
	if IsValidRawType(upper) {
		return upper
	}
	return ""
}
