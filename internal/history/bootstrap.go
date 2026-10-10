package history

import (
	"database/sql"
	"fmt"

	"data-collection-system/internal/database"
)

// InitTables 创建采集历史相关表：
// - device_data：采集值历史（数值/字符串）；
// - device_daily_extremum：当日极值（日/月/年聚合查询的数据源）。
//
// 说明：裁剪后的进程只做数据采集，不建告警记录、通知历史等表。
func InitTables() error {
	if database.HistoryDB == nil {
		return fmt.Errorf("history db is nil")
	}
	_, err := database.HistoryDB.Exec(`
		-- 历史数据（单表：数值/字符串）
		-- value_type: 0=数值(含布尔0/1), 1=字符串
		CREATE TABLE IF NOT EXISTS device_data (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			device_id INTEGER NOT NULL,
			point_id INTEGER NOT NULL,
			value_type INTEGER NOT NULL DEFAULT 0,
			value_num REAL,
			value_text TEXT,
			unit VARCHAR(20),
			data_type VARCHAR(50),
			collected_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			CHECK (
				(value_type = 0 AND value_num IS NOT NULL AND value_text IS NULL) OR
				(value_type = 1 AND value_num IS NULL AND value_text IS NOT NULL)
			)
		);
		CREATE INDEX IF NOT EXISTS idx_data_device_point_time
			ON device_data(device_id, point_id, collected_at);
		CREATE INDEX IF NOT EXISTS idx_data_collected_at
			ON device_data(collected_at);
		-- 数值查询主路径专用索引（强制 value_type=0）
		CREATE INDEX IF NOT EXISTS idx_data_num_device_point_time
			ON device_data(device_id, point_id, collected_at)
			WHERE value_type = 0;

		CREATE TABLE IF NOT EXISTS device_daily_extremum (
			device_id INTEGER NOT NULL,
			point_id  INTEGER NOT NULL,
			date      TEXT NOT NULL,
			min_value REAL,
			max_value REAL,
			unit      VARCHAR(20),
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(device_id, point_id, date)
		);
	`)
	if err != nil {
		return err
	}
	return ensureDeviceDataUniqueIndex(database.HistoryDB)
}

// ensureDeviceDataUniqueIndex 保证 device_data 具备 UPSERT 需要的唯一索引。
// 存量库缺索引且已有数据时返回错误——由 main.go 的 logger.Fatal 拒绝启动。
func ensureDeviceDataUniqueIndex(db *sql.DB) error {
	ready, err := hasDeviceDataUniqueIndex(db)
	if err != nil {
		return err
	}
	if ready {
		return nil
	}
	var rows int64
	if err := db.QueryRow(`SELECT COUNT(*) FROM device_data`).Scan(&rows); err != nil {
		return err
	}
	if rows > 0 {
		return fmt.Errorf("device_data 缺少唯一索引且库内已有 %d 行数据：请先清理历史库或完成去重与唯一索引迁移，再重新启动", rows)
	}
	if _, err := db.Exec(`DROP INDEX IF EXISTS idx_data_device_point_time`); err != nil {
		return err
	}
	_, err = db.Exec(`CREATE UNIQUE INDEX idx_data_device_point_time ON device_data(device_id, point_id, collected_at)`)
	return err
}

// hasDeviceDataUniqueIndex 读 PRAGMA index_list 判断唯一索引是否就绪。
func hasDeviceDataUniqueIndex(db *sql.DB) (bool, error) {
	rows, err := db.Query(`PRAGMA index_list(device_data)`)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return false, err
	}
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return false, err
		}
		// 找到列名为 name / unique 的下标（避免依赖列序）
		var name string
		var uniq int64
		for i, c := range cols {
			switch c {
			case "name":
				if s, ok := vals[i].(string); ok {
					name = s
				}
			case "unique":
				if v, ok := vals[i].(int64); ok {
					uniq = v
				}
			}
		}
		if name == "idx_data_device_point_time" && uniq == 1 {
			return true, nil
		}
	}
	return false, rows.Err()
}

func InitOnStartup() error {
	return InitTables()
}
