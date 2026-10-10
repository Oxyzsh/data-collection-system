package infrastructure

import (
	"database/sql"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	devicedomain "data-collection-system/internal/device/domain"
)

type SQLConfigRepository struct {
	db *sql.DB
}

// SQLConfigRepository 是设备配置写入场景的 SQL 实现。
// application 层只知道它满足 DeviceConfigRepository 接口，
// 不关心底层到底是 SQLite、MySQL 还是别的存储。
func NewSQLConfigRepository(db *sql.DB) *SQLConfigRepository {
	return &SQLConfigRepository{db: db}
}

// GetTemplateMeta 查询配置校验需要的模板信息。
// local_io 模板还需要从 data JSON 提取逻辑通道和读写方向，供设备保存前做通道校验。
func (r *SQLConfigRepository) GetTemplateMeta(templateID int) (devicedomain.TemplateMeta, error) {
	var meta devicedomain.TemplateMeta
	var data string
	err := r.db.QueryRow("SELECT type, collect_method, data FROM device_templates WHERE id = ?", templateID).
		Scan(&meta.Type, &meta.CollectMethod, &data)
	if err != nil {
		return meta, err
	}
	if strings.EqualFold(strings.TrimSpace(meta.Type), "local_io") ||
		strings.EqualFold(strings.TrimSpace(meta.Type), "local_di") ||
		strings.EqualFold(strings.TrimSpace(meta.Type), "local_do") {
		var payload struct {
			Points []struct {
				Access  string `json:"access"`
				LocalIO *struct {
					Channel string `json:"channel"`
				} `json:"local_io"`
			} `json:"points"`
		}
		if data != "" {
			if err := json.Unmarshal([]byte(data), &payload); err != nil {
				return meta, err
			}
		}
		for _, point := range payload.Points {
			if point.LocalIO == nil || strings.TrimSpace(point.LocalIO.Channel) == "" {
				continue
			}
			meta.LocalIOChannels = append(meta.LocalIOChannels, devicedomain.TemplateIOPointMeta{
				Channel: point.LocalIO.Channel,
				Access:  point.Access,
			})
		}
	}
	return meta, nil
}

func (r *SQLConfigRepository) GetDeviceSnapshot(deviceID int) (devicedomain.DeviceConfigSnapshot, error) {
	var snapshot devicedomain.DeviceConfigSnapshot
	err := r.db.QueryRow("SELECT template_id, status FROM devices WHERE id = ?", deviceID).
		Scan(&snapshot.TemplateID, &snapshot.Status)
	return snapshot, err
}

// GetDeviceFullSnapshot 返回设备完整配置（主信息 + 参数），供复制设备使用。
func (r *SQLConfigRepository) GetDeviceFullSnapshot(deviceID int) (devicedomain.DeviceConfigFull, error) {
	var full devicedomain.DeviceConfigFull
	if err := r.db.QueryRow(
		"SELECT name, template_id, status, description FROM devices WHERE id = ?", deviceID,
	).Scan(&full.Name, &full.TemplateID, &full.Status, &full.Description); err != nil {
		return full, err
	}

	rows, err := r.db.Query("SELECT param_name, param_value FROM device_params WHERE device_id = ?", deviceID)
	if err != nil {
		return full, err
	}
	defer rows.Close()

	full.Params = make(map[string]string)
	for rows.Next() {
		var name, value string
		if err := rows.Scan(&name, &value); err != nil {
			return full, err
		}
		full.Params[name] = value
	}
	return full, rows.Err()
}

// InsertDeviceWithParams 用事务把 devices 主表和 device_params 子表一起写入。
// 这里用事务的原因很直接：两张表要么都成功，要么都失败，避免留下半套配置。
func (r *SQLConfigRepository) InsertDeviceWithParams(name string, templateID int, description string, params map[string]string, now time.Time) (int64, error) {
	nowStr := now.Format("2006-01-02 15:04:05")
	tx, err := r.db.Begin()
	if err != nil {
		return 0, err
	}

	result, err := tx.Exec(
		"INSERT INTO devices (name, template_id, status, description, created_at, updated_at, enabled) VALUES (?, ?, ?, ?, ?, ?, ?)",
		name, templateID, "stopped", description, nowStr, nowStr, 0,
	)
	if err != nil {
		_ = tx.Rollback()
		return 0, err
	}

	deviceID, err := result.LastInsertId()
	if err != nil {
		_ = tx.Rollback()
		return 0, err
	}

	if err := replaceDeviceParamsTx(tx, strconv.FormatInt(deviceID, 10), params, nowStr); err != nil {
		_ = tx.Rollback()
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}

	return deviceID, nil
}

// SaveDeviceConfig 同时更新设备主信息和参数表。
// application 已经完成了参数规范化和业务校验，repository 这里只负责可靠落库。
func (r *SQLConfigRepository) SaveDeviceConfig(deviceID int, name string, templateID int, description string, params map[string]string, now time.Time) error {
	nowStr := now.Format("2006-01-02 15:04:05")
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}

	if _, err := tx.Exec(
		"UPDATE devices SET name = ?, template_id = ?, description = ?, updated_at = ? WHERE id = ?",
		name, templateID, description, nowStr, deviceID,
	); err != nil {
		_ = tx.Rollback()
		return err
	}

	if err := replaceDeviceParamsTx(tx, strconv.Itoa(deviceID), params, nowStr); err != nil {
		_ = tx.Rollback()
		return err
	}

	return tx.Commit()
}

func (r *SQLConfigRepository) SaveDeviceParams(deviceID int, params map[string]string, now time.Time) error {
	nowStr := now.Format("2006-01-02 15:04:05")
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}

	if err := replaceDeviceParamsTx(tx, strconv.Itoa(deviceID), params, nowStr); err != nil {
		_ = tx.Rollback()
		return err
	}

	return tx.Commit()
}

// replaceDeviceParamsTx 用“先删后插”的方式重建参数表。
// 这种方式实现简单、结果明确，适合设备参数量不大且整体覆盖写入的场景。
func replaceDeviceParamsTx(tx *sql.Tx, id string, params map[string]string, now string) error {
	if _, err := tx.Exec("DELETE FROM device_params WHERE device_id = ?", id); err != nil {
		return err
	}

	for paramName, paramValue := range params {
		if _, err := tx.Exec(
			"INSERT INTO device_params (device_id, param_name, param_value, created_at, updated_at) VALUES (?, ?, ?, ?, ?)",
			id, paramName, paramValue, now, now,
		); err != nil {
			return err
		}
	}

	return nil
}
