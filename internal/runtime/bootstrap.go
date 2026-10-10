package runtime

import (
	"fmt"

	"data-collection-system/internal/database"
)

// InitTables 创建采集运行态表：
// - 设备最新值（数值 / 文本 / 布尔三张表）；
// - 设备在线状态缓存。
//
// 说明：裁剪后的进程只做数据采集，不建告警运行态等表。
func InitTables() error {
	if database.RuntimeDB == nil {
		return fmt.Errorf("runtime db is nil")
	}
	_, err := database.RuntimeDB.Exec(`
		-- 数值最新值表
		CREATE TABLE IF NOT EXISTS device_latest_values_num (
			device_id INTEGER NOT NULL,
			point_id INTEGER NOT NULL,
			value REAL NOT NULL,
			unit VARCHAR(20),
			data_type VARCHAR(50),
			collected_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY(device_id, point_id)
		);
		CREATE INDEX IF NOT EXISTS idx_latest_num_device ON device_latest_values_num(device_id);

		-- 文本最新值表
		CREATE TABLE IF NOT EXISTS device_latest_values_text (
			device_id INTEGER NOT NULL,
			point_id INTEGER NOT NULL,
			value TEXT NOT NULL,
			unit VARCHAR(20),
			data_type VARCHAR(50),
			collected_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY(device_id, point_id)
		);
		CREATE INDEX IF NOT EXISTS idx_latest_text_device ON device_latest_values_text(device_id);

		-- 布尔最新值表
		CREATE TABLE IF NOT EXISTS device_latest_values_bool (
			device_id INTEGER NOT NULL,
			point_id INTEGER NOT NULL,
			value INTEGER NOT NULL,
			unit VARCHAR(20),
			data_type VARCHAR(50),
			collected_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY(device_id, point_id)
		);
		CREATE INDEX IF NOT EXISTS idx_latest_bool_device ON device_latest_values_bool(device_id);

		-- 设备在线状态缓存表
		CREATE TABLE IF NOT EXISTS device_online_status (
			device_id INTEGER PRIMARY KEY,
			online INTEGER DEFAULT 0,
			last_seen_at DATETIME,
			consecutive_failures INTEGER DEFAULT 0
		);
	`)
	return err
}

func InitOnStartup() error {
	return InitTables()
}
