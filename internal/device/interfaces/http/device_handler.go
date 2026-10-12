package http

import (
	"database/sql"
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"

	devicemodule "data-collection-system/internal/device"
	deviceapp "data-collection-system/internal/device/application"
	devicecfgdomain "data-collection-system/internal/device/domain"
	devicedomain "data-collection-system/internal/device/domain"
	"data-collection-system/internal/interfaces/httpx/locale"
	"data-collection-system/internal/interfaces/httpx/response"
	"data-collection-system/internal/shared/i18n"
	"data-collection-system/internal/shared/logger"
	"data-collection-system/internal/valueobj"

	"github.com/gin-gonic/gin"
)

// ---------------------------------------------------------------------------
// 请求/响应类型
// 这一层的 struct 主要服务于 HTTP 协议，不直接等同于 domain/application 里的模型。
// 也就是说：
// - domain struct 偏业务语义；
// - application struct 偏用例输入输出；
// - handler 里的 request/response struct 偏 JSON 绑定和接口返回格式。
// ---------------------------------------------------------------------------

// DeviceWithTemplate 是列表接口返回结构。
// 这里嵌入 devicedomain.Device，再额外补上列表页需要的 TemplateName/Enabled。
type DeviceWithTemplate struct {
	devicedomain.Device
	TemplateName string `json:"template_name"`
	Enabled      bool   `json:"enabled"`
	IOChannel    string `json:"io_channel"`
}

// DeviceDetailWithParams 设备详情+模板+参数
type DeviceDetailWithParams struct {
	devicedomain.Device
	TemplateName  string            `json:"template_name"`
	TemplateType  string            `json:"template_type"`
	CollectMethod string            `json:"collect_method"`
	Enabled       bool              `json:"enabled"`
	DeviceRef     string            `json:"device_ref"`
	Params        map[string]string `json:"params"`
}

// AddDeviceRequest 添加设备请求体
type AddDeviceRequest struct {
	Name         string                 `json:"name" binding:"required"`
	TemplateID   int                    `json:"template_id" binding:"required"`
	ProtocolType string                 `json:"protocol_type"` // 协议类型（如 RS485-RTU, MODBUS-TCP 等）
	DeviceRef    string                 `json:"device_ref"`    // device.yaml 生成的内部设备引用
	Description  string                 `json:"description"`
	Params       map[string]interface{} `json:"params"` // 设备参数（公共参数 + 协议专有参数）
}

// UpdateDeviceRequest 更新设备请求体
type UpdateDeviceRequest struct {
	Name         string                 `json:"name"`
	TemplateID   int                    `json:"template_id"`
	ProtocolType string                 `json:"protocol_type"`
	DeviceRef    string                 `json:"device_ref"`
	Description  string                 `json:"description"`
	Params       map[string]interface{} `json:"params"`
}

// PointData 测点数据返回结构
type PointData struct {
	PointID         int               `json:"point_id"`
	Name            string            `json:"name"`
	Value           interface{}       `json:"value"`
	ValueKind       string            `json:"value_kind"`
	Unit            string            `json:"unit"`
	DataType        string            `json:"data_type"`
	Access          string            `json:"access"` // 读写属性: r/w/rw
	Status          string            `json:"status"`
	Description     string            `json:"description"`
	Timestamp       string            `json:"timestamp"`
	EnabledCurrent  int               `json:"enabled_current"`
	EnabledDefault  int               `json:"enabled_default"`
	EnabledAlias    int               `json:"enabled_alias"`
	Alias           map[string]string `json:"alias"`
	ControlLevel    int               `json:"level_current"`
	ControlLevelDef int               `json:"level_default"`
	ValueMappingCN  string            `json:"value_mapping_cn"`
	ValueMappingEN  string            `json:"value_mapping_en"`
}

type WritePointRequest struct {
	Value interface{} `json:"value" binding:"required"`
}

type SavePointConfigRequest struct {
	Enabled      *bool   `json:"enabled"`
	AliasZH      *string `json:"alias_zh"`
	AliasEN      *string `json:"alias_en"`
	AliasEnabled *bool   `json:"alias_enabled"`
	ControlLevel *int    `json:"control_level"`
}

// EnableDeviceRequest 启用/禁用请求体
type EnableDeviceRequest struct {
	Enabled bool `json:"enabled"`
}

// HistoryPoint 历史数据点
type HistoryPoint struct {
	Timestamp string      `json:"timestamp"`
	Value     interface{} `json:"value"`
	ValueKind string      `json:"value_kind"`
}

var deviceValidationPattern = regexp.MustCompile(`^参数\s+(.+?)[:：]\s*(.+)$`)

// ---------------------------------------------------------------------------
// 系统设备初始化
// ---------------------------------------------------------------------------

// EnsureSystemCollectorDevice 确保系统采集器设备存在（如果不存在则自动创建）
func EnsureSystemCollectorDevice() error {
	return devicemodule.EnsureSystemCollectorDevice()
}

// ---------------------------------------------------------------------------
// 设备 CRUD API
// ---------------------------------------------------------------------------

// GetDevices 获取设备列表（JOIN模板名）
func GetDevices(c *gin.Context) {
	service := devicemodule.GetQueryService()
	items, err := service.GetDevices()
	if err != nil {
		response.API(c, 500, 500, nil, "获取设备列表失败")
		return
	}

	devices := make([]DeviceWithTemplate, 0, len(items))
	for _, item := range items {
		var device DeviceWithTemplate
		device.ID = item.ID
		device.Name = item.Name
		device.TemplateID = item.TemplateID
		device.Status = item.Status
		device.Description = item.Description
		device.CreatedAt = item.CreatedAt
		device.UpdatedAt = item.UpdatedAt
		device.Enabled = item.Enabled
		device.TemplateName = item.TemplateName
		device.IOChannel = item.IOChannel
		devices = append(devices, device)
	}

	response.API(c, 200, 200, devices, i18n.KeyCommonSuccess)
}

// AddDevice 添加设备。
// handler 层只做 4 件事：
// 1. 绑定和校验 HTTP 请求；
// 2. 调 application service；
// 3. 把已知业务错误映射成 HTTP 状态码和 message；
// 4. 组织响应体。
// 它不直接写 SQL，也不直接管理采集器生命周期。
func AddDevice(c *gin.Context) {
	var req AddDeviceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Warnf("[AddDevice] 参数绑定失败: %v", err)
		response.API(c, 400, 400, nil, i18n.KeyCommonInvalidParams)
		return
	}

	logger.Debugf("[AddDevice] 请求参数: name=%s, template_id=%d, protocol_type=%s, device_ref=%s, description=%s, params=%v",
		req.Name, req.TemplateID, req.ProtocolType, req.DeviceRef, req.Description, req.Params)

	service := devicemodule.GetConfigService()
	id, err := service.AddDevice(deviceapp.AddDeviceInput{
		Name:         req.Name,
		TemplateID:   req.TemplateID,
		ProtocolType: req.ProtocolType,
		DeviceRef:    req.DeviceRef,
		Description:  req.Description,
		Params:       req.Params,
	})
	if err != nil {
		if errors.Is(err, deviceapp.ErrTemplateNotFound) {
			response.API(c, 400, 400, nil, "模板不存在")
			return
		}
		if errors.Is(err, deviceapp.ErrParamValidation) || errors.Is(err, deviceapp.ErrInvalidProtocol) {
			response.API(c, 400, 400, nil, deviceConfigErrorMessage(c, err, i18n.KeyCommonInvalidParams))
			return
		}
		logger.Errorf("[AddDevice] 保存失败: %v", err)
		response.API(c, 500, 500, nil, "添加设备失败："+err.Error())
		return
	}
	logger.Infof("[AddDevice] 保存成功, id=%d", id)

	// 记录操作日志
	devicemodule.RecordOperation(
		c.GetInt64("userID"),
		c.GetString("username"),
		c.ClientIP(),
		"operlog.device.create",
		map[string]any{"name": req.Name},
	)

	response.API(c, 200, 200, gin.H{"id": id}, i18n.KeyCommonCreateSuccess)
}

// GetDeviceByID 获取设备详情。
// QueryService 返回的是 application.DeviceDetail，再由 handler 转成接口结构，
// 这样接口字段可以稳定演进，而不把 application 层暴露给前端。
func GetDeviceByID(c *gin.Context) {
	id := c.Param("id")
	deviceID, err := strconv.Atoi(id)
	if err != nil {
		response.API(c, 400, 400, nil, "无效的设备ID")
		return
	}

	service := devicemodule.GetQueryService()
	info, err := service.GetDeviceByID(deviceID)
	if err != nil {
		if errors.Is(err, deviceapp.ErrDeviceNotFound) {
			response.API(c, 404, 404, nil, "设备不存在")
			return
		}
		response.API(c, 500, 500, nil, "获取设备详情失败")
		return
	}
	detail := DeviceDetailWithParams{
		Device: devicedomain.Device{
			ID:          info.ID,
			Name:        info.Name,
			TemplateID:  info.TemplateID,
			Status:      info.Status,
			Description: info.Description,
			CreatedAt:   info.CreatedAt,
			UpdatedAt:   info.UpdatedAt,
		},
		TemplateName:  info.TemplateName,
		TemplateType:  info.TemplateType,
		CollectMethod: info.CollectMethod,
		Enabled:       info.Enabled,
		DeviceRef:     info.Params["device_ref"],
		Params:        info.Params,
	}

	response.API(c, 200, 200, detail, i18n.KeyCommonSuccess)
}

// UpdateDevice 更新设备基础信息和参数。
// 这里体现了 handler 常见的错误分发模式：
// 先用 errors.Is 判断是否是已知业务错误，再决定返回 400/404/500。
func UpdateDevice(c *gin.Context) {
	id := c.Param("id")
	deviceID, err := strconv.Atoi(id)
	if err != nil {
		response.API(c, 400, 400, nil, "无效的设备ID")
		return
	}
	var req UpdateDeviceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Warnf("[UpdateDevice] 参数绑定失败: %v", err)
		response.API(c, 400, 400, nil, i18n.KeyCommonInvalidParams)
		return
	}

	service := devicemodule.GetConfigService()
	err = service.UpdateDevice(deviceapp.UpdateDeviceInput{
		DeviceID:     deviceID,
		Name:         req.Name,
		TemplateID:   req.TemplateID,
		ProtocolType: req.ProtocolType,
		DeviceRef:    req.DeviceRef,
		Description:  req.Description,
		Params:       req.Params,
	})
	if err != nil {
		if errors.Is(err, deviceapp.ErrDeviceNotFound) {
			response.API(c, 404, 404, nil, "设备不存在")
			return
		}
		if errors.Is(err, deviceapp.ErrTemplateNotFound) {
			response.API(c, 400, 400, nil, "模板不存在")
			return
		}
		if errors.Is(err, deviceapp.ErrParamValidation) || errors.Is(err, deviceapp.ErrInvalidProtocol) {
			response.API(c, 400, 400, nil, deviceConfigErrorMessage(c, err, i18n.KeyCommonInvalidParams))
			return
		}
		if errors.Is(err, deviceapp.ErrCollectorReload) {
			logger.Errorf("[UpdateDevice] 重启采集器失败: %v", err)
			response.API(c, 500, 500, nil, "更新成功，但采集器重启失败，请手动重启设备")
			return
		}
		logger.Errorf("[UpdateDevice] 更新设备失败: %v", err)
		response.API(c, 500, 500, nil, "更新设备失败："+err.Error())
		return
	}

	// 记录操作日志
	devicemodule.RecordOperation(
		c.GetInt64("userID"),
		c.GetString("username"),
		c.ClientIP(),
		"operlog.device.update",
		map[string]any{"name": req.Name},
	)

	response.API(c, 200, 200, nil, i18n.KeyCommonUpdateSuccess)
}

// DeleteDevice 删除设备
func DeleteDevice(c *gin.Context) {
	id := c.Param("id")
	deviceID, err := strconv.Atoi(id)
	if err != nil {
		response.API(c, 400, 400, nil, "无效的设备ID")
		return
	}

	// 查询设备名（删除前获取，用于操作日志记录）
	var deviceName string
	if info, err := devicemodule.GetQueryService().GetDeviceByID(deviceID); err == nil {
		deviceName = info.Name
	}

	service := devicemodule.GetLifecycleService()
	if err := service.DeleteDevice(deviceID); err != nil {
		if errors.Is(err, deviceapp.ErrDeviceNotFound) {
			response.API(c, 404, 404, nil, "设备不存在")
			return
		}
		if strings.Contains(err.Error(), "stop collector timeout") {
			response.API(c, 500, 500, nil, "停止采集线程超时，请稍后重试删除")
			return
		}
		response.API(c, 500, 500, nil, "删除设备失败")
		return
	}

	// 记录操作日志
	devicemodule.RecordOperation(
		c.GetInt64("userID"),
		c.GetString("username"),
		c.ClientIP(),
		"operlog.device.delete",
		map[string]any{"name": deviceName},
	)

	response.API(c, 200, 200, nil, i18n.KeyCommonDeleteSuccess)
}

// CopyDevice 复制设备：主配置+参数用 CopyDevice 复制，测点点配置（启用/别名/控制等级）
// 逐点从原设备拷贝到新设备，保证用户定制不丢失。
func CopyDevice(c *gin.Context) {
	id := c.Param("id")
	deviceID, err := strconv.Atoi(id)
	if err != nil {
		response.API(c, 400, 400, nil, "无效的设备ID")
		return
	}

	// 读取原设备点配置（复制前先取，便于记录日志）
	srcPoints, _ := devicemodule.GetPointConfigService().GetDevicePointConfigs(deviceID)

	newID, err := devicemodule.GetConfigService().CopyDevice(deviceID)
	if err != nil {
		if errors.Is(err, deviceapp.ErrDeviceNotFound) {
			response.API(c, 404, 404, nil, "设备不存在")
			return
		}
		if errors.Is(err, deviceapp.ErrDeviceCopyConflict) {
			response.API(c, 400, 400, nil, response.Text(c, "device.copy_gpio_conflict"))
			return
		}
		logger.Errorf("[CopyDevice] 复制失败: %v", err)
		response.API(c, 500, 500, nil, "复制设备失败："+err.Error())
		return
	}

	now := time.Now().Format("2006-01-02 15:04:05")
	for pointID, cfg := range srcPoints {
		clone := *cfg
		clone.ID = 0
		clone.DeviceID = int(newID)
		clone.PointID = pointID
		if err := devicemodule.GetPointConfigService().SaveDevicePointConfig(&clone, now); err != nil {
			logger.Errorf("[CopyDevice] 复制测点配置 %d 失败: %v", pointID, err)
		}
	}

	var srcName string
	if info, err := devicemodule.GetQueryService().GetDeviceByID(deviceID); err == nil {
		srcName = info.Name
	}
	devicemodule.RecordOperation(
		c.GetInt64("userID"),
		c.GetString("username"),
		c.ClientIP(),
		"operlog.device.copy",
		map[string]any{"name": srcName, "new_id": newID},
	)

	response.API(c, 200, 200, gin.H{"id": newID}, i18n.KeyCommonCreateSuccess)
}

// ---------------------------------------------------------------------------
// 设备启停
// ---------------------------------------------------------------------------

// StartDevice 启动设备
func StartDevice(c *gin.Context) {
	id := c.Param("id")
	deviceID, err := strconv.Atoi(id)
	if err != nil {
		response.API(c, 400, 400, nil, "无效的设备ID")
		return
	}

	// 查询设备名（用于操作日志记录）
	var deviceName string
	if info, err := devicemodule.GetQueryService().GetDeviceByID(deviceID); err == nil {
		deviceName = info.Name
	}

	service := devicemodule.GetLifecycleService()
	if err := service.StartDevice(deviceID); err != nil {
		if errors.Is(err, deviceapp.ErrDeviceNotFound) {
			response.API(c, 404, 404, nil, "设备不存在")
			return
		}
		if errors.Is(err, deviceapp.ErrDeviceAlreadyRunning) {
			response.API(c, 400, 400, nil, "设备已在运行中")
			return
		}
		if errors.Is(err, deviceapp.ErrCollectorAlreadyRunning) {
			response.API(c, 400, 400, nil, "设备采集线程已在运行")
			return
		}
		response.API(c, 500, 500, nil, "启动设备失败")
		return
	}

	// 记录操作日志
	devicemodule.RecordOperation(
		c.GetInt64("userID"),
		c.GetString("username"),
		c.ClientIP(),
		"operlog.device.start",
		map[string]any{"name": deviceName},
	)

	response.API(c, 200, 200, nil, "启动成功")
}

// StopDevice 停止设备
func StopDevice(c *gin.Context) {
	id := c.Param("id")
	deviceID, err := strconv.Atoi(id)
	if err != nil {
		response.API(c, 400, 400, nil, "无效的设备ID")
		return
	}

	// 查询设备名（用于操作日志记录）
	var deviceName string
	if info, err := devicemodule.GetQueryService().GetDeviceByID(deviceID); err == nil {
		deviceName = info.Name
	}

	service := devicemodule.GetLifecycleService()
	if err := service.StopDevice(deviceID); err != nil {
		if errors.Is(err, deviceapp.ErrDeviceNotFound) {
			response.API(c, 404, 404, nil, "设备不存在")
			return
		}
		if errors.Is(err, deviceapp.ErrDeviceNotRunning) {
			response.API(c, 400, 400, nil, "设备未在运行")
			return
		}
		if strings.Contains(err.Error(), "timeout") {
			response.API(c, 500, 500, nil, "停止采集线程超时，请重试")
			return
		}
		response.API(c, 500, 500, nil, "更新设备状态失败")
		return
	}

	// 记录操作日志
	devicemodule.RecordOperation(
		c.GetInt64("userID"),
		c.GetString("username"),
		c.ClientIP(),
		"operlog.device.stop",
		map[string]any{"name": deviceName},
	)

	response.API(c, 200, 200, nil, "停止成功")
}

// SetDeviceEnabled 设置设备启用/禁用
func SetDeviceEnabled(c *gin.Context) {
	id := c.Param("id")
	deviceID, err := strconv.Atoi(id)
	if err != nil {
		response.API(c, 400, 400, nil, "无效的设备ID")
		return
	}
	var req EnableDeviceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.API(c, 400, 400, nil, i18n.KeyCommonInvalidParams)
		return
	}

	// 查询设备名（用于操作日志记录）
	var deviceName string
	if info, err := devicemodule.GetQueryService().GetDeviceByID(deviceID); err == nil {
		deviceName = info.Name
	}

	service := devicemodule.GetLifecycleService()
	if err := service.SetDeviceEnabled(deviceID, req.Enabled); err != nil {
		if errors.Is(err, deviceapp.ErrDeviceNotFound) {
			response.API(c, 404, 404, nil, "设备不存在")
			return
		}
		if strings.Contains(err.Error(), "timeout") {
			response.API(c, 500, 500, nil, "停止采集线程超时，请重试")
			return
		}
		response.API(c, 500, 500, nil, "更新设备状态失败")
		return
	}

	// 记录操作日志
	msgKey := "operlog.device.enable"
	if !req.Enabled {
		msgKey = "operlog.device.disable"
	}
	devicemodule.RecordOperation(
		c.GetInt64("userID"),
		c.GetString("username"),
		c.ClientIP(),
		msgKey,
		map[string]any{"name": deviceName},
	)

	response.API(c, 200, 200, nil, i18n.KeyCommonUpdateSuccess)
}

// ---------------------------------------------------------------------------
// 设备数据查询
// ---------------------------------------------------------------------------

// GetDevicePoints 获取设备测点实时值
// 支持通过 type 参数筛选：
// - type=read: 返回测点（Access=r 或 rw）
// - type=control: 返回控制点（Access=w 或 rw）
// - 不传参数: 返回全部测点
func GetDevicePoints(c *gin.Context) {
	id := c.Param("id")
	pointType := c.Query("type") // read/control/all
	deviceID, err := strconv.Atoi(id)
	if err != nil {
		response.API(c, 400, 400, nil, "无效的设备ID")
		return
	}

	snapshot, err := devicemodule.GetDevicePointsSnapshot(deviceID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			response.API(c, 404, 404, nil, "设备不存在")
			return
		}
		response.API(c, 500, 500, nil, "查询设备测点失败")
		return
	}

	pointCfgSvc := devicemodule.GetPointConfigService()
	validPointIDs := make(map[int]struct{}, len(snapshot.Points))
	for _, p := range snapshot.Points {
		validPointIDs[p.PointID] = struct{}{}
	}
	_ = pointCfgSvc.ReconcileDevicePointConfigs(deviceID, validPointIDs)
	pointCfgs, _ := pointCfgSvc.GetDevicePointConfigs(deviceID)

	// 构建结果
	var result []PointData

	for _, point := range snapshot.Points {
		// 根据 type 参数筛选
		switch pointType {
		case "read":
			if point.Access != "r" && point.Access != "rw" {
				continue
			}
		case "control":
			if point.Access != "w" && point.Access != "rw" {
				continue
			}
		}

		pointData := PointData{
			PointID:         point.PointID,
			Name:            point.Name,
			Unit:            point.Unit,
			DataType:        point.DataType,
			Access:          point.Access,
			Status:          response.Text(c, "已启用"),
			Description:     point.Description,
			EnabledDefault:  boolToInt(point.Enabled),
			EnabledCurrent:  boolToInt(point.Enabled),
			EnabledAlias:    0,
			Alias:           map[string]string{"zh-CN": "", "en-US": ""},
			ControlLevel:    1,
			ControlLevelDef: 1,
			ValueKind:       defaultValueKindByDataType(point.DataType),
			ValueMappingCN:  point.ValueMappingCN,
			ValueMappingEN:  point.ValueMappingEN,
		}

		if cfg, ok := pointCfgs[point.PointID]; ok {
			pointData.EnabledCurrent = boolToInt(cfg.Enabled)
			pointData.EnabledAlias = boolToInt(cfg.AliasEnabled)
			if cfg.AliasZH != "" || cfg.AliasEN != "" {
				pointData.Alias = map[string]string{
					"zh-CN": cfg.AliasZH,
					"en-US": cfg.AliasEN,
				}
				if cfg.AliasEnabled {
					pointData.Name = localizedAliasName(c, cfg.AliasZH, cfg.AliasEN, pointData.Name)
				}
			}
			if cfg.ControlLevel > 0 {
				pointData.ControlLevel = cfg.ControlLevel
			}
		}

		if pointData.EnabledCurrent == 0 {
			pointData.Status = response.Text(c, "已禁用")
		}

		if snapshot.Running {
			// 设备运行中，从内存获取实时值
			if v, ok := snapshot.Values[point.PointID]; ok && v.Found {
				pointData.Value = v.Value.AsAny()
				pointData.ValueKind = string(v.Value.Kind)
				pointData.Timestamp = v.Timestamp.Format("2006-01-02 15:04:05")
			} else {
				pointData.Value = defaultPointValueByDataType(point.DataType)
				pointData.Timestamp = "-"
			}
		} else {
			// 设备未启动，返回默认值
			pointData.Value = defaultPointValueByDataType(point.DataType)
			pointData.Timestamp = "-"
		}

		result = append(result, pointData)
	}

	c.JSON(200, gin.H{
		"code":    200,
		"data":    result,
		"message": response.Text(c, i18n.KeyCommonSuccess),
		"running": snapshot.Running,
	})
}

// WriteDevicePoint 写入单个设备测点值（仅支持写权限测点）
func WriteDevicePoint(c *gin.Context) {
	deviceID, pointID, ok := parseDeviceAndPointID(c)
	if !ok {
		return
	}

	var req WritePointRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.API(c, 400, 400, nil, i18n.KeyCommonInvalidParams)
		return
	}

	point, err := devicemodule.GetTemplatePointByDevice(deviceID, pointID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			response.API(c, 404, 404, nil, "设备或测点不存在")
			return
		}
		response.API(c, 500, 500, nil, "查询测点失败")
		return
	}

	if !strings.Contains(point.Access, "w") {
		response.API(c, 403, 403, nil, "该测点不支持写入")
		return
	}

	dataTypeUpper := strings.ToUpper(strings.TrimSpace(point.DataType))
	var cv valueobj.CollectedValue
	switch dataTypeUpper {
	case valueobj.DataTypeBOOL:
		var b bool
		switch v := req.Value.(type) {
		case bool:
			b = v
		case string:
			b = v == "1" || strings.EqualFold(v, "true")
		case float64:
			b = v != 0
		default:
			response.API(c, 400, 400, nil, "无效的布尔值")
			return
		}
		cv = valueobj.NewBoolValue(b)
	case valueobj.DataTypeSTRING:
		s, ok := req.Value.(string)
		if !ok {
			response.API(c, 400, 400, nil, "无效的字符串值")
			return
		}
		cv = valueobj.NewTextValue(s)
	default:
		var f float64
		switch v := req.Value.(type) {
		case float64:
			f = v
		case int:
			f = float64(v)
		case string:
			parsed, err := strconv.ParseFloat(v, 64)
			if err != nil {
				response.API(c, 400, 400, nil, "无效的数值")
				return
			}
			f = parsed
		default:
			response.API(c, 400, 400, nil, "无效的数值")
			return
		}
		cv = valueobj.NewNumberValue(f)
	}

	if err := devicemodule.WriteDevicePoint(deviceID, pointID, cv); err != nil {
		logger.Errorf("[WriteDevicePoint] device=%d point=%d write failed: %v", deviceID, pointID, err)
		response.API(c, 500, 500, nil, "写入失败")
		return
	}

	response.API(c, 200, 200, nil, i18n.KeyCommonSaveSuccess)
}

func defaultPointValueByDataType(dataType string) interface{} {
	switch strings.ToUpper(strings.TrimSpace(dataType)) {
	case valueobj.DataTypeSTRING:
		return ""
	case valueobj.DataTypeBOOL:
		return false
	default:
		return 0
	}
}

func defaultValueKindByDataType(dataType string) string {
	switch strings.ToUpper(strings.TrimSpace(dataType)) {
	case valueobj.DataTypeSTRING:
		return "text"
	case valueobj.DataTypeBOOL:
		return "bool"
	default:
		return "number"
	}
}

// SaveDevicePointConfig 保存设备测点/控制点配置（启用、别名、权限级别）
func SaveDevicePointConfig(c *gin.Context) {
	deviceID, pointID, ok := parseDeviceAndPointID(c)
	if !ok {
		return
	}

	var req SavePointConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.API(c, 400, 400, nil, i18n.KeyCommonInvalidParams)
		return
	}
	if req.Enabled == nil && req.AliasZH == nil && req.AliasEN == nil && req.AliasEnabled == nil && req.ControlLevel == nil {
		response.API(c, 400, 400, nil, "未提供任何可更新字段")
		return
	}
	if req.ControlLevel != nil && (*req.ControlLevel < 1 || *req.ControlLevel > 3) {
		response.API(c, 400, 400, nil, "control_level 仅支持 1~3")
		return
	}

	templatePoint, err := loadTemplatePointByDevice(deviceID, pointID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			response.API(c, 404, 404, nil, "设备或测点不存在")
			return
		}
		response.API(c, 500, 500, nil, "查询设备配置失败")
		return
	}

	pointCfgSvc := devicemodule.GetPointConfigService()
	cfgs, _ := pointCfgSvc.GetDevicePointConfigs(deviceID)

	current := buildCurrentPointConfig(deviceID, pointID, templatePoint, cfgs[pointID])
	applyPointConfigRequest(&current, req)

	nowStr := time.Now().Format("2006-01-02 15:04:05")
	if isDefaultPointConfig(current, templatePoint) {
		if err := pointCfgSvc.DeleteDevicePointConfig(deviceID, pointID); err != nil {
			response.API(c, 500, 500, nil, "重置配置失败")
			return
		}
		current = buildCurrentPointConfig(deviceID, pointID, templatePoint, nil)
	} else {
		if err := pointCfgSvc.SaveDevicePointConfig(&current, nowStr); err != nil {
			response.API(c, 500, 500, nil, "保存配置失败")
			return
		}
	}

	devicemodule.UpdatePointEnabled(deviceID, pointID, current.Enabled)

	response.API(c, 200, 200, gin.H{
		"device_id":     current.DeviceID,
		"point_id":      current.PointID,
		"enabled":       current.Enabled,
		"alias_enabled": current.AliasEnabled,
		"alias_zh":      current.AliasZH,
		"alias_en":      current.AliasEN,
		"control_level": current.ControlLevel,
	}, i18n.KeyCommonSaveSuccess)
}

// DeleteDevicePointConfig 删除设备测点配置并恢复模板默认值
func DeleteDevicePointConfig(c *gin.Context) {
	deviceID, pointID, ok := parseDeviceAndPointID(c)
	if !ok {
		return
	}

	templatePoint, err := loadTemplatePointByDevice(deviceID, pointID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			response.API(c, 404, 404, nil, "设备或测点不存在")
			return
		}
		response.API(c, 500, 500, nil, "查询设备配置失败")
		return
	}

	pointCfgSvc := devicemodule.GetPointConfigService()
	if err := pointCfgSvc.DeleteDevicePointConfig(deviceID, pointID); err != nil {
		response.API(c, 500, 500, nil, "删除配置失败")
		return
	}

	defaultEnabled := templatePoint.Enabled
	devicemodule.UpdatePointEnabled(deviceID, pointID, defaultEnabled)

	response.API(c, 200, 200, nil, i18n.KeyCommonDeleteSuccess)
}

func parseDeviceAndPointID(c *gin.Context) (int, int, bool) {
	deviceID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.API(c, 400, 400, nil, "无效的设备ID")
		return 0, 0, false
	}
	pointID, err := strconv.Atoi(c.Param("point_id"))
	if err != nil {
		response.API(c, 400, 400, nil, "无效的测点ID")
		return 0, 0, false
	}
	return deviceID, pointID, true
}

func loadTemplatePointByDevice(deviceID, pointID int) (devicemodule.PointTemplate, error) {
	return devicemodule.GetTemplatePointByDevice(deviceID, pointID)
}

func buildCurrentPointConfig(deviceID, pointID int, templatePoint devicemodule.PointTemplate, existing *devicecfgdomain.DevicePointConfig) devicecfgdomain.DevicePointConfig {
	cfg := devicecfgdomain.DevicePointConfig{
		DeviceID:     deviceID,
		PointID:      pointID,
		Enabled:      templatePoint.Enabled,
		AliasEnabled: false,
		AliasZH:      "",
		AliasEN:      "",
		ControlLevel: 1,
	}
	if existing == nil {
		return cfg
	}
	cfg.Enabled = existing.Enabled
	cfg.AliasEnabled = existing.AliasEnabled
	cfg.AliasZH = existing.AliasZH
	cfg.AliasEN = existing.AliasEN
	if existing.ControlLevel > 0 {
		cfg.ControlLevel = existing.ControlLevel
	}
	return cfg
}

func applyPointConfigRequest(cfg *devicecfgdomain.DevicePointConfig, req SavePointConfigRequest) {
	if req.Enabled != nil {
		cfg.Enabled = *req.Enabled
	}
	if req.AliasZH != nil {
		cfg.AliasZH = strings.TrimSpace(*req.AliasZH)
	}
	if req.AliasEN != nil {
		cfg.AliasEN = strings.TrimSpace(*req.AliasEN)
	}
	if req.AliasEnabled != nil {
		cfg.AliasEnabled = *req.AliasEnabled
	}
	if req.ControlLevel != nil {
		cfg.ControlLevel = *req.ControlLevel
	}
}

func isDefaultPointConfig(cfg devicecfgdomain.DevicePointConfig, templatePoint devicemodule.PointTemplate) bool {
	return cfg.Enabled == templatePoint.Enabled &&
		cfg.ControlLevel == 1 &&
		!cfg.AliasEnabled &&
		cfg.AliasZH == "" &&
		cfg.AliasEN == ""
}

// GetDeviceHistory 获取设备历史数据
func GetDeviceHistory(c *gin.Context) {
	id := c.Param("id")
	deviceID, err := strconv.Atoi(id)
	if err != nil {
		response.API(c, 400, 400, nil, "无效的设备ID")
		return
	}
	pointID := c.Query("point_id")
	limit := c.DefaultQuery("limit", "100")
	// 可选时间范围，格式 "2006-01-02 15:04:05"（与库内 collected_at 一致）。
	start := c.Query("start")
	end := c.Query("end")
	service := devicemodule.GetQueryService()
	history, err := service.GetDeviceHistory(deviceID, pointID, limit, start, end)
	if err != nil {
		response.API(c, 500, 500, nil, "获取历史数据失败")
		return
	}
	response.API(c, 200, 200, history, i18n.KeyCommonSuccess)
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

// GetDeviceParams 获取设备参数
func GetDeviceParams(c *gin.Context) {
	id := c.Param("id")
	deviceID, err := strconv.Atoi(id)
	if err != nil {
		response.API(c, 400, 400, nil, "无效的设备ID")
		return
	}
	service := devicemodule.GetQueryService()
	entries, err := service.GetDeviceParams(deviceID)
	if err != nil {
		response.API(c, 500, 500, nil, "获取设备参数失败")
		return
	}

	params := make([]map[string]string, 0, len(entries))
	for _, e := range entries {
		params = append(params, map[string]string{
			"name":  e.Name,
			"value": e.Value,
		})
	}

	response.API(c, 200, 200, params, i18n.KeyCommonSuccess)
}

// UpdateDeviceParams 更新设备参数
func UpdateDeviceParams(c *gin.Context) {
	id := c.Param("id")
	deviceID, err := strconv.Atoi(id)
	if err != nil {
		response.API(c, 400, 400, nil, "无效的设备ID")
		return
	}
	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.API(c, 400, 400, nil, i18n.KeyCommonInvalidParams)
		return
	}

	service := devicemodule.GetConfigService()
	err = service.UpdateDeviceParams(deviceapp.UpdateDeviceParamsInput{
		DeviceID: deviceID,
		Params:   req,
	})
	if err != nil {
		if errors.Is(err, deviceapp.ErrDeviceNotFound) {
			response.API(c, 404, 404, nil, "设备不存在")
			return
		}
		if errors.Is(err, deviceapp.ErrCollectorReload) {
			logger.Errorf("[UpdateDeviceParams] 重启采集器失败: %v", err)
			response.API(c, 500, 500, nil, "参数已保存，但采集器重启失败，请手动重启设备")
			return
		}
		logger.Errorf("[UpdateDeviceParams] 更新参数失败: %v", err)
		response.API(c, 500, 500, nil, "更新设备参数失败")
		return
	}

	response.API(c, 200, 200, nil, i18n.KeyCommonUpdateSuccess)
}

func localizedAliasName(c *gin.Context, zhName, enName, fallback string) string {
	if locale.IsEnglish(c) {
		if strings.TrimSpace(enName) != "" {
			return enName
		}
		if strings.TrimSpace(zhName) != "" {
			return zhName
		}
		return fallback
	}
	if strings.TrimSpace(zhName) != "" {
		return zhName
	}
	if strings.TrimSpace(enName) != "" {
		return enName
	}
	return fallback
}

func deviceConfigErrorMessage(c *gin.Context, err error, fallback string) string {
	if err == nil {
		return fallback
	}
	switch {
	case errors.Is(err, deviceapp.ErrParamValidation):
		detail := trimWrappedError(err.Error(), deviceapp.ErrParamValidation.Error())
		if localized := localizeValidationDetails(c, detail); localized != "" {
			return localized
		}
	case errors.Is(err, deviceapp.ErrInvalidProtocol):
		detail := trimWrappedError(err.Error(), deviceapp.ErrInvalidProtocol.Error())
		if strings.TrimSpace(detail) == "" {
			return response.Text(c, "device.invalid_protocol")
		}
		msg := response.Text(c, "device.invalid_protocol")
		if locale.IsEnglish(c) {
			return msg + ": " + detail
		}
		return msg + "：" + detail
	}
	if raw := strings.TrimSpace(err.Error()); raw != "" {
		return response.Text(c, raw)
	}
	return response.Text(c, fallback)
}

func localizeValidationDetails(c *gin.Context, raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	raw = strings.ReplaceAll(raw, "；", ";")
	parts := strings.Split(raw, ";")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if localized := localizeValidationSegment(c, part); localized != "" {
			result = append(result, localized)
		}
	}
	if len(result) == 0 {
		return ""
	}
	separator := "；"
	if locale.IsEnglish(c) {
		separator = "; "
	}
	return strings.Join(result, separator)
}

func localizeValidationSegment(c *gin.Context, raw string) string {
	segment := strings.TrimSpace(raw)
	if segment == "" {
		return ""
	}
	if matches := deviceValidationPattern.FindStringSubmatch(segment); len(matches) == 3 {
		detail := response.Text(c, matches[2])
		if locale.IsEnglish(c) {
			return "Parameter " + strings.TrimSpace(matches[1]) + ": " + detail
		}
		return "参数 " + strings.TrimSpace(matches[1]) + "：" + detail
	}
	return response.Text(c, segment)
}

func trimWrappedError(raw, prefix string) string {
	trimmed := strings.TrimSpace(raw)
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return trimmed
	}
	for _, sep := range []string{":", "："} {
		fullPrefix := prefix + sep
		if strings.HasPrefix(trimmed, fullPrefix) {
			return strings.TrimSpace(trimmed[len(fullPrefix):])
		}
	}
	return trimmed
}
