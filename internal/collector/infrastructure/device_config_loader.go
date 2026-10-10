package infrastructure

import (
	"fmt"
	"strconv"

	"data-collection-system/internal/database"
	deviceinfra "data-collection-system/internal/device/infrastructure"
	"data-collection-system/internal/shared/logger"
	templatebiz "data-collection-system/internal/template/domain"
	templateruntime "data-collection-system/internal/template/runtime"
)

// LoadedDeviceConfig 为采集运行编排准备的设备启动配置快照。
// 它是“启动一个 DeviceCollector 真正需要的最小集合”：
// - 设备名；
// - 模板点位/告警；
// - 设备参数；
// - 采集周期和重试策略；
// - 测点启停覆盖配置。
type LoadedDeviceConfig struct {
	DeviceName    string
	CollectMethod string
	Points        []templatebiz.DevicePoint
	Params        map[string]string
	Interval      int
	MaxRetries    int
	PointEnabled  map[int]bool
}

// LoadDeviceConfig 把设备配置、模板缓存、点位配置、告警覆盖配置装配成启动快照。
// 这是 collector/runtime 启动前的重要桥接点：
// 上游世界还是“数据库记录 + 模板缓存 + 用户覆盖配置”，
// 下游世界就要变成“可直接驱动采集循环的内存结构”。
func LoadDeviceConfig(deviceID, templateID int) (LoadedDeviceConfig, error) {
	db := database.ConfigDB
	if db == nil {
		return LoadedDeviceConfig{}, fmt.Errorf("config db not ready")
	}

	deviceName := fmt.Sprintf("设备%d", deviceID)
	if err := db.QueryRow("SELECT name FROM devices WHERE id = ?", deviceID).Scan(&deviceName); err != nil {
		logger.Warnf("[CollectorInfra] 查询设备 %d 名称失败: %v", deviceID, err)
	}

	// 模板点位和告警优先从模板 runtime 缓存拿。
	// 这样可以避免每次启动设备都重新拼模板详情。
	var collectMethod string
	var points []templatebiz.DevicePoint
	if cfg, err := templateruntime.GetTemplateConfig(templateID); err == nil {
		collectMethod = cfg.CollectMethod
		points = cfg.Points
	}
	if collectMethod == "" {
		if err := db.QueryRow("SELECT collect_method FROM device_templates WHERE id = ?", templateID).Scan(&collectMethod); err != nil || collectMethod == "" {
			collectMethod = "simulator"
		}
	}

	// 设备参数来自 device_params，属于“同模板下不同设备可不同”的那部分配置。
	params := make(map[string]string)
	rows, err := db.Query("SELECT param_name, param_value FROM device_params WHERE device_id = ?", deviceID)
	if err != nil {
		logger.Warnf("[CollectorInfra] 查询设备 %d 参数失败: %v，设备将以默认参数启动", deviceID, err)
	} else {
		defer rows.Close()
		for rows.Next() {
			var name, value string
			if rows.Scan(&name, &value) == nil {
				params[name] = value
			}
		}
	}

	interval := ResolveCollectorInterval(1000, params)
	maxRetries := 0
	if v, ok := params["retry_cnt"]; ok {
		if r, err := strconv.Atoi(v); err == nil && r >= 0 {
			maxRetries = r
		}
	}

	return LoadedDeviceConfig{
		DeviceName:    deviceName,
		CollectMethod: collectMethod,
		Points:        points,
		Params:        params,
		Interval:      interval,
		MaxRetries:    maxRetries,
		PointEnabled:  BuildPointEnabledMap(deviceID, points),
	}, nil
}

// BuildPointEnabledMap 先以模板默认 Enabled 为基线，再叠加设备级点位配置覆盖值。
func BuildPointEnabledMap(deviceID int, points []templatebiz.DevicePoint) map[int]bool {
	enabledMap := make(map[int]bool, len(points))
	validPointIDs := make(map[int]struct{}, len(points))
	for _, p := range points {
		enabledMap[p.PointID] = p.Enabled != 0
		validPointIDs[p.PointID] = struct{}{}
	}

	repo := deviceinfra.NewDefaultSQLPointConfigRepository()
	_ = repo.ReconcileDevicePointConfigs(deviceID, validPointIDs)
	cfgs, err := repo.GetDevicePointConfigs(deviceID)
	if err != nil {
		return enabledMap
	}
	for pointID, cfg := range cfgs {
		enabledMap[pointID] = cfg.Enabled
	}
	return enabledMap
}

// ResolveCollectorInterval 从设备参数中解析采集周期。
// 这里故意保留默认值回退，避免因为配置错误导致采集器完全无法启动。
func ResolveCollectorInterval(defaultInterval int, params map[string]string) int {
	if params == nil {
		return defaultInterval
	}
	if v, ok := params["interval"]; ok {
		if interval, err := strconv.Atoi(v); err == nil && interval > 0 {
			return interval
		}
		logger.Warnf("[CollectorInfra] interval 参数无效，使用默认值：%s", v)
	}
	return defaultInterval
}
