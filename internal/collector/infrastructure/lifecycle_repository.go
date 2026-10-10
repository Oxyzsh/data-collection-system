package infrastructure

import (
	"database/sql"
	"time"

	"data-collection-system/internal/database"
)

type DeviceCollectorRef struct {
	DeviceID   int
	TemplateID int
}

// ListEnabledDeviceCollectors 返回启动恢复所需的设备引用列表。
// 这里不直接启动采集器，只负责把“哪些设备应该恢复”查出来。
func ListEnabledDeviceCollectors() ([]DeviceCollectorRef, error) {
	db := database.ConfigDB
	if db == nil {
		return nil, sql.ErrConnDone
	}
	rows, err := db.Query("SELECT id, template_id FROM devices WHERE enabled = 1")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]DeviceCollectorRef, 0)
	for rows.Next() {
		var item DeviceCollectorRef
		if err := rows.Scan(&item.DeviceID, &item.TemplateID); err == nil {
			items = append(items, item)
		}
	}
	return items, rows.Err()
}

// ListRunningEnabledDeviceIDsByTemplate 查询某模板下当前应视为“运行中”的设备。
// 模板重建时只需要处理这部分设备，避免无谓重启已停止设备。
func ListRunningEnabledDeviceIDsByTemplate(templateID int) ([]int, error) {
	db := database.ConfigDB
	if db == nil {
		return nil, sql.ErrConnDone
	}
	rows, err := db.Query("SELECT id FROM devices WHERE template_id = ? AND status = 'running' AND enabled = 1", templateID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := make([]int, 0)
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err == nil {
			ids = append(ids, id)
		}
	}
	return ids, rows.Err()
}

// UpdateDeviceStatus 是 collector 基础设施层对 devices.status 的统一回写入口。
func UpdateDeviceStatus(deviceID int, status string, now time.Time) error {
	db := database.ConfigDB
	if db == nil {
		return sql.ErrConnDone
	}
	_, err := db.Exec("UPDATE devices SET status = ?, updated_at = ? WHERE id = ?", status, now.Format("2006-01-02 15:04:05"), deviceID)
	return err
}
