package infrastructure

import (
	"database/sql"
	"sort"
	"sync"

	alarmdomain "data-collection-system/internal/templatealarm"
	templatebiz "data-collection-system/internal/template/domain"
	"data-collection-system/internal/valueobj"
)

type templateCache struct {
	templates map[int]*templatebiz.TemplateCacheItem
	mutex     sync.RWMutex
}

var cacheInst *templateCache

func InitTemplateCache() {
	cacheInst = &templateCache{templates: make(map[int]*templatebiz.TemplateCacheItem)}
}

func ensureCache() {
	if cacheInst == nil {
		InitTemplateCache()
	}
}

func GetTemplateFromCache(templateID int) (*templatebiz.TemplateCacheItem, bool) {
	if cacheInst == nil {
		return nil, false
	}
	cacheInst.mutex.RLock()
	defer cacheInst.mutex.RUnlock()
	item, ok := cacheInst.templates[templateID]
	if !ok {
		return nil, false
	}
	return item, true
}

// SetTemplateToCacheWithMethod 把模板写入内存缓存。
// 这里会自动补上在线状态点和离线告警，保证运行时视图完整。
// collectMethod 参数允许指定采集方式，默认为 "simulator"，但不强制要求。
func SetTemplateToCacheWithMethod(templateID int, tmpl *templatebiz.DeviceTemplate, collectMethod string, points []templatebiz.DevicePoint, alarms []alarmdomain.DeviceAlarm) {
	ensureCache()
	if collectMethod == "" {
		collectMethod = "simulator"
	}

	cacheInst.mutex.Lock()
	defer cacheInst.mutex.Unlock()

	points = normalizeCachedPoints(points)
	allPoints := append([]templatebiz.DevicePoint{createOnlineStatusPoint()}, points...)
	allAlarms := append([]alarmdomain.DeviceAlarm{createOfflineAlarm()}, alarms...)

	cacheInst.templates[templateID] = &templatebiz.TemplateCacheItem{
		Template:      tmpl,
		CollectMethod: collectMethod,
		Points:        allPoints,
		Alarms:        allAlarms,
	}
}

// DeleteTemplateFromCache 从内存缓存移除模板。
func DeleteTemplateFromCache(templateID int) {
	if cacheInst == nil {
		return
	}
	cacheInst.mutex.Lock()
	defer cacheInst.mutex.Unlock()
	delete(cacheInst.templates, templateID)
}

// GetAllCachedTemplates 返回所有模板主信息，按 ID 排序。
func GetAllCachedTemplates() []*templatebiz.DeviceTemplate {
	if cacheInst == nil {
		return nil
	}
	cacheInst.mutex.RLock()
	defer cacheInst.mutex.RUnlock()

	items := make([]*templatebiz.DeviceTemplate, 0, len(cacheInst.templates))
	for _, item := range cacheInst.templates {
		items = append(items, item.Template)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	return items
}

// LoadTemplatesFromDB 从数据库加载全部模板并写入缓存。
func LoadTemplatesFromDB(db *sql.DB) error {
	rows, err := db.Query("SELECT id, name, type, collect_method, is_built_in, description, data, created_at, updated_at FROM device_templates")
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var id int
		var name, templateType, collectMethod, description, data, createdAt, updatedAt string
		var isBuiltIn int
		if err := rows.Scan(&id, &name, &templateType, &collectMethod, &isBuiltIn, &description, &data, &createdAt, &updatedAt); err != nil {
			return err
		}

		td, err := templatebiz.Deserialize(data)
		if err != nil {
			continue
		}
		// 回源数据先反序列化，再做一次模板语义校验。
		if err := templatebiz.ValidatePointsForTemplateType(templateType, td.Points); err != nil {
			return err
		}
		if templateType != "modbus" {
			for i := range td.Points {
				if td.Points[i].Modbus != nil && td.Points[i].Modbus.RegisterAddress == 0 {
					td.Points[i].Modbus.RegisterAddress = -1
				}
			}
		}

		tmpl := &templatebiz.DeviceTemplate{
			ID:          id,
			Name:        name,
			Type:        templateType,
			IsBuiltIn:   isBuiltIn == 1,
			Description: description,
			CreatedAt:   createdAt,
			UpdatedAt:   updatedAt,
		}
		SetTemplateToCacheWithMethod(id, tmpl, collectMethod, td.Points, td.Alarms)
	}
	return rows.Err()
}

// LoadTemplateFromDB 回源单个模板并写入缓存。
func LoadTemplateFromDB(db *sql.DB, templateID int) error {
	var id int
	var name, templateType, collectMethod, description, data, createdAt, updatedAt string
	var isBuiltIn int
	err := db.QueryRow("SELECT id, name, type, collect_method, is_built_in, description, data, created_at, updated_at FROM device_templates WHERE id = ?", templateID).
		Scan(&id, &name, &templateType, &collectMethod, &isBuiltIn, &description, &data, &createdAt, &updatedAt)
	if err != nil {
		return err
	}

	td, err := templatebiz.Deserialize(data)
	if err != nil {
		return err
	}
	// 单条回源也走同样的校验，避免缓存混入非法模板。
	if err := templatebiz.ValidatePointsForTemplateType(templateType, td.Points); err != nil {
		return err
	}
	if templateType != "modbus" {
		for i := range td.Points {
			if td.Points[i].Modbus != nil && td.Points[i].Modbus.RegisterAddress == 0 {
				td.Points[i].Modbus.RegisterAddress = -1
			}
		}
	}

	tmpl := &templatebiz.DeviceTemplate{
		ID:          id,
		Name:        name,
		Type:        templateType,
		IsBuiltIn:   isBuiltIn == 1,
		Description: description,
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	}
	SetTemplateToCacheWithMethod(id, tmpl, collectMethod, td.Points, td.Alarms)
	return nil
}

// GetTemplateFromCacheOrLoad 先查缓存，未命中再回源数据库。
func GetTemplateFromCacheOrLoad(db *sql.DB, templateID int) (*templatebiz.TemplateCacheItem, bool) {
	if item, ok := GetTemplateFromCache(templateID); ok {
		return item, true
	}
	if err := LoadTemplateFromDB(db, templateID); err != nil {
		return nil, false
	}
	return GetTemplateFromCache(templateID)
}

// normalizeCachedPoints 统一缓存里的测点类型。
func normalizeCachedPoints(points []templatebiz.DevicePoint) []templatebiz.DevicePoint {
	if len(points) == 0 {
		return points
	}
	out := make([]templatebiz.DevicePoint, 0, len(points))
	for _, p := range points {
		if p.Type == "" {
			p.Type = templatebiz.PointTypePOINT
		}
		out = append(out, p)
	}
	return out
}

// createOnlineStatusPoint 创建内置的设备在线状态点。
func createOnlineStatusPoint() templatebiz.DevicePoint {
	return templatebiz.DevicePoint{
		PointID:        0,
		Type:           templatebiz.PointTypePOINT,
		Name:           "设备在线状态",
		NameEN:         "Device Online Status",
		DataType:       valueobj.DataTypeINT64,
		Precision:      0,
		Access:         "r",
		Enabled:        1,
		ValueMappingCN: "0:离线,1:在线",
		ValueMappingEN: "0:offline,1:online",
		Description:    "设备在线状态，0表示离线，1表示在线",
	}
}

// createOfflineAlarm 创建内置的设备离线告警。
// 0.5/1.0 是为了复用通用区间引擎并形成 0/1 状态滞回；
// 对外表达式会在 alarm/domain 层还原为 =0 / =1，不能按普通模拟量展示。
func createOfflineAlarm() alarmdomain.DeviceAlarm {
	startLowLimit := float64(0.5)
	endLowLimit := float64(1.0)
	return alarmdomain.DeviceAlarm{
		ID:            0,
		PointID:       0,
		Name:          "设备离线告警",
		NameEN:        "Device Offline Alarm",
		Level:         alarmdomain.AlarmLevelEMERG,
		Enabled:       true,
		StartLowLimit: &startLowLimit,
		EndLowLimit:   &endLowLimit,
		Description:   "设备离线时触发告警",
	}
}

// copyAlarms 复制告警切片，避免缓存和调用方共享同一底层数组。
func copyAlarms(alarms []alarmdomain.DeviceAlarm) []alarmdomain.DeviceAlarm {
	if len(alarms) == 0 {
		return nil
	}
	out := make([]alarmdomain.DeviceAlarm, len(alarms))
	copy(out, alarms)
	return out
}
