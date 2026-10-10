package domain

// DevicePointConfig 设备测点/控制点用户配置。
type DevicePointConfig struct {
	ID           int    `json:"id"`
	DeviceID     int    `json:"device_id"`
	PointID      int    `json:"point_id"`
	Enabled      bool   `json:"enabled"`
	AliasEnabled bool   `json:"alias_enabled"`
	AliasZH      string `json:"alias_zh"`
	AliasEN      string `json:"alias_en"`
	ControlLevel int    `json:"control_level"`
	UpdatedAt    string `json:"updated_at"`
}

func NormalizeControlLevel(level int) int {
	if level < 1 || level > 3 {
		return 1
	}
	return level
}
