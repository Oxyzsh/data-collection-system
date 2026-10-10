// Package templatealarm 定义设备模板文件中「告警」页签的数据模型。
//
// 说明：模板导入/解析时告警规则随模板一起存储，但本进程只做数据采集，
// 不包含告警判定、通知等任何告警运行时逻辑。
package templatealarm

// 告警级别。
const (
	AlarmLevelINFO   = "INFO"
	AlarmLevelNOTICE = "NOTICE"
	AlarmLevelWARN   = "WARN"
	AlarmLevelEMERG  = "EMERG"
)

// DeviceAlarm 是模板中的一个告警规则定义。
type DeviceAlarm struct {
	ID         int    `json:"id" db:"id"`
	TemplateID int    `json:"template_id" db:"template_id"`
	PointID    int    `json:"point_id" db:"point_id"`
	Name       string `json:"name" db:"name"`
	NameEN     string `json:"name_en" db:"name_en"`
	Level      string `json:"level" db:"level"`
	Enabled    bool   `json:"enabled" db:"enabled"`
	// StartXxx 定义“进入告警”的判定条件。
	// 指针字段表示该阈值可选；nil 代表这一侧边界未配置。
	StartLowLimit  *float64 `json:"start_low_limit" db:"start_low_limit"`
	StartHighLimit *float64 `json:"start_high_limit" db:"start_high_limit"`
	StartDelay     int      `json:"start_delay" db:"start_delay"`
	// EndXxx 定义“恢复正常”的判定条件。
	EndLowLimit  *float64 `json:"end_low_limit" db:"end_low_limit"`
	EndHighLimit *float64 `json:"end_high_limit" db:"end_high_limit"`
	EndDelay     int      `json:"end_delay" db:"end_delay"`
	Description  string   `json:"description" db:"description"`
	// PointMask 用于把规则绑定到某类点位子集，具体解释由配置逻辑处理。
	PointMask string `json:"point_mask" db:"point_mask"`
}
