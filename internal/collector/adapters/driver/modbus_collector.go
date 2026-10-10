package collector

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/simonvetter/modbus"
	modbusspec "data-collection-system/internal/protocol/modbus"
	"data-collection-system/internal/shared/logger"
	templatebiz "data-collection-system/internal/template/domain"
	"data-collection-system/internal/valueobj"
)

const (
	// 最大连续错误次数，超过后触发重连
	maxConsecutiveErrors = 5
	// 错误恢复等待时间
	recoveryWaitTime = 3 * time.Second
)

// ModbusCollector 是 Modbus 协议采集器。
// 它的职责非常聚焦：
// - 按设备参数建立 Modbus 连接；
// - 按模板点位定义读取寄存器/线圈；
// - 把原始寄存器数据解码成统一的 CollectedValue。
// 它不关心数据库，不关心告警规则，也不关心 HTTP。
type ModbusCollector struct {
	client             *modbus.ModbusClient
	mu                 sync.Mutex
	transportProfile   modbusTransportProfile
	transportErr       error
	slaveID            uint8 // 从站地址
	maxContinuousCount int   // 单次连续读取最大数量

	consecutiveErrors int       // 连续错误计数
	lastErrorTime     time.Time // 上次错误时间
}

// NewModbusCollector 创建 Modbus 采集器实例。
// 这里先把参数收敛成 transportProfile，再由运行时决定用 RTU、TCP 还是透传链路。
func NewModbusCollector(params map[string]string) *ModbusCollector {
	transportProfile, err := buildModbusTransportProfile(params)
	c := &ModbusCollector{
		maxContinuousCount: 100,
		transportProfile:   transportProfile,
		transportErr:       err,
	}
	if v := strings.TrimSpace(params["baudrate"]); v != "" {
		if rate, parseErr := parseIntParam(v); parseErr == nil && rate > 0 {
			c.transportProfile.BaudRate = rate
		}
	}
	if v := strings.TrimSpace(params["slave_addr"]); v != "" {
		if id, parseErr := parseIntParam(v); parseErr == nil && id > 0 && id <= 247 {
			c.slaveID = uint8(id)
		}
	}
	if c.slaveID == 0 {
		c.slaveID = 1
	}
	if v := params["max_continuous_count"]; v != "" {
		if count, err := parseIntParam(v); err == nil && count > 0 && count <= 100 {
			c.maxContinuousCount = count
		}
	}

	return c
}

func init() {
	// 协议采集器通过 init() 自注册到全局工厂。
	// 这样上层只需要按 collectMethod 取实例，不需要手写一个巨大的 switch。
	RegisterCollector("modbus", func(params map[string]string) Collector {
		return NewModbusCollector(params)
	})
}

// Collect 执行一轮采集。
// 先按连续地址规划窗口，再逐窗读取并回填点位值；这样分段优化和故障统计都留在采集器层。
//
// 返回约定：只要有一个读取窗口收到有效应答就返回 nil error（允许其它窗口失败）；
// 若存在读取窗口但全部失败（超时/无应答），返回错误，交由上层按采集失败进入离线判定。
// 注意：单点解码失败不算窗口失败——设备有响应即视为可达，只是数据不合规。
func (c *ModbusCollector) Collect(ctx context.Context, deviceID int, points []templatebiz.DevicePoint, params map[string]string) (map[int]valueobj.CollectedValue, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.transportErr != nil {
		// 协议类型或运行时参数在构造阶段就无效时，采集器不进入后续连接流程。
		return nil, c.transportErr
	}

	result := make(map[int]valueobj.CollectedValue, len(points))
	failedCount := 0

	// 确保连接可用
	if err := c.ensureConnection(); err != nil {
		return nil, fmt.Errorf("Modbus 连接失败: %w", err)
	}

	// 连续错误达到阈值后，先进入等待窗口，避免坏链路上反复抢占资源。
	if c.consecutiveErrors >= maxConsecutiveErrors && time.Since(c.lastErrorTime) < recoveryWaitTime {
		return nil, fmt.Errorf("连续错误过多，等待恢复中（已连续失败 %d 次）", c.consecutiveErrors)
	}

	windows := planModbusReadWindows(points, c.maxContinuousCount)
	succeededWindows := 0
	windowFailures := 0
	for i, window := range windows {
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		default:
		}

		// 上一个窗口失败可能已触发阈值、关闭并把 c.client 置空（见 recordErrorCount）。
		// 继续读下一个窗口会在 nil 句柄上解引用（SetUnitId 第一句就崩），
		// 所以本轮立即收手，未读的窗口留给下一轮重连后再取。
		if c.client == nil {
			logger.Warnf("[ModbusCollector] 设备 %d 连接已因连续错误关闭，本轮跳过剩余 %d 个读取窗口",
				deviceID, len(windows)-i)
			break
		}

		windowFailedCount, err := c.collectWindow(ctx, deviceID, window, result)
		if err != nil {
			windowFailures++
			logger.Warnf("[ModbusCollector] 设备 %d 连续窗口读取失败: 起始 %d, 数量 %d, 功能码 0x%02X: %v",
				deviceID, window.startAddress, window.readCount, window.readFuncCode, err)
		} else {
			if windowFailedCount > 0 {
				// 设备有响应、窗口也读回来了，但窗口内有解码失败的测点：
				// 单点数据问题，不算链路失败，但同样计入连续异常以保持阈值敏感。
				windowFailures++
			}
			succeededWindows++
		}
		c.recordErrorCount(windowFailedCount, err)
		failedCount += windowFailedCount
	}

	// 有读取窗口却一个都没成功，说明本轮没有任何有效响应（超时、串口无应答等）。
	// 必须返回错误：否则上层会当成本轮采集成功，把已经掉线的设备一直标记为在线。
	if len(windows) > 0 && succeededWindows == 0 {
		return nil, fmt.Errorf("Modbus 设备 %d 的 %d 个读取窗口全部失败（无有效响应）", deviceID, len(windows))
	}

	for _, point := range points {
		if point.PointID <= 0 {
			continue
		}
		if point.Modbus == nil || point.Modbus.RegisterAddress < 0 {
			result[point.PointID] = valueobj.NewNumberValue(0)
		}
	}

	// 只有整轮没有任何窗口异常时，才重置错误计数；部分失败仍应保留故障趋势。
	if windowFailures == 0 && len(result) > 0 {
		c.consecutiveErrors = 0
	}

	// 汇总日志
	totalPoints := len(points)
	if failedCount > 0 {
		logger.Debugf("[ModbusCollector] 设备 %d 本轮采集完成: 成功 %d/%d，失败 %d", deviceID, len(result), totalPoints, failedCount)
	}

	return result, nil
}

// Write 将原始类型值写入 Modbus 设备。
// 按 rawValue 的 Go 类型分发到不同的写操作：
// - bool → WriteCoil（写单线圈）
// - []bool → WriteCoils（写多线圈）
// - uint16 → WriteRegister（写单寄存器）
// - []uint16 → WriteRegisters（写多寄存器）
func (c *ModbusCollector) Write(ctx context.Context, deviceID int, point templatebiz.DevicePoint, rawValue interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.transportErr != nil {
		return c.transportErr
	}

	spec := point.Modbus
	if spec == nil {
		return fmt.Errorf("测点 %d 无 Modbus 配置", point.PointID)
	}
	addr := uint16(spec.RegisterAddress)

	// 确保连接可用
	if err := c.ensureConnection(); err != nil {
		return fmt.Errorf("Modbus 连接失败: %w", err)
	}

	if err := c.client.SetUnitId(c.slaveID); err != nil {
		return fmt.Errorf("设置从站 ID 失败: %w", err)
	}

	return c.withRequestAccess(ctx, func() error {
		switch v := rawValue.(type) {
		case bool:
			return c.client.WriteCoil(addr, v)
		case []bool:
			return c.client.WriteCoils(addr, v)
		case uint16:
			return c.client.WriteRegister(addr, v)
		case []uint16:
			return c.client.WriteRegisters(addr, v)
		case int16:
			return c.client.WriteRegister(addr, uint16(v))
		case int32:
			buf := make([]byte, 4)
			binary.BigEndian.PutUint32(buf, uint32(v))
			return c.client.WriteRegisters(addr, encodeToRegisters(buf, spec.EncodingOrder))
		case uint32:
			buf := make([]byte, 4)
			binary.BigEndian.PutUint32(buf, v)
			return c.client.WriteRegisters(addr, encodeToRegisters(buf, spec.EncodingOrder))
		case float32:
			buf := make([]byte, 4)
			binary.BigEndian.PutUint32(buf, math.Float32bits(v))
			return c.client.WriteRegisters(addr, encodeToRegisters(buf, spec.EncodingOrder))
		case int64:
			buf := make([]byte, 8)
			binary.BigEndian.PutUint64(buf, uint64(v))
			return c.client.WriteRegisters(addr, encodeToRegisters(buf, spec.EncodingOrder))
		case uint64:
			buf := make([]byte, 8)
			binary.BigEndian.PutUint64(buf, v)
			return c.client.WriteRegisters(addr, encodeToRegisters(buf, spec.EncodingOrder))
		case float64:
			buf := make([]byte, 8)
			binary.BigEndian.PutUint64(buf, math.Float64bits(v))
			return c.client.WriteRegisters(addr, encodeToRegisters(buf, spec.EncodingOrder))
		default:
			return fmt.Errorf("不支持的 Modbus 写值类型: %T", rawValue)
		}
	})
}

// Stop 停止采集并关闭 Modbus 连接。
// 运行态只持有一个客户端句柄，所以这里直接收口即可。
func (c *ModbusCollector) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.client != nil {
		if err := c.client.Close(); err != nil {
			logger.Warnf("[ModbusCollector] 关闭连接失败: %v", err)
		}
		c.client = nil
		logger.Infof("[ModbusCollector] 已停止采集，Modbus 连接已关闭")
	}
}

func (c *ModbusCollector) withRequestAccess(ctx context.Context, fn func() error) error {
	// 请求门禁按传输身份共享，保证同一物理串口或同一透传网关上的请求串行化。
	release, err := defaultModbusRequestGate.acquire(ctx, c.transportProfile.RequestGateKey(), c.transportProfile.RequestMinInterval())
	if err != nil {
		return err
	}
	defer release()
	return fn()
}

func (c *ModbusCollector) collectWindow(ctx context.Context, deviceID int, window modbusReadWindow, result map[int]valueobj.CollectedValue) (int, error) {
	// 连接可能已被上一个窗口的失败阈值关闭（c.client 置空）。
	// 对 nil 句柄调方法会 nil 解引用带走整个进程，这里降级成“本窗口失败”。
	if c.client == nil {
		return len(window.points), fmt.Errorf("Modbus 连接已关闭")
	}

	if err := c.client.SetUnitId(c.slaveID); err != nil {
		return len(window.points), fmt.Errorf("设置从站 ID 失败: %w", err)
	}

	switch window.readFuncCode {
	case modbusspec.FuncCodeReadCoils:
		return c.collectBoolWindow(window, result, func(addr uint16, count uint16) ([]bool, error) {
			var values []bool
			err := c.withRequestAccess(ctx, func() error {
				var err error
				values, err = c.client.ReadCoils(addr, count)
				return err
			})
			return values, err
		}, "线圈")

	case modbusspec.FuncCodeReadDiscreteInputs:
		return c.collectBoolWindow(window, result, func(addr uint16, count uint16) ([]bool, error) {
			var values []bool
			err := c.withRequestAccess(ctx, func() error {
				var err error
				values, err = c.client.ReadDiscreteInputs(addr, count)
				return err
			})
			return values, err
		}, "离散输入")

	default:
		// 寄存器类读取先按窗口取回原始字节，再按每个点自己的类型切片解码。
		regType := modbus.HOLDING_REGISTER
		if window.readFuncCode == modbusspec.FuncCodeReadInputRegisters {
			regType = modbus.INPUT_REGISTER
		}

		var registers []uint16
		err := c.withRequestAccess(ctx, func() error {
			var err error
			registers, err = c.client.ReadRegisters(window.startAddress, window.readCount, regType)
			return err
		})
		if err != nil {
			return len(window.points), fmt.Errorf("读寄存器 0x%04X 数量 %d 失败: %w", window.startAddress, window.readCount, err)
		}

		data := uint16SliceToBytes(registers)
		failedCount := 0
		for _, point := range window.points {
			offset := int(point.startAddress - window.startAddress)
			start := offset * 2
			end := start + int(point.registerCount)*2
			if end > len(data) {
				return len(window.points), fmt.Errorf("窗口数据不足，测点 %d 需要 %d 字节，实际 %d 字节", point.point.PointID, int(point.registerCount)*2, len(data)-start)
			}

			decoded, err := DecodeRegister(data[start:end], point.dataType, point.encodingOrder)
			if err != nil {
				// 单点解码失败不回退整窗，但要计入故障统计，保持恢复阈值敏感。
				logger.Warnf("[ModbusCollector] 设备 %d 测点 %d (%s) 解码失败: %v", deviceID, point.point.PointID, point.point.Name, err)
				failedCount++
				continue
			}

			if math.IsNaN(decoded) || math.IsInf(decoded, 0) {
				logger.Warnf("[ModbusCollector] 设备 %d 测点 %d (%s) 解码值为无效浮点数: %f", deviceID, point.point.PointID, point.point.Name, decoded)
				result[point.point.PointID] = valueobj.NewNumberValue(0)
				continue
			}

			result[point.point.PointID] = valueobj.NewNumberValue(decoded)
		}

		return failedCount, nil
	}
}

func (c *ModbusCollector) collectBoolWindow(window modbusReadWindow, result map[int]valueobj.CollectedValue, read func(addr uint16, count uint16) ([]bool, error), label string) (int, error) {
	values, err := read(window.startAddress, window.readCount)
	if err != nil {
		return len(window.points), fmt.Errorf("读%s 0x%04X 数量 %d 失败: %w", label, window.startAddress, window.readCount, err)
	}

	for _, point := range window.points {
		offset := int(point.startAddress - window.startAddress)
		end := offset + int(point.registerCount)
		if end > len(values) {
			return len(window.points), fmt.Errorf("窗口数据不足，测点 %d 需要 %d 位，实际 %d 位", point.point.PointID, int(point.registerCount), len(values)-offset)
		}

		value := 0.0
		if len(values[offset:end]) > 0 && values[offset] {
			value = 1
		}
		result[point.point.PointID] = valueobj.NewNumberValue(value)
	}

	return 0, nil
}

// ensureConnection 确保串口连接可用，必要时创建或重建连接。
// ModbusCollector 会复用连接，但当连续错误过多时会主动断开并重连。
func (c *ModbusCollector) ensureConnection() error {
	// 已连接且没有达到最大错误数，直接返回
	if c.client != nil && c.consecutiveErrors < maxConsecutiveErrors {
		return nil
	}

	// 关闭旧连接
	if c.client != nil {
		c.client.Close()
		c.client = nil
	}

	logger.Debugf("[ModbusCollector] 连接 Modbus 设备 (%s, 协议: %s, 从站: %d, 超时: %v)",
		c.transportProfile.TargetURL, c.transportProfile.ProtocolType, c.slaveID, c.transportProfile.Timeout)

	client, err := modbus.NewClient(c.transportProfile.ClientConfig())
	if err != nil {
		return fmt.Errorf("创建 Modbus 客户端失败: %w", err)
	}

	if err := client.Open(); err != nil {
		return fmt.Errorf("打开 Modbus 连接 %s 失败: %w", c.transportProfile.TargetURL, err)
	}

	c.client = client
	c.consecutiveErrors = 0
	logger.Debugf("[ModbusCollector] Modbus 连接成功: %s", c.transportProfile.TargetURL)
	return nil
}

// recordErrorCount 记录一次“窗口级”采集异常，连续异常达到阈值时关闭连接。
//
// 阈值口径是连续异常的窗口数，不是异常点数：一个 100 点的窗口整体读失败只算 1 次。
// 按点数累加时，一个窗口失败就能把阈值 5 冲爆，等价于“每次窗口失败必关连接”。
// failedPoints 只用于日志（本窗口有多少测点没取到值）；failure 为 nil 表示窗口读取
// 成功但有解码失败测点（数据不合规，同样计入连续异常）。
func (c *ModbusCollector) recordErrorCount(failedPoints int, failure error) {
	if failure == nil && failedPoints <= 0 {
		return
	}

	c.consecutiveErrors++
	c.lastErrorTime = time.Now()

	if c.consecutiveErrors >= maxConsecutiveErrors {
		logger.Warnf("[ModbusCollector] 连续 %d 个读取窗口异常（最近一次: 失败测点 %d, 错误: %v），关闭连接等待恢复",
			c.consecutiveErrors, failedPoints, failure)
		if c.client != nil {
			c.client.Close()
			c.client = nil
		}
	}
}

// uint16SliceToBytes 将 []uint16 转换为大端序 []byte（每寄存器 2 字节）
func uint16SliceToBytes(registers []uint16) []byte {
	data := make([]byte, len(registers)*2)
	for i, reg := range registers {
		data[i*2] = byte(reg >> 8)
		data[i*2+1] = byte(reg & 0xFF)
	}
	return data
}

// parseIntParam 安全解析整数字符串参数
func parseIntParam(s string) (int, error) {
	s = strings.TrimSpace(s)
	// strconv.Atoi 不支持 '+' 前缀，手动去除
	s = strings.TrimPrefix(s, "+")
	return strconv.Atoi(s)
}
