package common

// ParamType 参数类型
type ParamType string

const (
	ParamTypeInt    ParamType = "int"
	ParamTypeString ParamType = "string"
	ParamTypeEnum   ParamType = "enum"
	ParamTypeBool   ParamType = "bool"
)

// ParamDefinition 参数定义
type ParamDefinition struct {
	Name        string    `json:"name"`        // 参数标识（英文）
	NameCN      string    `json:"name_cn"`     // 中文名称
	NameEN      string    `json:"name_en"`     // 英文名称
	Type        ParamType `json:"type"`        // 参数类型
	Required    bool      `json:"required"`    // 是否必填
	Min         float64   `json:"min"`         // 最小值(数值类型)
	Max         float64   `json:"max"`         // 最大值(数值类型)
	Options     []string  `json:"options"`     // 可选值(枚举类型)
	Default     string    `json:"default"`     // 缺省值
	Description string    `json:"description"` // 描述
}

// CommonParams 公共参数定义
// 所有协议类型的设备都包含这些参数
var CommonParams = []ParamDefinition{
	{
		Name:        "timeout",
		NameCN:      "超时时间(ms)",
		NameEN:      "Timeout(ms)",
		Type:        ParamTypeInt,
		Required:    false,
		Min:         100,
		Max:         60000,
		Default:     "500",
		Description: "通信超时时间，单位毫秒",
	},
	{
		Name:        "retry_cnt",
		NameCN:      "重试次数",
		NameEN:      "Retry Count",
		Type:        ParamTypeInt,
		Required:    false,
		Min:         0,
		Max:         100,
		Default:     "5",
		Description: "通信失败后的重试次数，0表示不重试，超过此次数标记设备离线",
	},
	{
		Name:        "interval",
		NameCN:      "采集间隔(ms)",
		NameEN:      "Collection Interval(ms)",
		Type:        ParamTypeInt,
		Required:    false,
		Min:         100,
		Max:         86400000,
		Default:     "5000",
		Description: "数据采集间隔时间，单位毫秒",
	},
}

func GetCommonParams() []ParamDefinition {
	return CommonParams
}

func GetCommonParamDefaults() map[string]string {
	defaults := make(map[string]string)
	for _, param := range CommonParams {
		if param.Default != "" {
			defaults[param.Name] = param.Default
		}
	}
	return defaults
}
