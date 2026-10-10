package parsers

import (
	templatebiz "data-collection-system/internal/template/domain"
)

// TemplateParser 是模板文件解析器接口。
// 每种协议模板实现一份，通过 RegisterParser 注册到全局注册表。
type TemplateParser interface {
	// Type 返回协议类型标识（如 "modbus"、"snmp"）。
	Type() string
	// Detect 根据表头特征判断是否为该协议的模板。
	Detect(headers map[string]int) bool
	// ParsePoints 解析测点数据行。
	ParsePoints(rows [][]string, headers map[string]int) ([]templatebiz.DevicePoint, error)
}

// parserRegistry 是全局解析器注册表。
var parserRegistry = map[string]TemplateParser{}

// supportedTypes 是已注册的协议类型列表，保持插入顺序。
var supportedTypes []string

// RegisterParser 注册模板解析器。
func RegisterParser(p TemplateParser) {
	if _, exists := parserRegistry[p.Type()]; !exists {
		supportedTypes = append(supportedTypes, p.Type())
	}
	parserRegistry[p.Type()] = p
}

// GetParser 根据类型获取已注册的解析器。
func GetParser(typ string) (TemplateParser, bool) {
	p, ok := parserRegistry[typ]
	return p, ok
}

// GetSupportedTypes 返回所有已注册的协议类型。
func GetSupportedTypes() []string {
	result := make([]string, len(supportedTypes))
	copy(result, supportedTypes)
	return result
}

// DetectType 遍历所有注册的解析器，自动检测模板类型。
// 返回第一个匹配的解析器类型；无匹配时返回 "modbus"（默认）。
func DetectType(headers map[string]int) string {
	for _, typ := range supportedTypes {
		if parserRegistry[typ].Detect(headers) {
			return typ
		}
	}
	return "modbus"
}

// init 接口不需要暴露给外部调用，它会在包加载时自动执行，注册内置解析器。
// 这是 Go 语言中常见的插件式设计模式，通过 init 函数实现自动注册，简化了使用者的配置工作。
// 是否会重复注册？不会，因为 RegisterParser 函数内部会检查是否已存在同类型的解析器，避免重复注册。
// 什么时候可以用这个机制？当你有一组功能相似但实现不同的组件（如不同协议的解析器）时，可以通过这种方式实现自动注册，降低模块间耦合，提高扩展性。
func init() {
	// 注册内置解析器
	RegisterParser(&localIOParser{typ: "local_di", channelHeader: "DI通道名称", direction: "input"})
	RegisterParser(&localIOParser{typ: "local_do", channelHeader: "DO通道名称", direction: "output"})
	RegisterParser(&localIOParser{typ: "local_io", channelHeader: "通道名称"})
	RegisterParser(&modbusParser{})
	RegisterParser(&snmpParser{})
}

// ==================== Modbus 解析器 ====================

type modbusParser struct{}

func (p *modbusParser) Type() string { return "modbus" }

func (p *modbusParser) Detect(headers map[string]int) bool {
	// Modbus 模板包含寄存器相关列，且不包含 OID 列
	_, hasOID := headers["OID"]
	if hasOID {
		return false
	}
	_, hasRegister := headers["寄存器地址"]
	return hasRegister
}

func (p *modbusParser) ParsePoints(rows [][]string, headers map[string]int) ([]templatebiz.DevicePoint, error) {
	return parseModbusPoints(rows, headers)
}

// ==================== SNMP 解析器 ====================

type snmpParser struct{}

func (p *snmpParser) Type() string { return "snmp" }

func (p *snmpParser) Detect(headers map[string]int) bool {
	// SNMP 模板包含 OID 列
	_, hasOID := headers["OID"]
	return hasOID
}

func (p *snmpParser) ParsePoints(rows [][]string, headers map[string]int) ([]templatebiz.DevicePoint, error) {
	return parseSNMPPoints(rows, headers)
}

// ==================== 本地 IO 解析器 ====================

type localIOParser struct {
	typ           string
	channelHeader string
	direction     string
}

func (p *localIOParser) Type() string { return p.typ }

func (p *localIOParser) Detect(headers map[string]int) bool {
	if _, hasOID := headers["OID"]; hasOID {
		return false
	}
	if _, hasRegister := headers["寄存器地址"]; hasRegister {
		return false
	}
	_, hasChannel := headers[p.channelHeader]
	return hasChannel
}

func (p *localIOParser) ParsePoints(rows [][]string, headers map[string]int) ([]templatebiz.DevicePoint, error) {
	return parseLocalIOPoints(rows, headers, p.channelHeader, p.direction, p.typ)
}
