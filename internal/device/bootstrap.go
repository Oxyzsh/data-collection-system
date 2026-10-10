package device

import (
	"fmt"
	"time"

	"data-collection-system/internal/database"
	"data-collection-system/internal/shared/logger"
)

func InitTables() error {
	if database.ConfigDB == nil {
		return fmt.Errorf("config db is nil")
	}
	_, err := database.ConfigDB.Exec(`
		-- 设备表
		CREATE TABLE IF NOT EXISTS devices (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name VARCHAR(100) NOT NULL,
			template_id INTEGER NOT NULL,
			status VARCHAR(20) DEFAULT 'stopped',
			enabled INTEGER DEFAULT 1,
			description TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (template_id) REFERENCES device_templates(id)
		);
		CREATE INDEX IF NOT EXISTS idx_devices_template ON devices(template_id);
		CREATE INDEX IF NOT EXISTS idx_devices_status ON devices(status, enabled);
		CREATE INDEX IF NOT EXISTS idx_devices_enabled ON devices(enabled);

		-- 设备参数表
		CREATE TABLE IF NOT EXISTS device_params (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			device_id INTEGER NOT NULL,
			param_name VARCHAR(100) NOT NULL,
			param_value VARCHAR(255),
			description TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (device_id) REFERENCES devices(id) ON DELETE CASCADE,
			UNIQUE(device_id, param_name)
		);
		CREATE INDEX IF NOT EXISTS idx_device_params_device ON device_params(device_id);

		-- 设备测点配置表
		CREATE TABLE IF NOT EXISTS cfg_dev_point (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			device_id INTEGER NOT NULL,
			point_id INTEGER NOT NULL,
			value_json TEXT NOT NULL DEFAULT '{}',
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(device_id, point_id)
		);
		CREATE INDEX IF NOT EXISTS idx_cfg_dev_point_device ON cfg_dev_point(device_id);
		CREATE INDEX IF NOT EXISTS idx_cfg_dev_point_point ON cfg_dev_point(point_id);
	`)
	return err
}

func InitOnStartup() error {
	return InitTables()
}

// EnsureSystemCollectorDevice 确保系统采集器设备存在。
// 它属于启动期的“自修复/自补齐”逻辑：如果系统依赖某个内置设备才能工作，
// 那么启动时就自动查缺补漏，而不是把这个责任交给用户手工配置。
func EnsureSystemCollectorDevice() error {
	db := database.ConfigDB

	var deviceID int
	err := db.QueryRow("SELECT id FROM devices WHERE name = ?", "系统采集器").Scan(&deviceID)
	if err == nil && deviceID > 0 {
		// 设备已存在时只做最小修复：确保 enabled/status 处于可运行状态。
		var enabled int
		_ = db.QueryRow("SELECT enabled FROM devices WHERE id = ?", deviceID).Scan(&enabled)
		if enabled == 0 {
			if _, err = db.Exec("UPDATE devices SET enabled = 1, status = ? WHERE id = ?", "running", deviceID); err != nil {
				return fmt.Errorf("启用系统采集器设备失败: %v", err)
			}
			logger.Infof("[EnsureSystemCollectorDevice] 启用现有系统采集器设备 (ID: %d)", deviceID)
		} else {
			logger.Infof("[EnsureSystemCollectorDevice] 系统采集器设备已存在且已启用 (ID: %d)", deviceID)
		}
		return nil
	}

	// 设备不存在时，再去找内置模板并补建一条设备记录。
	var templateID int
	if err = db.QueryRow("SELECT id FROM device_templates WHERE name = ? AND is_built_in = 1", "采集器").Scan(&templateID); err != nil {
		return fmt.Errorf("找不到采集器模板: %v", err)
	}

	now := time.Now().Format("2006-01-02 15:04:05")
	result, err := db.Exec(
		"INSERT INTO devices (name, template_id, status, description, enabled, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
		"系统采集器", templateID, "running", "自动创建的系统信息采集设备", 1, now, now,
	)
	if err != nil {
		return fmt.Errorf("创建系统采集器设备失败: %v", err)
	}

	newDeviceID, _ := result.LastInsertId()
	logger.Infof("[EnsureSystemCollectorDevice] 创建系统采集器设备 (ID: %d, 模板ID: %d)", newDeviceID, templateID)
	return nil
}
