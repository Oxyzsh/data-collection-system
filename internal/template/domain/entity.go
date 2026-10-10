package domain

// DeviceTemplate 是模板主表记录。
// 它描述“这类设备是什么”，真正的点位和告警数据放在 data 里。
type DeviceTemplate struct {
	ID          int    `json:"id" db:"id"`
	Name        string `json:"name" db:"name"`
	Type        string `json:"type" db:"type"`
	IsBuiltIn   bool   `json:"is_built_in" db:"is_built_in"`
	Description string `json:"description" db:"description"`
	CreatedAt   string `json:"created_at" db:"created_at"`
	UpdatedAt   string `json:"updated_at" db:"updated_at"`
}
