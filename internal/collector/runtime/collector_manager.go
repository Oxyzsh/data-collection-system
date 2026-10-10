package runtime

import (
	"context"
	"fmt"
	"math"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	collectoradapter "data-collection-system/internal/collector/adapters/driver"
	collectorinfra "data-collection-system/internal/collector/infrastructure"
	historymodule "data-collection-system/internal/history"
	runtimeinfra "data-collection-system/internal/runtime/infrastructure"
	"data-collection-system/internal/shared/logger"
	templatebiz "data-collection-system/internal/template/domain"
	"data-collection-system/internal/valueobj"
)

// StopCollectorTimeout 是停止单个设备采集线程的默认等待上限。
// 导出供配置重建路径（collector/adapters/runtime）沿用同一口径。
const StopCollectorTimeout = 8 * time.Second

// DeviceCollector 是单个设备的采集运行单元。
// 它负责循环采集、处理数值、更新 runtime、写 history、提交告警。
//
// 可以把它看成“一个设备对应一个后台 worker”：
// - CollectorManager 负责管所有 worker；
// - DeviceCollector 负责真正执行某个设备的采集循环。
type DeviceCollector struct {
	DeviceID      int                         // 设备ID
	TemplateID    int                         // 模板ID
	CollectMethod string                      // 采集方法（modbus/snmp/simulator）
	Points        []templatebiz.DevicePoint   // 设备测点列表
	Params        map[string]string           // 设备参数（协议参数）
	Interval      int                         // 采集间隔（毫秒）
	Collector     collectoradapter.Collector  // 具体采集器实例
	deviceRuntime *runtimeinfra.DeviceRuntime // 设备运行时数据（通过 DataManager 管理）
	ctx           context.Context             // 上下文（含取消信号）
	cancel        context.CancelFunc          // 取消函数

	// wg 用于等待后台 goroutine 真正退出。
	// WaitGroup 可以理解成“并发计数器 + 等待器”：Add(+1) 表示有任务启动，Done() 表示任务结束，
	// Wait() 则阻塞到计数回到 0。
	wg             sync.WaitGroup
	consecFailures int  // 连续失败次数
	maxRetries     int  // 最大重试次数（用于离线判断）
	online         bool // 当前在线状态
	onlineReported bool // 是否已经写入过 point_id=0 在线状态
	// pointCfgMu 只保护测点启停配置，避免和 deviceRuntime 内部锁混在一起。
	pointCfgMu   sync.RWMutex
	pointEnabled map[int]bool // point_id -> enabled
}

// CollectorManager 负责管理全部设备采集器实例。
// 它不直接处理单次采样逻辑，重点是“启动/停止/重启/查询是否运行”。
type CollectorManager struct {
	collectors map[int]*DeviceCollector // 设备ID到采集器的映射
	mu         sync.RWMutex             // 并发安全锁
}

type PointValueSnapshot struct {
	Value     valueobj.CollectedValue
	Timestamp time.Time
}

// markDeviceStopped 用于把设备状态回写为 stopped。
func markDeviceStopped(deviceID int) error {
	return collectorinfra.UpdateDeviceStatus(deviceID, "stopped", time.Now())
}

var managerInstance *CollectorManager
var managerOnce sync.Once

// GetCollectorManager 获取采集器管理器实例。
// sync.Once 保证全局只初始化一次。
func GetCollectorManager() *CollectorManager {
	managerOnce.Do(func() {
		managerInstance = &CollectorManager{
			collectors: make(map[int]*DeviceCollector),
		}
	})
	return managerInstance
}

// StartCollector 启动设备采集器。
// 配置加载放在锁外，注册放在锁内，减少互相阻塞。
// 这是并发代码里很重要的习惯：慢操作尽量放锁外，锁内只做很短的共享状态修改。
func (m *CollectorManager) StartCollector(deviceID, templateID int) error {
	// 1. 锁外加载设备配置（DB 查询不阻塞其他设备）
	deviceCollector, err := m.loadDeviceConfig(deviceID, templateID)
	if err != nil {
		return err
	}

	// 2. 短时间加锁，仅做注册
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.collectors[deviceID]; exists {
		return fmt.Errorf("设备采集线程已存在")
	}

	m.collectors[deviceID] = deviceCollector

	// go 关键字会启动一个新的 goroutine。
	// 它不是操作系统线程本身，更像由 Go runtime 调度的轻量执行单元。
	go deviceCollector.run()
	return nil
}

// IsCollectorRunning 判断设备采集器是否正在运行。
func (m *CollectorManager) IsCollectorRunning(deviceID int) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	_, exists := m.collectors[deviceID]
	return exists
}

// RunningDeviceIDs 返回当前运行中的设备采集器 ID 快照（顺序不保证）。
// 供配置整体替换后的运行态重建（RebuildEnabledDevices）与配置集合对比使用。
func (m *CollectorManager) RunningDeviceIDs() []int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ids := make([]int, 0, len(m.collectors))
	for id := range m.collectors {
		ids = append(ids, id)
	}
	return ids
}

// RestartCollector 重启设备采集器。
// 先停再启，避免旧配置和新配置同时存在。
func (m *CollectorManager) RestartCollector(deviceID, templateID int) error {
	if m.IsCollectorRunning(deviceID) {
		if err := m.StopCollectorWithTimeout(deviceID, StopCollectorTimeout); err != nil {
			return err
		}
	}
	if err := m.StartCollector(deviceID, templateID); err != nil {
		if statusErr := markDeviceStopped(deviceID); statusErr != nil {
			logger.Warnf("[RestartCollector] 设备 %d 状态更新为 stopped 失败: %v", deviceID, statusErr)
		}
		return err
	}
	return nil
}

// loadDeviceConfig 加载设备采集配置。
// 这里把散落在数据库和模板缓存中的数据拼成一个启动快照。
// 对初学者来说，可以把它理解为“把启动 worker 所需的一次性依赖全部装配好”。
func (m *CollectorManager) loadDeviceConfig(deviceID, templateID int) (*DeviceCollector, error) {
	loaded, err := collectorinfra.LoadDeviceConfig(deviceID, templateID)
	if err != nil {
		return nil, err
	}
	points := loaded.Points
	collectMethod := loaded.CollectMethod
	mergedParams := loaded.Params
	interval := loaded.Interval

	// 根据 collectMethod 创建具体协议采集器。
	factory := collectoradapter.GetCollectorFactory()
	collectorInstance := factory.CreateCollector(collectMethod, mergedParams)

	// 从模板测点提取运行时元信息。
	pointMetas := make([]runtimeinfra.PointMeta, 0, len(points))
	for _, pt := range points {
		pointMetas = append(pointMetas, runtimeinfra.PointMeta{
			PointID:  pt.PointID,
			Access:   pt.Access,
			Name:     pt.Name,
			Unit:     pt.Unit,
			DataType: pt.DataType,
		})
	}

	// 注册设备运行时，返回该设备的 PointRuntime 集合。
	dataMgr := runtimeinfra.GetDataManager()
	deviceRuntime := dataMgr.RegisterDevice(deviceID, templateID, loaded.DeviceName, pointMetas)

	// 创建可取消的运行上下文，停止采集时会触发退出。
	// context.WithCancel 返回两个值：
	// - ctx：传给下游，告诉它“请随时关注是否被取消”；
	// - cancel：由上游持有，在停止时主动发出取消信号。
	ctx, cancel := context.WithCancel(context.Background())

	// 组装设备采集器对象。
	dc := &DeviceCollector{
		DeviceID:       deviceID,
		TemplateID:     templateID,
		CollectMethod:  collectMethod,
		Points:         points,
		Params:         mergedParams,
		Interval:       interval,
		Collector:      collectorInstance,
		deviceRuntime:  deviceRuntime,
		ctx:            ctx,
		cancel:         cancel,
		consecFailures: 0,
		maxRetries:     loaded.MaxRetries,
		online:         true, // 初始状态假定在线，失败后再切离线
		pointEnabled:   loaded.PointEnabled,
	}

	return dc, nil
}

// StopCollector 停止设备采集器。
func (m *CollectorManager) StopCollector(deviceID int) {
	_ = m.StopCollectorWithTimeout(deviceID, StopCollectorTimeout)
}

// StopCollectorWithTimeout 在指定超时时间内尝试停止采集器。
// 停止流程分三步：
// 1. 从 manager 中摘掉 collector，避免新请求再拿到它；
// 2. cancel 通知采集循环退出，并等待 goroutine 收尾；
// 3. 清理告警引擎和运行态缓存。
func (m *CollectorManager) StopCollectorWithTimeout(deviceID int, timeout time.Duration) error {
	m.mu.Lock()
	dc, exists := m.collectors[deviceID]
	if !exists {
		m.mu.Unlock()
		return nil
	}
	delete(m.collectors, deviceID)
	dc.cancel()
	m.mu.Unlock()

	// channel 在这里被当作“一次性完成信号”使用。
	// 关闭 done 就表示“wg.Wait() 已经结束”。
	done := make(chan struct{})
	go func() {
		dc.wg.Wait()
		close(done)
	}()

	var waitErr error
	select {
	case <-done:
	case <-time.After(timeout):
		waitErr = fmt.Errorf("等待设备 %d 采集线程退出超时（%s）", deviceID, timeout)
		logger.Warnf("[StopCollector] %v", waitErr)
	}

	stopDone := make(chan struct{})
	go func() {
		dc.Collector.Stop()
		close(stopDone)
	}()
	select {
	case <-stopDone:
	case <-time.After(2 * time.Second):
		logger.Warnf("[StopCollector] 设备 %d Collector.Stop 执行超时，继续清理", deviceID)
	}

	runtimeinfra.GetDataManager().UnregisterDevice(deviceID)
	return waitErr
}

// UpdatePointEnabled 热更新设备采集器中的测点启用状态，无需重启。
func (m *CollectorManager) UpdatePointEnabled(deviceID, pointID int, enabled bool) {
	m.mu.RLock()
	dc, exists := m.collectors[deviceID]
	m.mu.RUnlock()
	if !exists || dc == nil {
		return
	}

	dc.pointCfgMu.Lock()
	if dc.pointEnabled == nil {
		dc.pointEnabled = make(map[int]bool)
	}
	dc.pointEnabled[pointID] = enabled
	dc.pointCfgMu.Unlock()
}

// GetPointRuntime 获取指定测点的运行时数据。
func (m *CollectorManager) GetPointRuntime(deviceID, pointID int) (*runtimeinfra.PointRuntime, bool) {
	return runtimeinfra.GetDataManager().GetPointRuntime(deviceID, pointID)
}

// GetPointValue 获取指定测点的当前值和时间戳。
func (m *CollectorManager) GetPointValue(deviceID, pointID int) (valueobj.CollectedValue, time.Time, bool) {
	pointRuntime, ok := m.GetPointRuntime(deviceID, pointID)
	if !ok || pointRuntime == nil {
		return valueobj.CollectedValue{}, time.Time{}, false
	}
	return pointRuntime.GetValue(), pointRuntime.GetLastUpdatedAt(), true
}

// GetPointValues 批量获取一个设备下多个测点的当前值。
func (m *CollectorManager) GetPointValues(deviceID int, pointIDs []int) (map[int]PointValueSnapshot, bool) {
	if !m.IsCollectorRunning(deviceID) {
		return map[int]PointValueSnapshot{}, false
	}

	result := make(map[int]PointValueSnapshot, len(pointIDs))
	for _, pointID := range pointIDs {
		value, ts, ok := m.GetPointValue(deviceID, pointID)
		if !ok {
			continue
		}
		result[pointID] = PointValueSnapshot{Value: value, Timestamp: ts}
	}
	return result, true
}

// getPointTemplate 从设备采集器的缓存测点列表中查找指定测点的模板配置。
func (m *CollectorManager) getPointTemplate(deviceID, pointID int) (templatebiz.DevicePoint, error) {
	m.mu.RLock()
	dc, ok := m.collectors[deviceID]
	m.mu.RUnlock()
	if !ok || dc == nil {
		return templatebiz.DevicePoint{}, fmt.Errorf("设备 %d 无运行的采集器", deviceID)
	}
	for _, pt := range dc.Points {
		if pt.PointID == pointID {
			return pt, nil
		}
	}
	return templatebiz.DevicePoint{}, fmt.Errorf("测点 %d 在设备 %d 中未找到", pointID, deviceID)
}

// reverseToRaw 将逆变换后的数值安全转换为原始类型。
// 这是收集链路中 convertRawToTargetValue 的逆操作。
func reverseToRaw(rawNum float64, originalType string) (interface{}, error) {
	normType := valueobj.NormalizeRawType(originalType)
	switch normType {
	case valueobj.RawTypeINT16:
		if math.Trunc(rawNum) != rawNum || rawNum < math.MinInt16 || rawNum > math.MaxInt16 {
			return nil, fmt.Errorf("数值超出 INT16 范围: %v", rawNum)
		}
		return int16(rawNum), nil
	case valueobj.RawTypeUINT16:
		if math.Trunc(rawNum) != rawNum || rawNum < 0 || rawNum > math.MaxUint16 {
			return nil, fmt.Errorf("数值超出 UINT16 范围: %v", rawNum)
		}
		return uint16(rawNum), nil
	case valueobj.RawTypeINT32:
		if math.Trunc(rawNum) != rawNum || rawNum < math.MinInt32 || rawNum > math.MaxInt32 {
			return nil, fmt.Errorf("数值超出 INT32 范围: %v", rawNum)
		}
		return int32(rawNum), nil
	case valueobj.RawTypeUINT32:
		if math.Trunc(rawNum) != rawNum || rawNum < 0 || rawNum > math.MaxUint32 {
			return nil, fmt.Errorf("数值超出 UINT32 范围: %v", rawNum)
		}
		return uint32(rawNum), nil
	case valueobj.RawTypeINT64:
		if math.Trunc(rawNum) != rawNum || rawNum < math.MinInt64 || rawNum > math.MaxInt64 {
			return nil, fmt.Errorf("数值超出 INT64 范围: %v", rawNum)
		}
		return int64(rawNum), nil
	case valueobj.RawTypeUINT64:
		if math.Trunc(rawNum) != rawNum || rawNum < 0 || rawNum > math.MaxUint64 {
			return nil, fmt.Errorf("数值超出 UINT64 范围: %v", rawNum)
		}
		return uint64(rawNum), nil
	case valueobj.RawTypeFLOAT32:
		return float32(rawNum), nil
	case valueobj.RawTypeFLOAT64:
		return rawNum, nil
	case valueobj.RawTypeBOOL:
		return rawNum != 0, nil
	case valueobj.RawTypeSTRING:
		return nil, fmt.Errorf("STRING 原始值由调用方直接提供")
	}
	// NormalizeRawType 对 SNMP 原始类型（INTEGER/COUNTER32/GAUGE32/TIMETICKS/OPAQUEFLOAT 等）
	// 返回空字符串，在此兜底处理，使写回驱动侧时 Go 类型与 PDU 类型匹配。
	switch strings.ToUpper(strings.TrimSpace(originalType)) {
	case "INTEGER", "INTEGER32":
		if math.Trunc(rawNum) != rawNum || rawNum < math.MinInt64 || rawNum > math.MaxInt64 {
			return nil, fmt.Errorf("数值超出 INTEGER 范围: %v", rawNum)
		}
		return int64(rawNum), nil
	case "UNSIGNED32", "UINTEGER32", "GAUGE", "GAUGE32", "COUNTER", "COUNTER32", "TIMETICKS":
		if math.Trunc(rawNum) != rawNum || rawNum < 0 || rawNum > math.MaxUint32 {
			return nil, fmt.Errorf("数值超出 UNSIGNED32 范围: %v", rawNum)
		}
		return uint32(rawNum), nil
	case "COUNTER64":
		if math.Trunc(rawNum) != rawNum || rawNum < 0 || rawNum > math.MaxUint64 {
			return nil, fmt.Errorf("数值超出 COUNTER64 范围: %v", rawNum)
		}
		return uint64(rawNum), nil
	case "OPAQUEFLOAT":
		return float32(rawNum), nil
	case "OPAQUEDOUBLE":
		return float64(rawNum), nil
	}
	if math.Trunc(rawNum) != rawNum || rawNum < math.MinInt64 || rawNum > math.MaxInt64 {
		return nil, fmt.Errorf("数值超出默认整数范围: %v", rawNum)
	}
	return int64(rawNum), nil
}

// WritePoint 将北向下发的最终值经逆变换后写入真实设备。
// 流程：校验 Access → 逆变换 → driver.Write → 成功回写缓存
func (m *CollectorManager) WritePoint(deviceID, pointID int, finalValue valueobj.CollectedValue) error {
	m.mu.RLock()
	dc, ok := m.collectors[deviceID]
	m.mu.RUnlock()
	if !ok || dc.Collector == nil {
		return fmt.Errorf("设备 %d 无运行的采集器", deviceID)
	}

	point, err := m.getPointTemplate(deviceID, pointID)
	if err != nil {
		return err
	}

	// Access 门禁：仅 w/rw 允许写入
	if point.Access != "w" && point.Access != "rw" {
		return fmt.Errorf("测点 %d 不可写 (Access=%s)", pointID, point.Access)
	}

	// 含自定义表达式的测点不可逆，拒绝写入
	if point.Expression != "" {
		return fmt.Errorf("含自定义表达式的测点 %d 暂不支持写回", point.PointID)
	}

	var rawValue interface{}
	if finalValue.Kind == valueobj.KindNumber {
		// 逆变换公式：rawValue = (finalValue - offset - correctionValue) / coefficient
		//
		// 注意：正向采集时 applyPrecision 做的精度舍入不可逆，写回值会与原始采集值
		// 有精度偏差。此处仅做系数/偏移的代数逆运算，不（也无法）还原精度舍入前的值。
		coefficient := point.Coefficient
		if coefficient == 0 {
			coefficient = 1.0
		}
		rawNum := (finalValue.Number - point.Offset - point.CorrectionValue) / coefficient
		if math.IsNaN(rawNum) || math.IsInf(rawNum, 0) {
			return fmt.Errorf("逆变换结果无效（NaN 或 Inf），可能由除零或无效系数导致: %v", rawNum)
		}
		rawValue, err = reverseToRaw(rawNum, point.OriginalType)
		if err != nil {
			return err
		}
	} else if finalValue.Kind == valueobj.KindBool {
		rawValue = finalValue.Bool
	} else if finalValue.Kind == valueobj.KindText {
		if valueobj.NormalizeRawType(point.OriginalType) != valueobj.RawTypeSTRING {
			return fmt.Errorf("数据类型 %s 不支持写入", finalValue.Kind)
		}
		rawValue = finalValue.Text
	} else {
		return fmt.Errorf("数据类型 %s 不支持写入", finalValue.Kind)
	}

	// 本地 DIDO 的具体通道和方向保存在设备参数中，写驱动前先解析到测点快照。
	if dc.CollectMethod == "gpio" {
		direction := strings.ToLower(strings.TrimSpace(dc.Params["io_direction"]))
		if direction == "di" {
			return fmt.Errorf("通道 %s 是数字输入，不支持写入", dc.Params["io_channel"])
		}
		channel := strings.ToUpper(strings.TrimSpace(dc.Params["io_channel"]))
		if channel != "" && point.LocalIO != nil && (strings.EqualFold(point.LocalIO.Channel, "AUTO") || strings.TrimSpace(point.LocalIO.Channel) == "") {
			spec := *point.LocalIO
			spec.Channel = channel
			point.LocalIO = &spec
		}
	}

	// driver.Write
	if err := dc.Collector.Write(context.Background(), deviceID, point, rawValue); err != nil {
		return fmt.Errorf("写设备失败: %v", err)
	}

	// 写入成功 → 回写缓存（保留最终值）
	if pr, ok := runtimeinfra.GetDataManager().GetPointRuntime(deviceID, pointID); ok {
		pr.Update(finalValue)
	}

	return nil
}

// run 是采集器主循环。
// context 用于停止信号，ticker 用于定时触发采集。
func (c *DeviceCollector) run() {
	c.wg.Add(1)
	defer c.wg.Done()

	ticker := time.NewTicker(time.Duration(c.Interval) * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			// 收到取消信号，退出循环
			return
		case <-ticker.C:
			// 定时器触发，执行采集（单轮 panic 兜底见 collectSafely）
			c.collectSafely()
		}
	}
}

// collectSafely 包住一轮采集，兜住驱动层的 panic。
//
// 采集 goroutine 里没有 recover 时，一台设备的 panic（例如对已关闭的空连接解引用）
// 会直接带走整个进程；配置还在库里，systemd 的 Restart=always 只会把它变成永久崩溃循环。
// 单设备故障的正确影响范围是“该设备本轮采集失败”，不能升级成整机宕机。
func (c *DeviceCollector) collectSafely() {
	defer func() {
		if r := recover(); r != nil {
			now := time.Now()
			logger.Errorf("[DeviceCollector] 设备 %d 采集 panic 已拦截，本轮按采集失败处理: %v\n%s",
				c.DeviceID, r, debug.Stack())
			c.handleCollectFailure(now, now.Format("2006-01-02 15:04:05"))
		}
	}()

	c.collect()
}

// collect 执行一次完整采集。
// 顺序是：采集 -> 值处理 -> runtime 更新 -> history 写入 -> 告警检测 -> 在线状态管理。
func (c *DeviceCollector) collect() {
	now := time.Now()
	nowStr := now.Format("2006-01-02 15:04:05")

	// 使用预加载的参数和测点调用采集器，不再回查数据库。
	data, err := c.Collector.Collect(c.ctx, c.DeviceID, c.Points, c.Params)
	if err != nil {
		if c.ctx.Err() != nil {
			return
		}
		logger.Errorf("[DeviceCollector] 设备 %d 采集数据失败：%v\n", c.DeviceID, err)
		c.handleCollectFailure(now, nowStr)
		return
	}

	// 采集成功，处理在线状态。
	c.handleCollectSuccess(now, nowStr)

	historyWriter := historymodule.GetWriteService()
	// 逐点处理采集结果，在线状态点单独处理。
	for _, point := range c.Points {
		if point.PointID == 0 {
			continue // ID为0的测点由在线状态管理逻辑处理
		}
		if !c.isPointEnabled(point.PointID) {
			continue
		}

		value, exists := data[point.PointID]
		if !exists {
			continue
		}

		// 数值点统一应用系数、偏移、修正值和精度。
		if value.Kind == valueobj.KindNumber {
			coefficient := point.Coefficient
			if coefficient == 0 {
				coefficient = 1.0
			}
			finalValue := value.Number * coefficient
			finalValue += point.Offset
			finalValue += point.CorrectionValue

			precision := point.Precision
			if precision < 0 {
				precision = 2
			}
			finalValue = applyPrecision(finalValue, precision)
			value.Number = finalValue
		}

		// 更新 runtime 当前值和历史缓冲。
		c.deviceRuntime.SetPointValue(point.PointID, value)

		// 写入历史缓存，后续由 history 层批量刷库。
		historyWriter.AddPoint(c.DeviceID, point.PointID, value, point.Unit, point.DataType, nowStr)
	}
}

func (c *DeviceCollector) isPointEnabled(pointID int) bool {
	c.pointCfgMu.RLock()
	defer c.pointCfgMu.RUnlock()
	if c.pointEnabled == nil {
		return true
	}
	enabled, ok := c.pointEnabled[pointID]
	if !ok {
		return true
	}
	return enabled
}

// handleCollectSuccess 处理采集成功：重置失败计数并恢复在线状态。
func (c *DeviceCollector) handleCollectSuccess(now time.Time, nowStr string) {
	// 重置连续失败计数
	c.consecFailures = 0

	// 首次采集成功或从离线恢复时，写入在线状态点并驱动离线告警恢复
	if !c.onlineReported || !c.online {
		c.online = true
		c.updateOnlineStatus(1.0, now, nowStr)
		logger.Infof("[DeviceCollector] 设备 %d 恢复在线\n", c.DeviceID)
	}
}

// handleCollectFailure 处理采集失败：增加失败计数，判断是否离线。
func (c *DeviceCollector) handleCollectFailure(now time.Time, nowStr string) {
	c.consecFailures++

	// 判断是否达到最大重试次数
	if c.consecFailures > c.maxRetries && (c.online || !c.onlineReported) {
		c.online = false
		c.updateOnlineStatus(0.0, now, nowStr)
		logger.Warnf("[DeviceCollector] 设备 %d 离线（连续失败 %d 次，超过阈值 %d）\n",
			c.DeviceID, c.consecFailures, c.maxRetries)
	}
}

// updateOnlineStatus 更新内置的在线状态测点（PointID = 0）。
func (c *DeviceCollector) updateOnlineStatus(value float64, now time.Time, nowStr string) {
	const onlinePointID = 0
	c.onlineReported = true

	// 更新 PointRuntime
	c.deviceRuntime.SetPointValue(onlinePointID, valueobj.NewNumberValue(value))

	// 写入历史数据
	historymodule.GetWriteService().AddPoint(c.DeviceID, onlinePointID, valueobj.NewNumberValue(value), "", valueobj.DataTypeINT64, nowStr)
}

// applyPrecision 应用精度处理，保留指定小数位数。
func applyPrecision(value float64, precision int) float64 {
	if precision <= 0 {
		return math.Round(value)
	}
	if precision > 10 {
		precision = 10
	}
	factor := math.Pow10(precision)
	return math.Round(value*factor) / factor
}

// resolveCollectorInterval 保留在 runtime 包中用于测试与兼容调用。
func resolveCollectorInterval(defaultInterval int, params map[string]string) int {
	return collectorinfra.ResolveCollectorInterval(defaultInterval, params)
}
