package protocol

import (
	modbusproto "data-collection-system/internal/protocol/modbus"
	snmpproto "data-collection-system/internal/protocol/snmp"
)

// ProtocolType 协议类型
type ProtocolType string

const (
	ProtocolRS485RTU  ProtocolType = "RS485-RTU"
	ProtocolRS232RTU  ProtocolType = "RS232-RTU"
	ProtocolMODBUSTCP ProtocolType = "MODBUS-TCP"
	ProtocolTCPRTU    ProtocolType = "TCP-RTU"
	ProtocolUDPRTU    ProtocolType = "UDP-RTU"
	ProtocolSNMP      ProtocolType = "SNMP"
)

// ProtocolInfo 协议信息
type ProtocolInfo struct {
	Type        ProtocolType `json:"type"`
	NameCN      string       `json:"name_cn"`
	NameEN      string       `json:"name_en"`
	Description string       `json:"description"`
	Category    string       `json:"category"`
}

type ProtocolValidator func(params map[string]string) []ValidationError

type ProtocolSpec struct {
	Info      ProtocolInfo
	Params    []ParamDefinition
	Validator ProtocolValidator
}

var (
	protocolRegistry = map[ProtocolType]ProtocolSpec{}
	// protocolOrder 记录注册顺序。registry 是 map, 直接遍历的顺序随机会让前端
	// 「协议类型」下拉每次刷新乱序；注册顺序由静态协议列表（modbus/snmp 包内声明序）
	// 决定, 稳定且符合声明意图。
	protocolOrder []ProtocolType
)

func RegisterProtocol(spec ProtocolSpec) {
	if spec.Info.Type == "" {
		return
	}
	if _, exists := protocolRegistry[spec.Info.Type]; !exists {
		protocolOrder = append(protocolOrder, spec.Info.Type)
	}
	protocolRegistry[spec.Info.Type] = spec
}

func GetModbusProtocolTypes() []ProtocolInfo {
	all := GetAllProtocolTypes()
	out := make([]ProtocolInfo, 0, len(all))
	for _, info := range all {
		if info.Category == "modbus" {
			out = append(out, info)
		}
	}
	return out
}

func GetSNMPProtocolTypes() []ProtocolInfo {
	all := GetAllProtocolTypes()
	out := make([]ProtocolInfo, 0, len(all))
	for _, info := range all {
		if info.Category == "snmp" {
			out = append(out, info)
		}
	}
	return out
}

func GetAllProtocolTypes() []ProtocolInfo {
	out := make([]ProtocolInfo, 0, len(protocolOrder))
	for _, t := range protocolOrder {
		out = append(out, protocolRegistry[t].Info)
	}
	return out
}

// GetProtocolParams 根据协议类型获取专有参数定义
func GetProtocolParams(protocolType ProtocolType) []ParamDefinition {
	if spec, ok := protocolRegistry[protocolType]; ok {
		return spec.Params
	}
	return nil
}

func getProtocolValidator(protocolType ProtocolType) ProtocolValidator {
	if spec, ok := protocolRegistry[protocolType]; ok {
		return spec.Validator
	}
	return nil
}

func registerBuiltInProtocols() {
	modbusTypes := modbusproto.GetProtocolTypes()
	for _, item := range modbusTypes {
		pt := ProtocolType(item.Type)
		modbusType := item.Type
		RegisterProtocol(ProtocolSpec{
			Info: ProtocolInfo{
				Type:        pt,
				NameCN:      item.NameCN,
				NameEN:      item.NameEN,
				Description: item.Description,
				Category:    item.Category,
			},
			Params: modbusproto.GetProtocolParams(item.Type),
			Validator: func(params map[string]string) []ValidationError {
				return modbusproto.ValidateParams(modbusType, params)
			},
		})
	}

	for _, item := range snmpproto.GetProtocolTypes() {
		RegisterProtocol(ProtocolSpec{
			Info: ProtocolInfo{
				Type:        ProtocolType(item.Type),
				NameCN:      item.NameCN,
				NameEN:      item.NameEN,
				Description: item.Description,
				Category:    item.Category,
			},
			Params: snmpproto.GetProtocolParams(),
			Validator: func(params map[string]string) []ValidationError {
				return snmpproto.ValidateParams(params)
			},
		})
	}
}

func init() {
	registerBuiltInProtocols()
}

func GetProtocolInfo(protocolType ProtocolType) *ProtocolInfo {
	if spec, ok := protocolRegistry[protocolType]; ok {
		info := spec.Info
		return &info
	}
	return nil
}

func GetProtocolParamDefaults(protocolType ProtocolType) map[string]string {
	params := GetProtocolParams(protocolType)
	defaults := make(map[string]string)
	for _, param := range params {
		if param.Default != "" {
			defaults[param.Name] = param.Default
		}
	}
	return defaults
}

// GetAllParamDefaults 获取所有参数默认值（公共 + 协议专有）
func GetAllParamDefaults(protocolType ProtocolType) map[string]string {
	defaults := GetCommonParamDefaults()
	for k, v := range GetProtocolParamDefaults(protocolType) {
		defaults[k] = v
	}
	return defaults
}

func GetProtocolInfoByCategory(category string) []ProtocolInfo {
	all := GetAllProtocolTypes()
	out := make([]ProtocolInfo, 0, len(all))
	for _, info := range all {
		if info.Category == category {
			out = append(out, info)
		}
	}
	return out
}
