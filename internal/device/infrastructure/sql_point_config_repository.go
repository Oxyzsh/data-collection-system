package infrastructure

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"data-collection-system/internal/database"
	devicedomain "data-collection-system/internal/device/domain"
)

type pointConfigValue struct {
	Enabled      bool   `json:"enabled"`
	AliasEnabled bool   `json:"alias_enabled"`
	AliasZH      string `json:"alias_zh"`
	AliasEN      string `json:"alias_en"`
	ControlLevel int    `json:"control_level"`
}

type SQLPointConfigRepository struct {
	configDB *sql.DB
}

var pointConfigCache = struct {
	mu     sync.RWMutex
	loaded map[int]map[int]*devicedomain.DevicePointConfig
}{
	loaded: make(map[int]map[int]*devicedomain.DevicePointConfig),
}

func NewSQLPointConfigRepository(db *sql.DB) *SQLPointConfigRepository {
	return &SQLPointConfigRepository{configDB: db}
}

func NewDefaultSQLPointConfigRepository() *SQLPointConfigRepository {
	return NewSQLPointConfigRepository(database.ConfigDB)
}

func (r *SQLPointConfigRepository) GetDevicePointConfigs(deviceID int) (map[int]*devicedomain.DevicePointConfig, error) {
	if cached, ok := getCachedPointConfigs(deviceID); ok {
		return clonePointConfigMap(cached), nil
	}
	cfgs, err := r.loadDevicePointConfigs(deviceID)
	if err != nil {
		return nil, err
	}
	setCachedPointConfigs(deviceID, cfgs)
	return clonePointConfigMap(cfgs), nil
}

func (r *SQLPointConfigRepository) SaveDevicePointConfig(cfg *devicedomain.DevicePointConfig, nowStr string) error {
	cfg.ControlLevel = devicedomain.NormalizeControlLevel(cfg.ControlLevel)
	valueJSON, err := json.Marshal(pointConfigValue{
		Enabled:      cfg.Enabled,
		AliasEnabled: cfg.AliasEnabled,
		AliasZH:      cfg.AliasZH,
		AliasEN:      cfg.AliasEN,
		ControlLevel: cfg.ControlLevel,
	})
	if err != nil {
		return err
	}
	enabledInt := 0
	if cfg.Enabled {
		enabledInt = 1
	}
	aliasEnabledInt := 0
	if cfg.AliasEnabled {
		aliasEnabledInt = 1
	}
	_, err = r.configDB.Exec(`
		INSERT INTO cfg_dev_point (device_id, point_id, value_json, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(device_id, point_id) DO UPDATE SET
			value_json = excluded.value_json,
			updated_at = excluded.updated_at
	`, cfg.DeviceID, cfg.PointID, string(valueJSON), nowStr)
	if err != nil {
		return err
	}
	cfg.Enabled = enabledInt == 1
	cfg.AliasEnabled = aliasEnabledInt == 1
	cfg.UpdatedAt = nowStr
	invalidatePointConfigCache(cfg.DeviceID)
	return nil
}

func (r *SQLPointConfigRepository) ReconcileDevicePointConfigs(deviceID int, validPointIDs map[int]struct{}) error {
	if len(validPointIDs) == 0 {
		_, err := r.configDB.Exec("DELETE FROM cfg_dev_point WHERE device_id = ?", deviceID)
		if err == nil {
			invalidatePointConfigCache(deviceID)
		}
		return err
	}

	placeholders := make([]string, 0, len(validPointIDs))
	args := make([]interface{}, 0, len(validPointIDs)+1)
	args = append(args, deviceID)
	for pointID := range validPointIDs {
		placeholders = append(placeholders, "?")
		args = append(args, pointID)
	}

	query := fmt.Sprintf("DELETE FROM cfg_dev_point WHERE device_id = ? AND point_id NOT IN (%s)", strings.Join(placeholders, ","))
	_, err := r.configDB.Exec(query, args...)
	if err == nil {
		invalidatePointConfigCache(deviceID)
	}
	return err
}

func (r *SQLPointConfigRepository) DeleteDevicePointConfig(deviceID, pointID int) error {
	_, err := r.configDB.Exec("DELETE FROM cfg_dev_point WHERE device_id = ? AND point_id = ?", deviceID, pointID)
	if err == nil {
		invalidatePointConfigCache(deviceID)
	}
	return err
}

func (r *SQLPointConfigRepository) loadDevicePointConfigs(deviceID int) (map[int]*devicedomain.DevicePointConfig, error) {
	rows, err := r.configDB.Query(`
		SELECT id, device_id, point_id, value_json, updated_at
		FROM cfg_dev_point
		WHERE device_id = ?
	`, deviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[int]*devicedomain.DevicePointConfig)
	for rows.Next() {
		var (
			cfg       devicedomain.DevicePointConfig
			valueJSON sql.NullString
		)
		if err := rows.Scan(&cfg.ID, &cfg.DeviceID, &cfg.PointID, &valueJSON, &cfg.UpdatedAt); err != nil {
			continue
		}
		payload := pointConfigValue{Enabled: true, ControlLevel: 1}
		if valueJSON.Valid && valueJSON.String != "" {
			_ = json.Unmarshal([]byte(valueJSON.String), &payload)
		}
		cfg.Enabled = payload.Enabled
		cfg.AliasEnabled = payload.AliasEnabled
		cfg.AliasZH = payload.AliasZH
		cfg.AliasEN = payload.AliasEN
		cfg.ControlLevel = devicedomain.NormalizeControlLevel(payload.ControlLevel)
		result[cfg.PointID] = &cfg
	}
	return result, rows.Err()
}

func getCachedPointConfigs(deviceID int) (map[int]*devicedomain.DevicePointConfig, bool) {
	pointConfigCache.mu.RLock()
	defer pointConfigCache.mu.RUnlock()
	cfgs, ok := pointConfigCache.loaded[deviceID]
	return cfgs, ok
}

func setCachedPointConfigs(deviceID int, cfgs map[int]*devicedomain.DevicePointConfig) {
	pointConfigCache.mu.Lock()
	pointConfigCache.loaded[deviceID] = clonePointConfigMap(cfgs)
	pointConfigCache.mu.Unlock()
}

func invalidatePointConfigCache(deviceID int) {
	pointConfigCache.mu.Lock()
	delete(pointConfigCache.loaded, deviceID)
	pointConfigCache.mu.Unlock()
}

func clonePointConfigMap(src map[int]*devicedomain.DevicePointConfig) map[int]*devicedomain.DevicePointConfig {
	if len(src) == 0 {
		return map[int]*devicedomain.DevicePointConfig{}
	}
	dst := make(map[int]*devicedomain.DevicePointConfig, len(src))
	for k, v := range src {
		if v == nil {
			continue
		}
		copied := *v
		dst[k] = &copied
	}
	return dst
}
