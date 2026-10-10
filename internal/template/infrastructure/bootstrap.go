package infrastructure

import (
	"database/sql"
	"fmt"
	"runtime"
	"time"

	alarmdomain "data-collection-system/internal/templatealarm"
	"data-collection-system/internal/database"
	"data-collection-system/internal/shared/logger"
	templatebiz "data-collection-system/internal/template/domain"
	"data-collection-system/internal/valueobj"

	"github.com/shirou/gopsutil/v3/cpu"
)

// builtInTemplateIDMax 是内置模板的 ID 预留上限：内置模板只用 1..1000（ID 0 留给指标模板），
// 导入/新建模板一律从 1001 起分配，两边互不消耗。
// 取 1000 是为了和历史动环的约定对齐（老动环 <1000 是内置、1000 起是导入的外部模板）。
const builtInTemplateIDMax = 1000

type BuiltInTemplateDef struct {
	Name          string
	Type          string
	CollectMethod string
	Description   string
	Points        []templatebiz.DevicePoint
	Alarms        []alarmdomain.DeviceAlarm
}

// BuildSystemCollectorTemplate 构造内置系统采集器模板。
// 它会根据当前机器 CPU 核数动态生成核心点位。
func BuildSystemCollectorTemplate() BuiltInTemplateDef {
	coreCount := getCPUCoreCount()
	points := make([]templatebiz.DevicePoint, 0, coreCount+3)

	for i := 1; i <= coreCount; i++ {
		points = append(points, templatebiz.DevicePoint{
			PointID:     i,
			Type:        templatebiz.PointTypePOINT,
			Name:        fmt.Sprintf("CPU核心%d", i),
			NameEN:      fmt.Sprintf("CPU Core %d", i),
			DataType:    valueobj.DataTypeFLOAT64,
			Unit:        "%",
			UnitEN:      "%",
			Precision:   2,
			Access:      "r",
			Enabled:     1,
			Description: fmt.Sprintf("CPU核心%d使用率", i),
		})
	}

	cpuTotalPointID := coreCount + 1
	memPointID := coreCount + 2
	diskPointID := coreCount + 3

	points = append(points,
		templatebiz.DevicePoint{PointID: cpuTotalPointID, Type: templatebiz.PointTypePOINT, Name: "CPU总使用率", NameEN: "CPU Total Usage", DataType: valueobj.DataTypeFLOAT64, Unit: "%", UnitEN: "%", Precision: 2, Access: "r", Enabled: 1, Description: "CPU平均使用率"},
		templatebiz.DevicePoint{PointID: memPointID, Type: templatebiz.PointTypePOINT, Name: "内存使用率", NameEN: "Memory Usage", DataType: valueobj.DataTypeFLOAT64, Unit: "%", UnitEN: "%", Precision: 2, Access: "r", Enabled: 1, Description: "内存使用率"},
		templatebiz.DevicePoint{PointID: diskPointID, Type: templatebiz.PointTypePOINT, Name: "磁盘使用率", NameEN: "Disk Usage", DataType: valueobj.DataTypeFLOAT64, Unit: "%", UnitEN: "%", Precision: 2, Access: "r", Enabled: 1, Description: "根分区磁盘使用率"},
	)

	return BuiltInTemplateDef{
		Name:          "采集器",
		Type:          "系统",
		CollectMethod: "system",
		Description:   "系统信息采集器（CPU、内存、磁盘）",
		Points:        points,
		Alarms: []alarmdomain.DeviceAlarm{
			{ID: 1, PointID: cpuTotalPointID, Name: "CPU使用率过高", NameEN: "CPU Usage High", Level: alarmdomain.AlarmLevelEMERG, Enabled: true, StartHighLimit: floatPtr(90.0), Description: "CPU使用率过高"},
			{ID: 2, PointID: memPointID, Name: "内存使用率过高", NameEN: "Memory Usage High", Level: alarmdomain.AlarmLevelWARN, Enabled: true, StartHighLimit: floatPtr(85.0), Description: "内存使用率过高"},
			{ID: 3, PointID: diskPointID, Name: "磁盘使用率过高", NameEN: "Disk Usage High", Level: alarmdomain.AlarmLevelWARN, Enabled: true, StartHighLimit: floatPtr(85.0), Description: "磁盘使用率过高"},
		},
	}
}

// BuildSimulatorTempHumTemplate 构造内置模拟温湿度模板，风格与 BuildSystemCollectorTemplate 保持一致。
func BuildSimulatorTempHumTemplate() BuiltInTemplateDef {
	return BuiltInTemplateDef{
		Name:          "模拟温湿度",
		Type:          "模拟",
		CollectMethod: "simulator",
		Description:   "内置模拟温湿度模板",
		Points: []templatebiz.DevicePoint{
			{PointID: 1, Type: templatebiz.PointTypePOINT, Name: "温度", NameEN: "Temperature", DataType: valueobj.DataTypeFLOAT64, Unit: "℃", UnitEN: "C", Precision: 2, Access: "r", Enabled: 1, Description: "温度值"},
			{PointID: 2, Type: templatebiz.PointTypePOINT, Name: "湿度", NameEN: "Humidity", DataType: valueobj.DataTypeFLOAT64, Unit: "%", UnitEN: "%", Precision: 2, Access: "r", Enabled: 1, Description: "湿度值"},
			{PointID: 3, Type: templatebiz.PointTypePOINT, Name: "设定温度", NameEN: "Set Temperature", DataType: valueobj.DataTypeFLOAT64, Unit: "℃", UnitEN: "C", Precision: 2, Access: "rw", Enabled: 1, Description: "设定温度值"},
			{PointID: 4, Type: templatebiz.PointTypePOINT, Name: "控制模式", NameEN: "Control Mode", DataType: valueobj.DataTypeINT64, Unit: "", UnitEN: "", Precision: 0, Access: "rw", Enabled: 1, Description: "控制模式 (0:自动, 1:手动)"},
		},
		Alarms: []alarmdomain.DeviceAlarm{
			{ID: 1, PointID: 1, Name: "温度过高告警", NameEN: "Temperature High Alarm", Level: alarmdomain.AlarmLevelWARN, Enabled: true, StartHighLimit: floatPtr(60.0), EndLowLimit: floatPtr(35.0), Description: "温度过高告警"},
			{ID: 2, PointID: 2, Name: "湿度过高告警", NameEN: "Humidity High Alarm", Level: alarmdomain.AlarmLevelNOTICE, Enabled: true, StartHighLimit: floatPtr(80.0), EndLowLimit: floatPtr(60.0), Description: "湿度过高告警"},
		},
	}
}

// InitOnStartup 预热模板缓存并写入内置模板。
func InitOnStartup(db *sql.DB) error {
	if db == nil {
		db = database.ConfigDB
	}
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS device_templates (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name VARCHAR(100) NOT NULL,
			type VARCHAR(50) NOT NULL,
			collect_method VARCHAR(50) DEFAULT 'simulator',
			is_built_in INTEGER DEFAULT 0,
			description TEXT,
			data TEXT,
			point_count INTEGER DEFAULT 0,
			alarm_count INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_templates_type ON device_templates(type);
		CREATE INDEX IF NOT EXISTS idx_templates_built_in ON device_templates(is_built_in);
	`); err != nil {
		return err
	}
	InitTemplateCache()
	if err := LoadTemplatesFromDB(db); err != nil {
		return err
	}
	return initBuiltInTemplates(db)
}

// insertBuiltInTemplateSQL 内置模板的插入语句：ID 由调用方给定，
// 要么是列表顺序对应的期望 ID，要么是预留区里的空位。
const insertBuiltInTemplateSQL = "INSERT INTO device_templates (id, name, type, collect_method, is_built_in, description, data, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)"

// firstFreeBuiltInTemplateID 在预留区 1..builtInTemplateIDMax 里取第一个空位。
// 只在“内置模板的期望 ID 已被占用”时用到——这种情况只会出现在尚未迁移的老库
// （导入模板落在低号区间）。此时宁可把内置模板补进预留区的空洞，
// 也不能退回 SQLite 自增：自增会落到导入区（≥1001），内置模板就跑到预留区之外了。
func firstFreeBuiltInTemplateID(db *sql.DB) (int, error) {
	rows, err := db.Query("SELECT id FROM device_templates WHERE id BETWEEN 1 AND ? ORDER BY id", builtInTemplateIDMax)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	next := 1
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return 0, err
		}
		if id > next {
			break // 出现空洞，next 即第一个空位
		}
		next = id + 1
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	if next > builtInTemplateIDMax {
		return 0, fmt.Errorf("内置模板预留区 1..%d 已占满", builtInTemplateIDMax)
	}
	return next, nil
}

// cleanupObsoleteBuiltInTemplates 删除已被新模板模型替代且没有被设备引用的旧内置模板。
func cleanupObsoleteBuiltInTemplates(db *sql.DB) {
	obsoleteNames := []string{"板载6DI2DO"}
	for _, name := range obsoleteNames {
		var templateID int
		if err := db.QueryRow("SELECT id FROM device_templates WHERE name = ? AND is_built_in = 1", name).Scan(&templateID); err != nil {
			continue
		}
		var refCount int
		if err := db.QueryRow("SELECT COUNT(*) FROM devices WHERE template_id = ?", templateID).Scan(&refCount); err != nil || refCount > 0 {
			logger.Warnf("[InitBuiltInTemplates] 旧内置模板 %s 仍被 %d 个设备引用，暂不删除", name, refCount)
			continue
		}
		if _, err := db.Exec("DELETE FROM device_templates WHERE id = ?", templateID); err != nil {
			logger.Warnf("[InitBuiltInTemplates] 删除旧内置模板 %s 失败: %v", name, err)
			continue
		}
		DeleteTemplateFromCache(templateID)
		logger.Infof("[InitBuiltInTemplates] 已删除旧内置模板: %s (ID: %d)", name, templateID)
	}
}

// initBuiltInTemplates 将项目内置模板写入数据库和缓存。
func initBuiltInTemplates(db *sql.DB) error {
	if db == nil {
		db = database.ConfigDB
	}

	cleanupObsoleteBuiltInTemplates(db)

	builtInTemplates := []BuiltInTemplateDef{
		BuildSystemCollectorTemplate(),
		BuildSimulatorTempHumTemplate(),
		BuildLocalDITemplate(),
		BuildLocalDOTemplate(),
	}

	for i, templateDef := range builtInTemplates {
		expectedID := i + 1
		var existingID int
		dbErr := db.QueryRow("SELECT id FROM device_templates WHERE name = ? AND is_built_in = 1", templateDef.Name).Scan(&existingID)

		dataJSON, err := templatebiz.Serialize(templateDef.Points, templateDef.Alarms)
		if err != nil {
			return fmt.Errorf("序列化模板 %s 失败: %v", templateDef.Name, err)
		}

		now := time.Now().Format("2006-01-02 15:04:05")
		if dbErr == nil && existingID > 0 {
			_, err = db.Exec(
				"UPDATE device_templates SET type = ?, collect_method = ?, description = ?, data = ?, updated_at = ? WHERE id = ?",
				templateDef.Type, templateDef.CollectMethod, templateDef.Description, string(dataJSON), now, existingID,
			)
			if err != nil {
				return fmt.Errorf("更新内置模板 %s 失败: %v", templateDef.Name, err)
			}
			logger.Infof("[InitBuiltInTemplates] 更新内置模板: %s (ID: %d)", templateDef.Name, existingID)
			SetTemplateToCacheWithMethod(existingID, &templatebiz.DeviceTemplate{ID: existingID, Name: templateDef.Name, Type: templateDef.Type, IsBuiltIn: true, Description: templateDef.Description, CreatedAt: now, UpdatedAt: now}, templateDef.CollectMethod, templateDef.Points, templateDef.Alarms)
			continue
		}

		result, err := db.Exec(
			insertBuiltInTemplateSQL,
			expectedID, templateDef.Name, templateDef.Type, templateDef.CollectMethod, true, templateDef.Description, string(dataJSON), now, now,
		)
		if err != nil {
			// 期望 ID 被占：只可能是未迁移的老库里导入模板落在了预留区（如 3 已被占用）。
			// 退到预留区的第一个空位，绝不退回自增——自增会把内置模板放进导入区（≥1001）。
			fallbackID, idErr := firstFreeBuiltInTemplateID(db)
			if idErr != nil {
				return fmt.Errorf("为内置模板 %s 分配预留 ID 失败: %v", templateDef.Name, idErr)
			}
			result, err = db.Exec(
				insertBuiltInTemplateSQL,
				fallbackID, templateDef.Name, templateDef.Type, templateDef.CollectMethod, true, templateDef.Description, string(dataJSON), now, now,
			)
			if err != nil {
				return fmt.Errorf("创建内置模板 %s 失败: %v", templateDef.Name, err)
			}
			logger.Warnf("[InitBuiltInTemplates] 内置模板 %s 的期望 ID %d 已被占用，改用预留区空位 %d", templateDef.Name, expectedID, fallbackID)
		}

		newID, _ := result.LastInsertId()
		logger.Infof("[InitBuiltInTemplates] 创建内置模板: %s (ID: %d)", templateDef.Name, newID)
		SetTemplateToCacheWithMethod(int(newID), &templatebiz.DeviceTemplate{ID: int(newID), Name: templateDef.Name, Type: templateDef.Type, IsBuiltIn: true, Description: templateDef.Description, CreatedAt: now, UpdatedAt: now}, templateDef.CollectMethod, templateDef.Points, templateDef.Alarms)
	}

	return nil
}

// BuildLocalDITemplate 构造开关量输入单通道模板。
func BuildLocalDITemplate() BuiltInTemplateDef {
	point := templatebiz.DevicePoint{
		PointID:      1,
		Type:         templatebiz.PointTypePOINT,
		Name:         "DI",
		NameEN:       "DI",
		OriginalType: valueobj.RawTypeBOOL,
		DataType:     valueobj.DataTypeBOOL,
		Access:       "r",
		Enabled:      1,
		Description:  "本地数字输入单通道",
		LocalIO:      &templatebiz.LocalIOPointSpec{Channel: "AUTO", Direction: "input"},
	}
	return BuiltInTemplateDef{
		Name:          "开关量输入",
		Type:          "local_di",
		CollectMethod: "gpio",
		Description:   "开关量输入单通道模板",
		Points:        []templatebiz.DevicePoint{point},
		Alarms: []alarmdomain.DeviceAlarm{{
			ID:             1,
			PointID:        1,
			Name:           "DI状态告警",
			NameEN:         "DI State Alarm",
			Level:          alarmdomain.AlarmLevelNOTICE,
			Enabled:        false,
			StartHighLimit: floatPtr(0.5),
			EndHighLimit:   floatPtr(0.5),
			Description:    "DI状态告警",
		}},
	}
}

// BuildLocalDOTemplate 构造开关量输出单通道模板。
func BuildLocalDOTemplate() BuiltInTemplateDef {
	point := templatebiz.DevicePoint{
		PointID:      1,
		Type:         templatebiz.PointTypePOINT,
		Name:         "DO",
		NameEN:       "DO",
		OriginalType: valueobj.RawTypeBOOL,
		DataType:     valueobj.DataTypeBOOL,
		Access:       "rw",
		Enabled:      1,
		Description:  "本地数字输出单通道",
		LocalIO:      &templatebiz.LocalIOPointSpec{Channel: "AUTO", Direction: "output"},
	}
	return BuiltInTemplateDef{
		Name:          "开关量输出",
		Type:          "local_do",
		CollectMethod: "gpio",
		Description:   "开关量输出单通道模板",
		Points:        []templatebiz.DevicePoint{point},
		Alarms: []alarmdomain.DeviceAlarm{{
			ID:             1,
			PointID:        1,
			Name:           "DO状态告警",
			NameEN:         "DO State Alarm",
			Level:          alarmdomain.AlarmLevelNOTICE,
			Enabled:        false,
			StartHighLimit: floatPtr(0.5),
			EndHighLimit:   floatPtr(0.5),
			Description:    "DO状态告警",
		}},
	}
}

// floatPtr 是小工具函数，避免到处写局部变量取地址。
func floatPtr(v float64) *float64 { return &v }

// getCPUCoreCount 获取 CPU 核数，失败时回退到 runtime.NumCPU。
func getCPUCoreCount() int {
	count, err := cpu.Counts(true)
	if err == nil && count > 0 {
		return count
	}
	if fallback := runtime.NumCPU(); fallback > 0 {
		return fallback
	}
	return 1
}
