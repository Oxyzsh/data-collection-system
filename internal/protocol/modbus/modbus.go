package modbus

import protocolcommon "data-collection-system/internal/protocol/common"

const (
	ProtocolRS485RTU  = "RS485-RTU"
	ProtocolRS232RTU  = "RS232-RTU"
	ProtocolMODBUSTCP = "MODBUS-TCP"
	ProtocolTCPRTU    = "TCP-RTU"
	ProtocolUDPRTU    = "UDP-RTU"
)

type ProtocolInfo struct {
	Type        string `json:"type"`
	NameCN      string `json:"name_cn"`
	NameEN      string `json:"name_en"`
	Description string `json:"description"`
	Category    string `json:"category"`
}

var protocolTypes = []ProtocolInfo{
	{
		Type:        ProtocolRS485RTU,
		NameCN:      "RS485-RTU",
		NameEN:      "RS485-RTU",
		Description: "Modbus RTU over RS485 串口通信",
		Category:    "modbus",
	},
	{
		Type:        ProtocolRS232RTU,
		NameCN:      "RS232-RTU",
		NameEN:      "RS232-RTU",
		Description: "Modbus RTU over RS232 串口通信",
		Category:    "modbus",
	},
	{
		Type:        ProtocolMODBUSTCP,
		NameCN:      "MODBUS-TCP",
		NameEN:      "MODBUS-TCP",
		Description: "Modbus TCP over Ethernet",
		Category:    "modbus",
	},
	{
		Type:        ProtocolTCPRTU,
		NameCN:      "TCP-RTU",
		NameEN:      "TCP-RTU",
		Description: "Modbus RTU over TCP (透传模式)",
		Category:    "modbus",
	},
	{
		Type:        ProtocolUDPRTU,
		NameCN:      "UDP-RTU",
		NameEN:      "UDP-RTU",
		Description: "Modbus RTU over UDP (透传模式)",
		Category:    "modbus",
	},
}

var serialParams = []protocolcommon.ParamDefinition{
	{
		Name:        "serial",
		NameCN:      "串口",
		NameEN:      "Serial Port",
		Type:        protocolcommon.ParamTypeString,
		Required:    true,
		Default:     "",
		Description: "设备串口路径，如 /dev/ttyUSB0 或 COM1",
	},
	{
		Name:        "baudrate",
		NameCN:      "波特率",
		NameEN:      "Baud Rate",
		Type:        protocolcommon.ParamTypeEnum,
		Required:    false,
		Options:     []string{"1200", "2400", "4800", "9600", "19200", "38400", "57600", "115200"},
		Default:     "9600",
		Description: "串口波特率",
	},
	{
		Name:        "data_bit",
		NameCN:      "数据位",
		NameEN:      "Data Bits",
		Type:        protocolcommon.ParamTypeEnum,
		Required:    false,
		Options:     []string{"5", "6", "7", "8"},
		Default:     "8",
		Description: "串口数据位",
	},
	{
		Name:        "stop_bit",
		NameCN:      "停止位",
		NameEN:      "Stop Bits",
		Type:        protocolcommon.ParamTypeEnum,
		Required:    false,
		Options:     []string{"1", "2"},
		Default:     "1",
		Description: "串口停止位",
	},
	{
		Name:        "parity",
		NameCN:      "校验位",
		NameEN:      "Parity",
		Type:        protocolcommon.ParamTypeEnum,
		Required:    false,
		Options:     []string{"N", "O", "E"},
		Default:     "N",
		Description: "串口校验位：N=无校验, O=奇校验, E=偶校验",
	},
	{
		Name:        "slave_addr",
		NameCN:      "从机地址",
		NameEN:      "Slave Address",
		Type:        protocolcommon.ParamTypeInt,
		Required:    true,
		Min:         1,
		Max:         247,
		Default:     "1",
		Description: "Modbus 从机地址，范围 1-247",
	},
}

var modbusTCPParams = []protocolcommon.ParamDefinition{
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
		Default:     "502",
		Description: "Modbus TCP 端口号，默认 502",
	},
	{
		Name:        "slave_addr",
		NameCN:      "从机地址",
		NameEN:      "Slave Address",
		Type:        protocolcommon.ParamTypeInt,
		Required:    true,
		Min:         1,
		Max:         247,
		Default:     "1",
		Description: "Modbus 从机地址，范围 1-247",
	},
}

var tcpRTUParams = []protocolcommon.ParamDefinition{
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
		NameCN:      "端口",
		NameEN:      "Port",
		Type:        protocolcommon.ParamTypeInt,
		Required:    true,
		Min:         1,
		Max:         65535,
		Default:     "",
		Description: "设备端口号",
	},
	{
		Name:        "baudrate",
		NameCN:      "波特率",
		NameEN:      "Baud Rate",
		Type:        protocolcommon.ParamTypeEnum,
		Required:    false,
		Options:     []string{"1200", "2400", "4800", "9600", "19200", "38400", "57600", "115200"},
		Default:     "9600",
		Description: "透传链路波特率",
	},
	{
		Name:        "slave_addr",
		NameCN:      "从机地址",
		NameEN:      "Slave Address",
		Type:        protocolcommon.ParamTypeInt,
		Required:    true,
		Min:         1,
		Max:         247,
		Default:     "1",
		Description: "Modbus 从机地址，范围 1-247",
	},
}

// modbusWindowParams 是所有 Modbus 协议共享的连续读取上限。
// 这个值约束的是“单次窗口能读多大”，不是某一种传输介质的私有参数；
// 真正的协议标准上限会在采集器 planner 里再收敛一次。
var modbusWindowParams = []protocolcommon.ParamDefinition{
	{
		Name:        "max_continuous_count",
		NameCN:      "最大连续读取数量",
		NameEN:      "Max Continuous Read Count",
		Type:        protocolcommon.ParamTypeInt,
		Required:    false,
		Min:         1,
		Max:         2000,
		Default:     "100",
		Description: "连续读取窗口的最大数量，适用于寄存器、线圈和离散输入",
	},
}

// modbusRTUIntervalParams 仅描述需要节流的 RTU / RTU 透传场景。
// 标准 Modbus TCP 不依赖这个间隔参数。
var modbusRTUIntervalParams = []protocolcommon.ParamDefinition{
	{
		Name:        "continuous_interval_ms",
		NameCN:      "连续采集间隔(ms)",
		NameEN:      "Continuous Collect Interval(ms)",
		Type:        protocolcommon.ParamTypeInt,
		Required:    false,
		Min:         30,
		Max:         60000,
		Default:     "30",
		Description: "两次 Modbus 请求之间的最小间隔，单位毫秒",
	},
}

// cloneParamDefinitions 复制参数定义切片，避免组装参数集时共享底层数组。
func cloneParamDefinitions(defs []protocolcommon.ParamDefinition) []protocolcommon.ParamDefinition {
	out := make([]protocolcommon.ParamDefinition, len(defs))
	copy(out, defs)
	return out
}

// joinParamDefinitions 将多个参数组拼成一个新切片。
// 这里保留独立分组，再在 init 阶段合并，方便按协议组合不同参数族。
func joinParamDefinitions(groups ...[]protocolcommon.ParamDefinition) []protocolcommon.ParamDefinition {
	total := 0
	for _, group := range groups {
		total += len(group)
	}

	out := make([]protocolcommon.ParamDefinition, 0, total)
	for _, group := range groups {
		out = append(out, cloneParamDefinitions(group)...)
	}
	return out
}

func init() {
	// 把协议专属参数和通用窗口参数组合起来，保证各协议返回的参数集各自独立。
	serialParams = joinParamDefinitions(serialParams, modbusWindowParams, modbusRTUIntervalParams)
	modbusTCPParams = joinParamDefinitions(modbusTCPParams, modbusWindowParams)
	tcpRTUParams = joinParamDefinitions(tcpRTUParams, modbusWindowParams, modbusRTUIntervalParams)
}

func GetProtocolTypes() []ProtocolInfo {
	return protocolTypes
}

func GetProtocolParams(protocolType string) []protocolcommon.ParamDefinition {
	switch protocolType {
	case ProtocolRS485RTU, ProtocolRS232RTU:
		return serialParams
	case ProtocolMODBUSTCP:
		return modbusTCPParams
	case ProtocolTCPRTU, ProtocolUDPRTU:
		return tcpRTUParams
	default:
		return nil
	}
}
