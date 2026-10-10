package application

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	devicedomain "data-collection-system/internal/device/domain"
	"data-collection-system/internal/protocol"
)

var (
	ErrDeviceNotFound     = errors.New("device not found")
	ErrTemplateNotFound   = errors.New("template not found")
	ErrParamValidation    = errors.New("param validation failed")
	ErrCollectorReload    = errors.New("collector reload failed")
	ErrInvalidDeviceID    = errors.New("invalid device id")
	ErrInvalidProtocol    = errors.New("invalid protocol type")
	ErrDeviceCopyConflict = errors.New("device copy conflict")
)

type DeviceConfigRepository interface {
	// 这里用接口隔离 SQL 实现，方便测试和替换存储层。
	// interface 在 Go 里通常放在“使用方”这一侧定义，而不是放在实现方旁边。
	// 原因是 service 真正关心的是“它需要哪些方法”，而不是底层仓储还会额外做什么。
	GetTemplateMeta(templateID int) (devicedomain.TemplateMeta, error)
	GetDeviceSnapshot(deviceID int) (devicedomain.DeviceConfigSnapshot, error)
	GetDeviceFullSnapshot(deviceID int) (devicedomain.DeviceConfigFull, error)
	InsertDeviceWithParams(name string, templateID int, description string, params map[string]string, now time.Time) (int64, error)
	SaveDeviceConfig(deviceID int, name string, templateID int, description string, params map[string]string, now time.Time) error
	SaveDeviceParams(deviceID int, params map[string]string, now time.Time) error
}

type DeviceReloader interface {
	// 配置变更后，如果设备正在运行，就由外层负责重启采集器。
	ReloadIfRunning(deviceID int, status string, templateID int) error
}

// ResolvedDeviceEntry 是 device 用例层需要的设备目录解析结果。
// 这里只保留 selector、类型和最终固定参数，不把 system 模块模型泄露到 application。
type ResolvedDeviceEntry struct {
	Ref      string
	Selector string
	Kind     string
	Params   map[string]string
	Defaults map[string]string
	Channels map[string]string
}

// DeviceCatalogResolver 由模块装配层注入，application 只依赖该能力。
type DeviceCatalogResolver interface {
	ResolveDeviceRef(ref string) (ResolvedDeviceEntry, error)
}

type ConfigService struct {
	repo     DeviceConfigRepository
	reloader DeviceReloader
	catalog  DeviceCatalogResolver
}

func NewConfigService(repo DeviceConfigRepository, reloader DeviceReloader, catalog DeviceCatalogResolver) *ConfigService {
	return &ConfigService{repo: repo, reloader: reloader, catalog: catalog}
}

type AddDeviceInput struct {
	// 这里的 Params 用 map[string]interface{} 是为了直接接住 JSON 请求体里
	// 可能出现的 string/number/bool 等混合类型，后面再统一归一为 string map。
	Name         string
	TemplateID   int
	ProtocolType string
	DeviceRef    string
	Description  string
	Params       map[string]interface{}
}

type UpdateDeviceInput struct {
	DeviceID     int
	Name         string
	TemplateID   int
	ProtocolType string
	DeviceRef    string
	Description  string
	Params       map[string]interface{}
}

type UpdateDeviceParamsInput struct {
	DeviceID int
	Params   map[string]interface{}
}

// AddDevice 新建设备，并按模板规则整理设备参数。
// 主流程：
// 1. 查模板元信息，确定这是哪类模板；
// 2. 按模板规则整理并校验参数；
// 3. 把整理后的配置交给 repository 落库。
func (s *ConfigService) AddDevice(input AddDeviceInput) (int64, error) {
	templateMeta, err := s.repo.GetTemplateMeta(input.TemplateID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, ErrTemplateNotFound
		}
		return 0, err
	}

	deviceParams, err := prepareDeviceParamsForTemplate(templateMeta, protocol.ProtocolType(input.ProtocolType), input.DeviceRef, input.Params, s.catalog, true)
	if err != nil {
		if errors.Is(err, ErrInvalidProtocol) || errors.Is(err, ErrParamValidation) {
			return 0, err
		}
		return 0, fmt.Errorf("%w: %v", ErrParamValidation, err)
	}

	return s.repo.InsertDeviceWithParams(input.Name, input.TemplateID, input.Description, deviceParams, time.Now())
}

// UpdateDevice 更新设备主配置。
// 这里会先校验模板和参数，再决定是否触发采集器重载。
// 注意这里先保存，再按“旧状态”决定是否重载运行中的采集器。
// 这样配置态和运行态的职责是分开的：repository 只负责存储，reloader 只负责把新配置生效。
func (s *ConfigService) UpdateDevice(input UpdateDeviceInput) error {
	// 设备 ID 0 是合法的：全局指标虚拟设备（metric.MetricDeviceID = 0）就用 0，
	// 这里只能排除负数，否则该行所有操作都会被误判为 “invalid device id”。
	if input.DeviceID < 0 {
		return ErrInvalidDeviceID
	}

	snapshot, err := s.repo.GetDeviceSnapshot(input.DeviceID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrDeviceNotFound
		}
		return err
	}

	templateID := input.TemplateID
	if templateID == 0 {
		templateID = snapshot.TemplateID
	}

	templateMeta, err := s.repo.GetTemplateMeta(templateID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrTemplateNotFound
		}
		return err
	}

	deviceParams, err := prepareDeviceParamsForTemplate(templateMeta, protocol.ProtocolType(input.ProtocolType), input.DeviceRef, input.Params, s.catalog, false)
	if err != nil {
		if errors.Is(err, ErrInvalidProtocol) || errors.Is(err, ErrParamValidation) {
			return err
		}
		return fmt.Errorf("%w: %v", ErrParamValidation, err)
	}

	if err := s.repo.SaveDeviceConfig(input.DeviceID, input.Name, templateID, input.Description, deviceParams, time.Now()); err != nil {
		return err
	}

	if err := s.reloader.ReloadIfRunning(input.DeviceID, snapshot.Status, templateID); err != nil {
		return fmt.Errorf("%w: %v", ErrCollectorReload, err)
	}

	return nil
}

// CopyDevice 复制设备主配置：新设备以“原名称_copy”命名，沿用模板、描述与参数。
// 测点用户配置的复制由 handler 层组合 PointConfigService 完成，不在此处耦合。
func (s *ConfigService) CopyDevice(deviceID int) (int64, error) {
	// 设备 ID 0 是合法的：全局指标虚拟设备（metric.MetricDeviceID = 0）就用 0，
	// 这里只排除负数，否则该行所有操作都会被误判为 “invalid device id”。
	if deviceID < 0 {
		return 0, ErrInvalidDeviceID
	}

	full, err := s.repo.GetDeviceFullSnapshot(deviceID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, ErrDeviceNotFound
		}
		return 0, err
	}

	// local_io 的通道由同一块控制器独占，复制会直接造成通道冲突。
	if full.Params["collect_method"] == "gpio" {
		return 0, ErrDeviceCopyConflict
	}

	return s.repo.InsertDeviceWithParams(full.Name+"_copy", full.TemplateID, full.Description, full.Params, time.Now())
}

// UpdateDeviceParams 只更新设备参数，不改设备主配置。
// 它是 UpdateDevice 的一个轻量版本，适合只改协议参数的场景。
func (s *ConfigService) UpdateDeviceParams(input UpdateDeviceParamsInput) error {
	// 设备 ID 0 是合法的：全局指标虚拟设备（metric.MetricDeviceID = 0）就用 0，
	// 这里只能排除负数，否则该行所有操作都会被误判为 “invalid device id”。
	if input.DeviceID < 0 {
		return ErrInvalidDeviceID
	}

	snapshot, err := s.repo.GetDeviceSnapshot(input.DeviceID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrDeviceNotFound
		}
		return err
	}

	params := normalizeDeviceParams(input.Params)
	if err := s.repo.SaveDeviceParams(input.DeviceID, params, time.Now()); err != nil {
		return err
	}

	if err := s.reloader.ReloadIfRunning(input.DeviceID, snapshot.Status, snapshot.TemplateID); err != nil {
		return fmt.Errorf("%w: %v", ErrCollectorReload, err)
	}
	return nil
}

// isProtocolManagedTemplate 判断模板是否由协议层托管参数。
func isProtocolManagedTemplate(templateType string) bool {
	switch strings.ToLower(strings.TrimSpace(templateType)) {
	case "modbus", "snmp", "local_io", "local_di", "local_do":
		return true
	default:
		return false
	}
}

// resolveProtocolTypeForTemplate 把请求的协议类型归一到模板允许的值。
func resolveProtocolTypeForTemplate(templateType string, requested protocol.ProtocolType) (protocol.ProtocolType, error) {
	requested = normalizeRequestedProtocolType(requested)
	switch strings.ToLower(strings.TrimSpace(templateType)) {
	case "modbus":
		protocolType := requested
		if protocolType == "" {
			protocolType = protocol.ProtocolRS485RTU
		}
		for _, info := range protocol.GetModbusProtocolTypes() {
			if info.Type == protocolType {
				return protocolType, nil
			}
		}
		return "", fmt.Errorf("%w: %s", ErrInvalidProtocol, requested)
	case "snmp":
		return protocol.ProtocolSNMP, nil
	case "local_io", "local_di", "local_do":
		return protocol.ProtocolType(strings.ToLower(strings.TrimSpace(templateType))), nil
	default:
		return "", nil
	}
}

// prepareDeviceParamsForTemplate 统一整理设备参数。
// deviceRef 非空时必须由后端目录解析，前端提交的物理端点和固定参数不可覆盖。
func prepareDeviceParamsForTemplate(templateMeta devicedomain.TemplateMeta, requested protocol.ProtocolType, deviceRef string, rawParams map[string]interface{}, catalog DeviceCatalogResolver, requireRef bool) (map[string]string, error) {
	templateType := templateMeta.Type
	collectMethod := templateMeta.CollectMethod
	if rawParams == nil {
		rawParams = make(map[string]interface{})
	}
	deviceParams := normalizeDeviceParams(rawParams)

	if isProtocolManagedTemplate(templateType) {
		protocolType, err := resolveProtocolTypeForTemplate(templateType, requested)
		if err != nil {
			return nil, err
		}

		preservedParams := deviceParams
		deviceParams = protocol.ApplyParamDefaults(protocolType, preservedParams)
		for key, value := range preservedParams {
			if _, exists := deviceParams[key]; !exists {
				deviceParams[key] = value
			}
		}
		deviceParams["protocol_type"] = string(protocolType)

		if strings.TrimSpace(deviceRef) != "" {
			if catalog == nil {
				return nil, fmt.Errorf("%w: device catalog is not configured", ErrParamValidation)
			}
			entry, err := catalog.ResolveDeviceRef(strings.TrimSpace(deviceRef))
			if err != nil {
				return nil, fmt.Errorf("%w: 设备引用不存在或已失效", ErrParamValidation)
			}
			if err := validateCatalogSelector(protocolType, templateType, entry.Selector); err != nil {
				return nil, err
			}
			deviceParams, err = mergeCatalogParams(deviceParams, entry.Params, entry.Defaults)
			if err != nil {
				return nil, err
			}
			if isLocalIOTemplateType(templateType) {
				switch strings.ToLower(strings.TrimSpace(templateType)) {
				case "local_di":
					deviceParams["io_direction"] = "di"
				case "local_do":
					deviceParams["io_direction"] = "do"
				}
				if err := validateLocalIORequirements(deviceParams, templateMeta.LocalIOChannels, entry.Channels); err != nil {
					return nil, err
				}
			}
		} else if requiresDeviceRef(templateType, protocolType) && (requireRef || isLocalIOTemplateType(templateType)) {
			return nil, fmt.Errorf("%w: 设备引用不能为空", ErrParamValidation)
		}

		validationErrors := protocol.ValidateDeviceParams(protocolType, deviceParams)
		if len(validationErrors) > 0 {
			var errMsgs []string
			for _, e := range validationErrors {
				errMsgs = append(errMsgs, e.Message)
			}
			return nil, fmt.Errorf("%w: %s", ErrParamValidation, strings.Join(errMsgs, "; "))
		}
		return deviceParams, nil
	}

	// 非协议模板（系统、模拟等）：直接保存模板元信息
	deviceParams["collect_method"] = collectMethod
	deviceParams["template_type"] = templateType
	return deviceParams, nil
}

func isLocalIOTemplateType(templateType string) bool {
	switch strings.ToLower(strings.TrimSpace(templateType)) {
	case "local_io", "local_di", "local_do":
		return true
	default:
		return false
	}
}

func validateLocalIORequirements(deviceParams map[string]string, requirements []devicedomain.TemplateIOPointMeta, channels map[string]string) error {
	direction := strings.ToLower(strings.TrimSpace(deviceParams["io_direction"]))
	if direction != "di" && direction != "do" {
		return fmt.Errorf("%w: DIDO类型无效", ErrParamValidation)
	}
	channel := strings.ToUpper(strings.TrimSpace(deviceParams["io_channel"]))
	channelDirection, ok := channels[strings.ToLower(channel)]
	if !ok {
		return fmt.Errorf("%w: DIDO接口不存在", ErrParamValidation)
	}
	if (direction == "di" && channelDirection != "input") || (direction == "do" && channelDirection != "output") {
		return fmt.Errorf("%w: DIDO接口方向不匹配", ErrParamValidation)
	}
	deviceParams["io_direction"] = direction
	deviceParams["io_channel"] = channel

	alias0ZH, err := normalizeLocalIOAlias(deviceParams["io_alias_0_zh"])
	if err != nil {
		return err
	}
	alias1ZH, err := normalizeLocalIOAlias(deviceParams["io_alias_1_zh"])
	if err != nil {
		return err
	}
	alias0EN, err := normalizeLocalIOAlias(deviceParams["io_alias_0_en"])
	if err != nil {
		return err
	}
	alias1EN, err := normalizeLocalIOAlias(deviceParams["io_alias_1_en"])
	if err != nil {
		return err
	}
	deviceParams["io_alias_0_zh"] = alias0ZH
	deviceParams["io_alias_1_zh"] = alias1ZH
	deviceParams["io_alias_0_en"] = alias0EN
	deviceParams["io_alias_1_en"] = alias1EN
	deviceParams["io_value_mapping_zh"] = buildLocalIOValueMapping(alias0ZH, alias1ZH)
	deviceParams["io_value_mapping_en"] = buildLocalIOValueMapping(alias0EN, alias1EN)

	for _, requirement := range requirements {
		requiredChannel := strings.ToUpper(strings.TrimSpace(requirement.Channel))
		if requiredChannel == "AUTO" {
			continue
		}
		requiredDirection, ok := channels[strings.ToLower(requiredChannel)]
		if !ok {
			return fmt.Errorf("%w: 模板要求的本地 IO 通道不存在", ErrParamValidation)
		}
		access := strings.ToLower(strings.TrimSpace(requirement.Access))
		if requiredDirection == "input" && access != "r" {
			return fmt.Errorf("%w: 本地 IO 通道方向不匹配", ErrParamValidation)
		}
		if requiredDirection == "output" && access != "w" && access != "rw" {
			return fmt.Errorf("%w: 本地 IO 通道方向不匹配", ErrParamValidation)
		}
	}
	return nil
}

func normalizeLocalIOAlias(value string) (string, error) {
	value = strings.TrimSpace(value)
	if strings.ContainsAny(value, ":,") {
		return "", fmt.Errorf("%w: 别名不能包含冒号或逗号", ErrParamValidation)
	}
	return value, nil
}

func buildLocalIOValueMapping(alias0, alias1 string) string {
	parts := make([]string, 0, 2)
	if alias0 != "" {
		parts = append(parts, "0:"+alias0)
	}
	if alias1 != "" {
		parts = append(parts, "1:"+alias1)
	}
	return strings.Join(parts, ",")
}

func normalizeRequestedProtocolType(value protocol.ProtocolType) protocol.ProtocolType {
	switch strings.ToLower(strings.TrimSpace(string(value))) {
	case "rs485_rtu", "rs485-rtu":
		return protocol.ProtocolRS485RTU
	case "rs232_rtu", "rs232-rtu":
		return protocol.ProtocolRS232RTU
	default:
		return protocol.ProtocolType(strings.ToUpper(strings.TrimSpace(string(value))))
	}
}

func requiresDeviceRef(templateType string, protocolType protocol.ProtocolType) bool {
	if isLocalIOTemplateType(templateType) {
		return true
	}
	switch protocolType {
	case protocol.ProtocolRS485RTU, protocol.ProtocolRS232RTU:
		return true
	default:
		return false
	}
}

func validateCatalogSelector(protocolType protocol.ProtocolType, templateType, selector string) error {
	selector = strings.ToLower(strings.TrimSpace(selector))
	if isLocalIOTemplateType(templateType) {
		if selector == "local_io" {
			return nil
		}
		return fmt.Errorf("%w: 设备引用与协议不匹配", ErrParamValidation)
	}
	want := strings.ToLower(string(protocolType))
	want = strings.ReplaceAll(want, "-", "_")
	if selector != want {
		return fmt.Errorf("%w: 设备引用与协议不匹配", ErrParamValidation)
	}
	return nil
}

func mergeCatalogParams(userParams, catalogParams, catalogDefaults map[string]string) (map[string]string, error) {
	if len(catalogParams) == 0 {
		return userParams, nil
	}
	merged := make(map[string]string, len(userParams)+len(catalogParams))
	for key, value := range userParams {
		merged[key] = value
	}
	for key, value := range catalogParams {
		if _, isDefault := catalogDefaults[key]; isDefault {
			if current, exists := merged[key]; !exists || strings.TrimSpace(current) == "" {
				merged[key] = value
			}
			continue
		}
		if current, exists := merged[key]; exists && strings.TrimSpace(current) != "" && current != value {
			return nil, fmt.Errorf("%w: 固定参数不允许修改", ErrParamValidation)
		}
		merged[key] = value
	}
	return merged, nil
}

// normalizeDeviceParams 把任意 JSON 风格参数转成字符串 map。
// type switch 是 Go 里常见的动态类型分发语法。
// 可把它类比成：
//
//	先判断 interface{} 当前实际装的是什么类型，再进入对应分支处理。
func normalizeDeviceParams(params map[string]interface{}) map[string]string {
	normalized := make(map[string]string, len(params))
	for name, value := range params {
		switch v := value.(type) {
		case nil:
			normalized[name] = ""
		case string:
			normalized[name] = v
		default:
			normalized[name] = fmt.Sprint(v)
		}
	}
	return normalized
}
