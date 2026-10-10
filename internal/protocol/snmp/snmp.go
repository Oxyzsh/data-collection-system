package snmp

import protocolcommon "data-collection-system/internal/protocol/common"

const ProtocolSNMP = "SNMP"

type ProtocolInfo struct {
	Type        string `json:"type"`
	NameCN      string `json:"name_cn"`
	NameEN      string `json:"name_en"`
	Description string `json:"description"`
	Category    string `json:"category"`
}

var protocolTypes = []ProtocolInfo{
	{
		Type:        ProtocolSNMP,
		NameCN:      "SNMP",
		NameEN:      "SNMP",
		Description: "SNMP 协议设备参数（对接 gosnmp 采集器）",
		Category:    "snmp",
	},
}

var params = []protocolcommon.ParamDefinition{
	{
		Name:        "ip_address",
		NameCN:      "IP地址",
		NameEN:      "IP Address",
		Type:        protocolcommon.ParamTypeString,
		Required:    true,
		Default:     "",
		Description: "设备 IP 地址",
	},
	{
		Name:        "port",
		NameCN:      "端口号",
		NameEN:      "Port",
		Type:        protocolcommon.ParamTypeInt,
		Required:    false,
		Min:         1,
		Max:         65535,
		Default:     "161",
		Description: "SNMP 端口号，默认 161",
	},
	{
		Name:        "community",
		NameCN:      "团体名",
		NameEN:      "Community",
		Type:        protocolcommon.ParamTypeString,
		Required:    false,
		Default:     "public",
		Description: "SNMP v1/v2c 团体名",
	},
	{
		Name:        "snmp_v",
		NameCN:      "SNMP版本",
		NameEN:      "SNMP Version",
		Type:        protocolcommon.ParamTypeEnum,
		Required:    true,
		Options:     []string{"1", "2", "3"},
		Default:     "2",
		Description: "SNMP 协议版本，支持 v1/v2c/v3",
	},
	{
		Name:        "segment_max",
		NameCN:      "分段最大OID数",
		NameEN:      "Max OIDs per Segment",
		Type:        protocolcommon.ParamTypeInt,
		Required:    false,
		Min:         1,
		Max:         1000,
		Default:     "200",
		Description: "每批 SNMP 请求携带的最大 OID 数量",
	},
	{
		Name:        "user_name",
		NameCN:      "用户名",
		NameEN:      "User Name",
		Type:        protocolcommon.ParamTypeString,
		Required:    false,
		Default:     "",
		Description: "SNMPv3 用户名",
	},
	{
		Name:        "auth_protocol",
		NameCN:      "认证协议",
		NameEN:      "Auth Protocol",
		Type:        protocolcommon.ParamTypeEnum,
		Required:    false,
		Options:     []string{"MD5", "SHA"},
		Default:     "MD5",
		Description: "SNMPv3 认证协议",
	},
	{
		Name:        "auth_password",
		NameCN:      "认证密码",
		NameEN:      "Auth Password",
		Type:        protocolcommon.ParamTypeString,
		Required:    false,
		Default:     "",
		Description: "SNMPv3 认证密码",
	},
	{
		Name:        "priv_protocol",
		NameCN:      "加密协议",
		NameEN:      "Privacy Protocol",
		Type:        protocolcommon.ParamTypeEnum,
		Required:    false,
		Options:     []string{"DES", "AES"},
		Default:     "AES",
		Description: "SNMPv3 加密协议",
	},
	{
		Name:        "priv_password",
		NameCN:      "加密密码",
		NameEN:      "Privacy Password",
		Type:        protocolcommon.ParamTypeString,
		Required:    false,
		Default:     "",
		Description: "SNMPv3 加密密码",
	},
	{
		Name:        "security_level",
		NameCN:      "安全级别",
		NameEN:      "Security Level",
		Type:        protocolcommon.ParamTypeEnum,
		Required:    false,
		Options:     []string{"noAuthNoPriv", "authNoPriv", "authPriv"},
		Default:     "authPriv",
		Description: "SNMPv3 安全级别",
	},
}

func GetProtocolTypes() []ProtocolInfo {
	return protocolTypes
}

func GetProtocolParams() []protocolcommon.ParamDefinition {
	return params
}
