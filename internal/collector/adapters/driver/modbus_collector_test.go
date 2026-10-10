package collector

import (
	"context"
	"errors"
	"net"
	"strings"
	"sync"
	"testing"

	modbusspec "data-collection-system/internal/protocol/modbus"
	templatebiz "data-collection-system/internal/template/domain"
	"data-collection-system/internal/valueobj"
)

func TestNewModbusCollectorParamAliases(t *testing.T) {
	c := NewModbusCollector(map[string]string{
		"protocol_type": "RS485-RTU",
		"serial":        "/dev/ttyUSB9",
		"baudrate":      "115200",
		"slave_addr":    "12",
		"timeout":       "3000",
	})

	if c.transportErr != nil {
		t.Fatalf("transportErr = %v", c.transportErr)
	}
	if c.transportProfile.TargetURL != "rtu:///dev/ttyUSB9" {
		t.Fatalf("targetURL = %q, want %q", c.transportProfile.TargetURL, "rtu:///dev/ttyUSB9")
	}
	if c.transportProfile.BaudRate != 115200 {
		t.Fatalf("baudRate = %d, want %d", c.transportProfile.BaudRate, 115200)
	}
	if c.slaveID != 12 {
		t.Fatalf("slaveID = %d, want %d", c.slaveID, 12)
	}
	if got := c.transportProfile.Timeout.Milliseconds(); got != 3000 {
		t.Fatalf("timeout = %dms, want %dms", got, 3000)
	}
}

func TestModbusCollectorRecordErrorCount(t *testing.T) {
	c := NewModbusCollector(nil)

	c.recordErrorCount(3, errors.New("读寄存器失败"))
	if c.consecutiveErrors != 1 {
		t.Fatalf("一次窗口异常只应计 1 次，实际 %d", c.consecutiveErrors)
	}
	if c.lastErrorTime.IsZero() {
		t.Fatal("lastErrorTime should be set")
	}

	before := c.consecutiveErrors
	c.recordErrorCount(0, nil)
	if c.consecutiveErrors != before {
		t.Fatalf("无异常时 consecutiveErrors 不应变化: got %d, want %d", c.consecutiveErrors, before)
	}
}

// 阈值口径必须是“连续异常的窗口数”，不是“异常点数”：
// 一个 100 点的窗口整体读失败只算 1 次；否则一次窗口失败就把阈值 5 冲爆，
// 等价于“每次窗口失败必关连接”。（修复前该用例失败：一次即计到 100。）
func TestModbusCollectorErrorThresholdCountsWindowsNotPoints(t *testing.T) {
	c := NewModbusCollector(nil)

	c.recordErrorCount(100, errors.New("读寄存器失败"))
	if c.consecutiveErrors != 1 {
		t.Fatalf("100 点窗口一次失败应只计 1 次，实际 %d", c.consecutiveErrors)
	}

	for i := 0; i < maxConsecutiveErrors-1; i++ {
		c.recordErrorCount(1, errors.New("读寄存器失败"))
	}
	if c.consecutiveErrors != maxConsecutiveErrors {
		t.Fatalf("连续 %d 个窗口异常后 consecutiveErrors = %d",
			maxConsecutiveErrors, c.consecutiveErrors)
	}

	// 窗口读回来了但存在解码失败测点：同样算一次异常（保持恢复阈值敏感）。
	before := c.consecutiveErrors
	c.recordErrorCount(2, nil)
	if c.consecutiveErrors != before+1 {
		t.Fatalf("解码失败应计 1 次异常，实际增加 %d", c.consecutiveErrors-before)
	}
}

func TestNewModbusCollectorProtocolRouting(t *testing.T) {
	tests := []struct {
		name               string
		protocolType       string
		params             map[string]string
		wantTargetURL      string
		wantGateKey        string
		wantPacing         bool
		wantBaudRate       int
		wantTimeoutMs      int64
		wantTransportError bool
	}{
		{
			name:         "local rtu",
			protocolType: "RS485-RTU",
			params: map[string]string{
				"serial":     "/dev/ttyUSB0",
				"slave_addr": "1",
			},
			wantTargetURL: "rtu:///dev/ttyUSB0",
			wantGateKey:   "/dev/ttyUSB0",
			wantPacing:    true,
			wantBaudRate:  9600,
			wantTimeoutMs: 500,
		},
		{
			name:         "modbus tcp",
			protocolType: "MODBUS-TCP",
			params: map[string]string{
				"ip_address": "192.168.1.10",
				"slave_addr": "1",
			},
			wantTargetURL: "tcp://192.168.1.10:502",
			wantGateKey:   "",
			wantPacing:    false,
			wantTimeoutMs: 1000,
		},
		{
			name:         "rtu over tcp",
			protocolType: "TCP-RTU",
			params: map[string]string{
				"ip_address": "192.168.1.20",
				"port":       "1502",
				"slave_addr": "2",
				"baudrate":   "19200",
			},
			wantTargetURL: "rtuovertcp://192.168.1.20:1502",
			wantGateKey:   "192.168.1.20:1502",
			wantPacing:    true,
			wantBaudRate:  19200,
			wantTimeoutMs: 1000,
		},
		{
			name:         "invalid protocol",
			protocolType: "UNKNOWN",
			params: map[string]string{
				"serial":     "/dev/ttyUSB0",
				"slave_addr": "1",
			},
			wantTransportError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := map[string]string{}
			for k, v := range tt.params {
				params[k] = v
			}
			params["protocol_type"] = tt.protocolType

			c := NewModbusCollector(params)
			if tt.wantTransportError {
				if c.transportErr == nil {
					t.Fatal("expected transport error")
				}
				return
			}
			if c.transportErr != nil {
				t.Fatalf("transportErr = %v", c.transportErr)
			}
			if c.transportProfile.TargetURL != tt.wantTargetURL {
				t.Fatalf("targetURL = %q, want %q", c.transportProfile.TargetURL, tt.wantTargetURL)
			}
			if c.transportProfile.RequestGateKey() != tt.wantGateKey {
				t.Fatalf("gateKey = %q, want %q", c.transportProfile.RequestGateKey(), tt.wantGateKey)
			}
			if c.transportProfile.RequiresPacing() != tt.wantPacing {
				t.Fatalf("requiresPacing = %v, want %v", c.transportProfile.RequiresPacing(), tt.wantPacing)
			}
			if tt.wantBaudRate != 0 && c.transportProfile.BaudRate != tt.wantBaudRate {
				t.Fatalf("baudRate = %d, want %d", c.transportProfile.BaudRate, tt.wantBaudRate)
			}
			if tt.wantTimeoutMs != 0 && c.transportProfile.Timeout.Milliseconds() != tt.wantTimeoutMs {
				t.Fatalf("timeout = %dms, want %dms", c.transportProfile.Timeout.Milliseconds(), tt.wantTimeoutMs)
			}
		})
	}
}

// 设备完全不响应（所有读取窗口都超时失败）时，Collect 必须返回错误，
// 让上层 DeviceCollector 走失败分支累计重试、最终判定离线；
// 修复前这里返回 nil error，会把掉线设备一直标记为在线。
func TestModbusCollectAllWindowsFailReturnsError(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	// 接受连接但不回任何数据，模拟设备超时无应答。
	var mu sync.Mutex
	var conns []net.Conn
	acceptDone := make(chan struct{})
	go func() {
		defer close(acceptDone)
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			mu.Lock()
			conns = append(conns, conn)
			mu.Unlock()
		}
	}()
	t.Cleanup(func() {
		_ = ln.Close()
		<-acceptDone
		mu.Lock()
		for _, conn := range conns {
			_ = conn.Close()
		}
		mu.Unlock()
	})

	host, port, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}

	params := map[string]string{
		"protocol_type": modbusspec.ProtocolMODBUSTCP,
		"ip_address":    host,
		"port":          port,
		"timeout":       "200",
	}
	c := NewModbusCollector(params)

	points := []templatebiz.DevicePoint{
		testModbusPoint(1, 10, 1, modbusspec.FuncCodeReadHoldingRegisters, valueobj.RawTypeUINT16),
	}

	data, err := c.Collect(context.Background(), 1, points, params)
	if err == nil {
		t.Fatal("所有读取窗口失败时应返回错误，实际返回 nil")
	}
	if !strings.Contains(err.Error(), "全部失败") {
		t.Fatalf("期望“全部失败”错误，实际: %v", err)
	}
	if len(data) != 0 {
		t.Fatalf("失败轮次不应返回数据，实际 %d 个", len(data))
	}
}

// startSilentModbusTCPServer 起一个只接受连接、不回任何数据的 TCP 服务端，
// 用来模拟“设备在线但不应答”，并在用例结束时关闭连接。
func startSilentModbusTCPServer(t *testing.T) (host, port string) {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	var mu sync.Mutex
	var conns []net.Conn
	acceptDone := make(chan struct{})
	go func() {
		defer close(acceptDone)
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			mu.Lock()
			conns = append(conns, conn)
			mu.Unlock()
		}
	}()
	t.Cleanup(func() {
		_ = ln.Close()
		<-acceptDone
		mu.Lock()
		for _, conn := range conns {
			_ = conn.Close()
		}
		mu.Unlock()
	})

	host, port, err = net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	return host, port
}

// 窗口循环中途连接被错误阈值关闭（recordErrorCount 会 Close 并把 c.client 置 nil）后，
// 剩余窗口不能再对 nil 句柄调 SetUnitId——那是 nil 解引用，会带走整个进程
// （现场表现就是：一台设备的采集把整机打成 systemd 崩溃循环）。
//
// 用例形状：6 个互不连续的地址 → 6 个独立窗口，每个 1 个点。
// 前 5 个窗口连续超时后阈值 5 触发（关闭并置空连接），第 6 个窗口必须被跳过而不是拿去用。
func TestModbusCollectNoPanicWhenConnectionClosedMidRound(t *testing.T) {
	host, port := startSilentModbusTCPServer(t)

	params := map[string]string{
		"protocol_type": modbusspec.ProtocolMODBUSTCP,
		"ip_address":    host,
		"port":          port,
		"timeout":       "200",
	}
	c := NewModbusCollector(params)

	points := make([]templatebiz.DevicePoint, 0, 6)
	for i, addr := range []int{0, 100, 200, 300, 400, 500} {
		points = append(points, testModbusPoint(i+1, addr, 1, modbusspec.FuncCodeReadHoldingRegisters, valueobj.RawTypeUINT16))
	}

	if windows := planModbusReadWindows(points, c.maxContinuousCount); len(windows) != 6 {
		t.Fatalf("用例前提不成立：需要 6 个读取窗口，实际 %d", len(windows))
	}

	_, err := c.Collect(context.Background(), 7, points, params)
	if err == nil {
		t.Fatal("全部窗口失败时应返回错误")
	}
	if c.client != nil {
		t.Fatal("第 5 个窗口失败触发阈值后，连接应已被关闭并置空")
	}
	if c.consecutiveErrors != maxConsecutiveErrors {
		t.Fatalf("连续窗口异常数 = %d, want %d（第 6 个窗口被跳过，不应再累加）",
			c.consecutiveErrors, maxConsecutiveErrors)
	}
}
