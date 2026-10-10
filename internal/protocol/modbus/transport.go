package modbus

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/simonvetter/modbus"
)

const (
	DefaultLocalRtuModbusTimeoutMs = 500
	DefaultNetworkModbusTimeoutMs  = 1000
)

// TransportProfile 把设备参数收敛成运行时传输画像。
type TransportProfile struct {
	ProtocolType    string
	TargetURL       string
	RequestKey      string
	RequestPacing   bool
	RequestInterval time.Duration
	BaudRate        int
	DataBits        uint
	Parity          uint
	StopBits        uint
	Timeout         time.Duration
}

// BuildTransportProfile 根据 params 构建传输配置。
func BuildTransportProfile(params map[string]string) (TransportProfile, error) {
	protocolType := strings.ToUpper(strings.TrimSpace(params["protocol_type"]))
	if protocolType == "" {
		protocolType = ProtocolRS485RTU
	}

	switch protocolType {
	case ProtocolRS485RTU, ProtocolRS232RTU:
		return buildLocalRtuProfile(protocolType, params)
	case ProtocolMODBUSTCP:
		return buildModbusTCPProfile(protocolType, params)
	case ProtocolTCPRTU:
		return buildRtuOverTCPProfile(protocolType, params, "rtuovertcp")
	case ProtocolUDPRTU:
		return buildRtuOverTCPProfile(protocolType, params, "rtuoverudp")
	default:
		return TransportProfile{}, fmt.Errorf("不支持的 Modbus 协议类型: %s", protocolType)
	}
}

func buildLocalRtuProfile(protocolType string, params map[string]string) (TransportProfile, error) {
	serial := strings.TrimSpace(params["serial"])
	if serial == "" {
		serial = "/dev/ttyS1"
	}

	baudRate := 9600
	if v := strings.TrimSpace(params["baudrate"]); v != "" {
		if rate, err := parseIntParam(v); err == nil && rate > 0 {
			baudRate = rate
		}
	}

	dataBits := uint(8)
	if v := strings.TrimSpace(params["data_bit"]); v != "" {
		if bits, err := parseIntParam(v); err == nil && bits > 0 {
			dataBits = uint(bits)
		}
	}

	stopBits := uint(1)
	if v := strings.TrimSpace(params["stop_bit"]); v != "" {
		if bits, err := parseIntParam(v); err == nil && bits > 0 {
			stopBits = uint(bits)
		}
	}

	parity := modbus.PARITY_NONE
	switch strings.ToUpper(strings.TrimSpace(params["parity"])) {
	case "O":
		parity = modbus.PARITY_ODD
	case "E":
		parity = modbus.PARITY_EVEN
	}

	interval := 30 * time.Millisecond
	if v := strings.TrimSpace(params["continuous_interval_ms"]); v != "" {
		if ms, err := parseIntParam(v); err == nil && ms >= 30 {
			interval = time.Duration(ms) * time.Millisecond
		}
	}

	return TransportProfile{
		ProtocolType:    protocolType,
		TargetURL:       fmt.Sprintf("rtu://%s", serial),
		RequestKey:      serial,
		RequestPacing:   true,
		RequestInterval: interval,
		BaudRate:        baudRate,
		DataBits:        dataBits,
		Parity:          parity,
		StopBits:        stopBits,
		Timeout:         parseModbusTimeout(params, DefaultLocalRtuModbusTimeoutMs),
	}, nil
}

func buildModbusTCPProfile(protocolType string, params map[string]string) (TransportProfile, error) {
	host := strings.TrimSpace(params["ip_address"])
	if host == "" {
		return TransportProfile{}, fmt.Errorf("MODBUS-TCP 缺少 ip_address")
	}

	port := 502
	if v := strings.TrimSpace(params["port"]); v != "" {
		if parsed, err := parseIntParam(v); err == nil && parsed > 0 && parsed <= 65535 {
			port = parsed
		} else {
			return TransportProfile{}, fmt.Errorf("MODBUS-TCP 的 port 无效: %s", v)
		}
	}

	return TransportProfile{
		ProtocolType: protocolType,
		TargetURL:    "tcp://" + net.JoinHostPort(host, strconv.Itoa(port)),
		Timeout:      parseModbusTimeout(params, DefaultNetworkModbusTimeoutMs),
	}, nil
}

func buildRtuOverTCPProfile(protocolType string, params map[string]string, scheme string) (TransportProfile, error) {
	host := strings.TrimSpace(params["ip_address"])
	if host == "" {
		return TransportProfile{}, fmt.Errorf("%s 缺少 ip_address", protocolType)
	}

	port := strings.TrimSpace(params["port"])
	if port == "" {
		return TransportProfile{}, fmt.Errorf("%s 缺少 port", protocolType)
	}
	if parsedPort, err := parseIntParam(port); err != nil || parsedPort <= 0 || parsedPort > 65535 {
		return TransportProfile{}, fmt.Errorf("%s 的 port 无效: %s", protocolType, port)
	}

	baudRate := 9600
	if v := strings.TrimSpace(params["baudrate"]); v != "" {
		if rate, err := parseIntParam(v); err == nil && rate > 0 {
			baudRate = rate
		}
	}

	interval := 30 * time.Millisecond
	if v := strings.TrimSpace(params["continuous_interval_ms"]); v != "" {
		if ms, err := parseIntParam(v); err == nil && ms >= 30 {
			interval = time.Duration(ms) * time.Millisecond
		}
	}

	return TransportProfile{
		ProtocolType:    protocolType,
		TargetURL:       fmt.Sprintf("%s://%s", scheme, net.JoinHostPort(host, port)),
		RequestKey:      net.JoinHostPort(host, port),
		RequestPacing:   true,
		RequestInterval: interval,
		BaudRate:        baudRate,
		DataBits:        8,
		StopBits:        1,
		Parity:          modbus.PARITY_NONE,
		Timeout:         parseModbusTimeout(params, DefaultNetworkModbusTimeoutMs),
	}, nil
}

func parseModbusTimeout(params map[string]string, defaultTimeoutMs int) time.Duration {
	timeout := time.Duration(defaultTimeoutMs) * time.Millisecond
	if v := strings.TrimSpace(params["timeout"]); v != "" {
		if ms, err := parseIntParam(v); err == nil && ms > 0 {
			timeout = time.Duration(ms) * time.Millisecond
		}
	}
	return timeout
}

func parseIntParam(s string) (int, error) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "+")
	return strconv.Atoi(s)
}

// ClientConfig 把传输画像转换成 modbus 库的客户端配置。
func (p TransportProfile) ClientConfig() *modbus.ClientConfiguration {
	return &modbus.ClientConfiguration{
		URL:      p.TargetURL,
		Timeout:  p.Timeout,
		Speed:    uint(p.BaudRate),
		DataBits: p.DataBits,
		Parity:   p.Parity,
		StopBits: p.StopBits,
	}
}

// RequestGateKey 返回请求门禁的共享 key。
func (p TransportProfile) RequestGateKey() string {
	if !p.RequestPacing {
		return ""
	}
	return p.RequestKey
}

// RequestMinInterval 返回当前传输要求的最小请求间隔。
func (p TransportProfile) RequestMinInterval() time.Duration {
	if !p.RequestPacing {
		return 0
	}
	return p.RequestInterval
}

// RequiresPacing 表示当前传输是否需要请求节流。
func (p TransportProfile) RequiresPacing() bool {
	return p.RequestPacing && p.RequestKey != ""
}
