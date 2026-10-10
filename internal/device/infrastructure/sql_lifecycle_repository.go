package infrastructure

import (
	"database/sql"
	"time"

	devicedomain "data-collection-system/internal/device/domain"
)

type SQLLifecycleRepository struct {
	db *sql.DB
}

func NewSQLLifecycleRepository(db *sql.DB) *SQLLifecycleRepository {
	return &SQLLifecycleRepository{db: db}
}

func (r *SQLLifecycleRepository) GetDeviceLifecycleState(deviceID int) (devicedomain.DeviceLifecycleState, error) {
	var state devicedomain.DeviceLifecycleState
	var enabled int
	err := r.db.QueryRow("SELECT id, name, template_id, status, enabled FROM devices WHERE id = ?", deviceID).
		Scan(&state.ID, &state.Name, &state.TemplateID, &state.Status, &enabled)
	state.Enabled = enabled == 1
	return state, err
}

func (r *SQLLifecycleRepository) UpdateDeviceStatus(deviceID int, status string, now time.Time) error {
	nowStr := now.Format("2006-01-02 15:04:05")
	_, err := r.db.Exec("UPDATE devices SET status = ?, updated_at = ? WHERE id = ?", status, nowStr, deviceID)
	return err
}

func (r *SQLLifecycleRepository) UpdateDeviceEnabled(deviceID int, enabled bool, now time.Time) error {
	nowStr := now.Format("2006-01-02 15:04:05")
	_, err := r.db.Exec("UPDATE devices SET enabled = ?, updated_at = ? WHERE id = ?", enabled, nowStr, deviceID)
	return err
}

func (r *SQLLifecycleRepository) UpdateDeviceEnabledAndStatus(deviceID int, enabled bool, status string, now time.Time) error {
	nowStr := now.Format("2006-01-02 15:04:05")
	_, err := r.db.Exec("UPDATE devices SET enabled = ?, status = ?, updated_at = ? WHERE id = ?", enabled, status, nowStr, deviceID)
	return err
}

func (r *SQLLifecycleRepository) DeleteDeviceCascade(deviceID int) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}

	if _, err := tx.Exec("DELETE FROM cfg_dev_warn WHERE device_id = ?", deviceID); err != nil {
		_ = tx.Rollback()
		return err
	}
	if _, err := tx.Exec("DELETE FROM cfg_dev_point WHERE device_id = ?", deviceID); err != nil {
		_ = tx.Rollback()
		return err
	}
	if _, err := tx.Exec("DELETE FROM device_params WHERE device_id = ?", deviceID); err != nil {
		_ = tx.Rollback()
		return err
	}
	result, err := tx.Exec("DELETE FROM devices WHERE id = ?", deviceID)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		_ = tx.Rollback()
		return sql.ErrNoRows
	}
	return tx.Commit()
}
