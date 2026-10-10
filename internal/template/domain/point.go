package domain

import "strings"

const (
	PointTypePOINT  = "POINT"
	PointTypeMETRIC = "METRIC"
)

// DevicePoint 定义模板中的单个测点。
//
// 字段语义：
// - OriginalType: 协议原始类型，只用于指导原始数据如何解析。
// - DataType: 系统最终值类型，只允许 INT64/FLOAT64/BOOL/STRING。
// - Modbus / SNMP: 协议专用配置，按模板类型二选一。
type DevicePoint struct {
	ID         int `json:"id" db:"id"`                   // 模板测点记录 ID
	TemplateID int `json:"template_id" db:"template_id"` // 所属模板 ID
	PointID    int `json:"point_id" db:"point_id"`       // 模板内测点编号

	Type   string `json:"type" db:"type"`       // 测点类型，如 POINT / METRIC
	Name   string `json:"name" db:"name"`       // 中文名称
	NameEN string `json:"name_en" db:"name_en"` // 英文名称

	OriginalType string `json:"original_type" db:"original_type"` // 协议原始解析类型
	DataType     string `json:"data_type" db:"data_type"`         // 系统最终值类型：INT64/FLOAT64/BOOL/STRING

	Unit        string  `json:"unit" db:"unit"`               // 中文单位
	UnitEN      string  `json:"unit_en" db:"unit_en"`         // 英文单位
	Coefficient float64 `json:"coefficient" db:"coefficient"` // 系数，原始值线性换算参数
	Offset      float64 `json:"offset" db:"offset"`           // 偏移量，原始值线性换算参数
	Precision   int     `json:"precision" db:"precision"`     // 浮点展示精度
	Access      string  `json:"access" db:"access"`           // 读写属性
	Enabled     int     `json:"enabled" db:"enabled"`         // 是否启用

	Expression      string  `json:"expression" db:"expression"`             // 自定义表达式
	ValueMappingCN  string  `json:"value_mapping_cn" db:"value_mapping_cn"` // 中文枚举值映射
	ValueMappingEN  string  `json:"value_mapping_en" db:"value_mapping_en"` // 英文枚举值映射
	Description     string  `json:"description" db:"description"`           // 备注说明
	ValueMeaning    string  `json:"value_meaning" db:"value_meaning"`       // 数值含义说明
	CorrectionValue float64 `json:"correction_value" db:"correction_value"` // 修正值

	Modbus  *ModbusPointSpec  `json:"modbus,omitempty"`
	SNMP    *SNMPPointSpec    `json:"snmp,omitempty"`
	LocalIO *LocalIOPointSpec `json:"local_io,omitempty"`
}

// ModbusPointSpec 是 Modbus 模板专用配置。
type ModbusPointSpec struct {
	RegisterAddress int    `json:"register_addr" db:"register_addr"`     // Modbus 寄存器地址
	RegisterCount   int    `json:"register_count" db:"register_count"`   // Modbus 寄存器数量
	ReadFuncCode    int    `json:"read_func_code" db:"read_func_code"`   // Modbus 读功能码
	WriteFuncCode   int    `json:"write_func_code" db:"write_func_code"` // Modbus 写功能码
	EncodingOrder   string `json:"encoding_order" db:"encoding_order"`   // Modbus 多寄存器编码顺序
}

// SNMPPointSpec 是 SNMP 模板专用配置。
type SNMPPointSpec struct {
	OID string `json:"oid" db:"oid"` // SNMP OID
}

// LocalIOPointSpec 是本地 GPIO 模板专用配置。
// Channel 只保存 DI/DO 逻辑通道名，真实 value 路径由 device.yaml 解析。
type LocalIOPointSpec struct {
	Channel   string `json:"channel" db:"channel"`
	Direction string `json:"direction" db:"direction"`
}

// NormalizePointType 把点类型归一到项目支持的枚举值。
// 非法值会回退到 POINT。
func NormalizePointType(s string) string {
	upper := strings.ToUpper(strings.TrimSpace(s))
	switch upper {
	case PointTypePOINT, PointTypeMETRIC:
		return upper
	default:
		return PointTypePOINT
	}
}
