package infrastructure

import (
	"sync"
	"time"

	"data-collection-system/internal/valueobj"
)

const HistoryBufferSize = 300

var pointNow = time.Now

// SetPointNowFunc 允许测试替换时间源，方便构造跨秒/跨分钟缓存场景。
func SetPointNowFunc(fn func() time.Time) {
	if fn == nil {
		pointNow = time.Now
		return
	}
	pointNow = fn
}

// bucketKey 计算时间桶的 key，单位为秒，时间桶的概念在这里是为了支持按秒/分钟粒度存储历史数据，方便后续查询时快速定位对应时间段的数据。
// 例如，如果当前时间是 2024-06-01 12:34:56，那么秒级桶 key 就是 2024-06-01 12:34:56 对应的 Unix 时间戳（秒），分钟级桶 key 就是 2024-06-01 12:34:00 对应的 Unix 时间戳（秒）。
// 在这里是怎么用的？在 PointRuntime 的 Update 方法里，我们会根据当前时间计算秒级和分钟级的桶 key，
// 然后把数值写入对应的环形缓冲区。这个函数的好处是把时间桶的计算逻辑集中在一起，避免散落在代码里多处重复实现。
type numericSeriesSlot struct {
	Bucket int64
	Value  float64
	Valid  bool
}

// RecentHistoryPoint 表示一条最近时序数据。
// 运行态缓存和历史库查询最终都尽量收敛成这个视图，避免上层分支处理两套结构。
type RecentHistoryPoint struct {
	Timestamp string
	Value     interface{}
	ValueKind string
	Unit      string
	DataType  string
}

// PointRuntime 测点运行时数据。
// 维护单个测点的当前值、300 点环形缓冲区与最后更新时间，
// 并带着测点元信息，支持按 Access 属性筛选测点/控制点。
//
// 这里的“运行时”可以理解成 C 里常说的“内存态结构体”：
// 它不一定和数据库表一一对应，而是为了高频读写专门设计的数据布局。
type PointRuntime struct {
	Value         valueobj.CollectedValue
	SecondHistory [HistoryBufferSize]numericSeriesSlot
	SecondCursor  int
	SecondCount   int
	MinuteHistory [HistoryBufferSize]numericSeriesSlot
	MinuteCursor  int
	MinuteCount   int
	LastUpdatedAt time.Time

	// 元信息字段（初始化时从模板缓存获取）
	Access   string // 读写属性: r/w/rw
	Name     string // 测点名称
	Unit     string // 单位
	DataType string // 数据类型

	// mu 保护单个测点内部字段。
	// 采用“每个点自己带锁”的细粒度方案，避免全局一把大锁导致所有设备互相阻塞。
	mu sync.RWMutex
}

// Update 更新测点值并写入时间桶缓存。
// 数值点和布尔点进入秒级/分钟级缓存，文本点只更新当前值。
func (p *PointRuntime) Update(val valueobj.CollectedValue) {
	now := pointNow()
	p.mu.Lock()
	defer p.mu.Unlock()

	p.Value = val
	if val.Kind == valueobj.KindNumber || val.Kind == valueobj.KindBool {
		num := val.Number
		if val.Kind == valueobj.KindBool {
			if val.Bool {
				num = 1
			} else {
				num = 0
			}
		}
		secondBucket := now.Unix()
		minuteBucket := now.Unix() / 60
		p.recordSeriesLocked(&p.SecondHistory, &p.SecondCursor, &p.SecondCount, secondBucket, num)
		p.recordSeriesLocked(&p.MinuteHistory, &p.MinuteCursor, &p.MinuteCount, minuteBucket, num)
	}
	p.LastUpdatedAt = now
}

func (p *PointRuntime) recordSeriesLocked(buf *[HistoryBufferSize]numericSeriesSlot, cursor *int, count *int, bucket int64, value float64) {
	if count == nil || cursor == nil {
		return
	}
	if *count > 0 {
		lastIdx := (*cursor - 1 + HistoryBufferSize) % HistoryBufferSize
		last := buf[lastIdx]
		if last.Valid && last.Bucket == bucket {
			buf[lastIdx].Value = value
			return
		}
	}

	buf[*cursor] = numericSeriesSlot{
		Bucket: bucket,
		Value:  value,
		Valid:  true,
	}
	*cursor = (*cursor + 1) % HistoryBufferSize
	if *count < HistoryBufferSize {
		*count = *count + 1
	}
}

// GetValue 获取当前值
func (p *PointRuntime) GetValue() valueobj.CollectedValue {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.Value
}

func (p *PointRuntime) GetNumericValue() (float64, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.Value.Kind != valueobj.KindNumber {
		return 0, false
	}
	return p.Value.Number, true
}

// GetLastUpdatedAt 获取最后更新时间
func (p *PointRuntime) GetLastUpdatedAt() time.Time {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.LastUpdatedAt
}

// GetHistory 获取最近 N 条历史数据。
// 这里用固定长度环形缓冲区实现，cursor 向前推进，旧值自动覆盖。
// 对比 C 里的循环队列：本质上也是同一个思路，只是用数组配合取模运算实现。
func (p *PointRuntime) GetHistory(count int) []float64 {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if count > p.SecondCount {
		count = p.SecondCount
	}

	if count <= 0 {
		return nil
	}
	result := make([]float64, count)
	for i := 0; i < count; i++ {
		idx := (p.SecondCursor - count + i + HistoryBufferSize) % HistoryBufferSize
		result[i] = p.SecondHistory[idx].Value
	}
	return result
}

// GetRecentSeries 按秒或分钟返回最近时序数据。
// granularity 仅接受 second / minute / sec / min / s / m。
func (p *PointRuntime) GetRecentSeries(granularity string, count int) []RecentHistoryPoint {
	p.mu.RLock()
	defer p.mu.RUnlock()

	switch normalizeGranularity(granularity) {
	case "minute":
		return p.extractSeriesLocked(p.MinuteHistory[:], p.MinuteCursor, p.MinuteCount, count, time.Minute)
	default:
		return p.extractSeriesLocked(p.SecondHistory[:], p.SecondCursor, p.SecondCount, count, time.Second)
	}
}

// GetRecentSeriesRange 按时间范围返回最近时序数据。
// 仅适用于运行态缓存窗口内的数据；如果查询范围早于缓存最早时间，则返回 false。
func (p *PointRuntime) GetRecentSeriesRange(granularity string, startTime, endTime time.Time) ([]RecentHistoryPoint, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	bucketSize, samples, cursor, count := p.selectSeriesBucketLocked(granularity)
	if bucketSize <= 0 || count <= 0 {
		return nil, false
	}

	startBucket := startTime.Unix() / int64(bucketSize/time.Second)
	endBucket := endTime.Unix() / int64(bucketSize/time.Second)
	if endBucket < startBucket {
		return nil, false
	}

	oldestBucket, ok := oldestSeriesBucket(samples, cursor, count)
	if !ok {
		return nil, false
	}
	// 如果查询范围早于缓存最早时间，说明无法满足查询需求，返回 false。
	if startBucket < oldestBucket {
		return nil, false
	}

	points := make([]RecentHistoryPoint, 0, count)
	for i := 0; i < count; i++ {
		idx := (cursor - count + i + HistoryBufferSize) % HistoryBufferSize
		sample := samples[idx]
		if !sample.Valid {
			continue
		}
		if sample.Bucket < startBucket || sample.Bucket > endBucket {
			continue
		}
		ts := time.Unix(sample.Bucket*int64(bucketSize/time.Second), 0).Format("2006-01-02 15:04:05")
		points = append(points, RecentHistoryPoint{
			Timestamp: ts,
			Value:     sampleValueForDisplay(sample.Value, p.DataType),
			ValueKind: seriesValueKind(p.DataType),
			Unit:      p.Unit,
			DataType:  p.DataType,
		})
	}
	return points, true
}

func (p *PointRuntime) extractSeriesLocked(samples []numericSeriesSlot, cursor, count, limit int, bucketSize time.Duration) []RecentHistoryPoint {
	if limit <= 0 {
		return nil
	}
	if count <= 0 {
		return nil
	}
	if limit > count {
		limit = count
	}

	start := count - limit
	if start < 0 {
		start = 0
	}
	points := make([]RecentHistoryPoint, 0, limit)
	for i := start; i < count; i++ {
		idx := (cursor - count + i + HistoryBufferSize) % HistoryBufferSize
		sample := samples[idx]
		if !sample.Valid {
			continue
		}
		ts := time.Unix(sample.Bucket*int64(bucketSize/time.Second), 0).Format("2006-01-02 15:04:05")
		points = append(points, RecentHistoryPoint{
			Timestamp: ts,
			Value:     sampleValueForDisplay(sample.Value, p.DataType),
			ValueKind: seriesValueKind(p.DataType),
			Unit:      p.Unit,
			DataType:  p.DataType,
		})
	}
	return points
}

func (p *PointRuntime) selectSeriesBucketLocked(granularity string) (time.Duration, []numericSeriesSlot, int, int) {
	switch normalizeGranularity(granularity) {
	case "minute":
		return time.Minute, p.MinuteHistory[:], p.MinuteCursor, p.MinuteCount
	default:
		return time.Second, p.SecondHistory[:], p.SecondCursor, p.SecondCount
	}
}

// oldestSeriesBucket 获取当前环形缓冲区里最旧的时间桶 key。
// 这里的逻辑是：如果 count > 0，说明缓冲区里至少有一条数据；
// 那么最旧的数据就是 cursor 前面 count 个位置的那个桶（取模计算）
// 如果那个桶有效且与当前 bucket 不同，说明它就是最旧的桶；否则说明当前 bucket 是最旧的。
func oldestSeriesBucket(samples []numericSeriesSlot, cursor, count int) (int64, bool) {
	if count <= 0 {
		return 0, false
	}
	idx := (cursor - count + HistoryBufferSize) % HistoryBufferSize
	sample := samples[idx]
	if !sample.Valid {
		return 0, false
	}
	return sample.Bucket, true
}

func normalizeGranularity(granularity string) string {
	switch granularity {
	case "second", "sec", "s":
		return "second"
	case "minute", "min", "m":
		return "minute"
	default:
		return "second"
	}
}

func seriesValueKind(dataType string) string {
	if valueobj.NormalizeDataType(dataType) == valueobj.DataTypeBOOL {
		return "bool"
	}
	return "number"
}

func sampleValueForDisplay(v float64, dataType string) interface{} {
	if valueobj.NormalizeDataType(dataType) == valueobj.DataTypeBOOL {
		return v != 0
	}
	return v
}

// DeviceRuntime 设备运行时数据。
// 它是“单设备视角”的容器，DataManager 再往上一层，负责管理所有设备。
type DeviceRuntime struct {
	DeviceID   int
	TemplateID int
	DeviceName string                // 设备名称
	Points     map[int]*PointRuntime // pointID → PointRuntime
	mu         sync.RWMutex
}

// PointMeta 是启动阶段用的“静态描述”。
// 它和 PointRuntime 的关系可以理解为：
// - PointMeta 负责说明点长什么样；
// - PointRuntime 负责记录点当前跑成什么样。
type PointMeta struct {
	PointID  int
	Access   string
	Name     string
	Unit     string
	DataType string
}

// NewDeviceRuntime 创建设备运行时实例。
// 这里会提前为每个 point 建好 PointRuntime，后续采集循环只负责更新值，不再频繁建对象。
func NewDeviceRuntime(deviceID, templateID int, deviceName string, pointMetas []PointMeta) *DeviceRuntime {
	dr := &DeviceRuntime{
		DeviceID:   deviceID,
		TemplateID: templateID,
		DeviceName: deviceName,
		Points:     make(map[int]*PointRuntime, len(pointMetas)),
	}
	for _, meta := range pointMetas {
		dr.Points[meta.PointID] = &PointRuntime{
			Access:   meta.Access,
			Name:     meta.Name,
			Unit:     meta.Unit,
			DataType: meta.DataType,
		}
	}
	return dr
}

// SetPointValue 更新指定测点的值。
// 注意这里先拿设备级读锁，再进入点级锁；锁分层后，单点更新不会阻塞整个设备对象的读操作。
func (d *DeviceRuntime) SetPointValue(pointID int, value valueobj.CollectedValue) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if rt, ok := d.Points[pointID]; ok {
		rt.Update(value)
	}
}

// GetPointValue 获取指定测点的当前值，同时返回时间戳。
// Go 支持函数返回多个值，这在错误处理和查询接口里非常常见。
// 这里直接返回 value / timestamp / found，比额外传输出参数更自然。
func (d *DeviceRuntime) GetPointValue(pointID int) (valueobj.CollectedValue, time.Time, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if rt, ok := d.Points[pointID]; ok {
		return rt.GetValue(), rt.GetLastUpdatedAt(), true
	}
	return valueobj.CollectedValue{}, time.Time{}, false
}

// GetNumericPointValue 只返回数值点的当前值。
func (d *DeviceRuntime) GetNumericPointValue(pointID int) (float64, time.Time, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if rt, ok := d.Points[pointID]; ok {
		if v, okNum := rt.GetNumericValue(); okNum {
			return v, rt.GetLastUpdatedAt(), true
		}
	}
	return 0, time.Time{}, false
}

// GetPointRuntime 获取指定测点的 PointRuntime 引用
func (d *DeviceRuntime) GetPointRuntime(pointID int) (*PointRuntime, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	rt, ok := d.Points[pointID]
	return rt, ok
}

// GetAllPointIDs 获取所有测点 ID 列表
func (d *DeviceRuntime) GetAllPointIDs() []int {
	d.mu.RLock()
	defer d.mu.RUnlock()

	ids := make([]int, 0, len(d.Points))
	for id := range d.Points {
		ids = append(ids, id)
	}
	return ids
}

// GetReadPoints 获取所有测点（Access = r 或 rw）。
// 这里返回的是 []*PointRuntime，即“指针切片”。
// 指针语义的好处是避免拷贝整个结构体，也能让调用方继续访问最新的运行态数据。
func (d *DeviceRuntime) GetReadPoints() []*PointRuntime {
	d.mu.RLock()
	defer d.mu.RUnlock()

	result := make([]*PointRuntime, 0)
	for _, rt := range d.Points {
		if rt.Access == "r" || rt.Access == "rw" {
			result = append(result, rt)
		}
	}
	return result
}

// GetControlPoints 获取所有控制点（Access = w 或 rw）
// 用于前端控制点列表展示
func (d *DeviceRuntime) GetControlPoints() []*PointRuntime {
	d.mu.RLock()
	defer d.mu.RUnlock()

	result := make([]*PointRuntime, 0)
	for _, rt := range d.Points {
		if rt.Access == "w" || rt.Access == "rw" {
			result = append(result, rt)
		}
	}
	return result
}

// GetAllPoints 获取所有测点（不区分类型）
func (d *DeviceRuntime) GetAllPoints() []*PointRuntime {
	d.mu.RLock()
	defer d.mu.RUnlock()

	result := make([]*PointRuntime, 0, len(d.Points))
	for _, rt := range d.Points {
		result = append(result, rt)
	}
	return result
}

// PointInfo 是 API 层返回的测点视图。
type PointInfo struct {
	PointID   int         `json:"point_id"`
	Name      string      `json:"name"`
	Value     interface{} `json:"value"`
	ValueKind string      `json:"value_kind"`
	Unit      string      `json:"unit"`
	DataType  string      `json:"data_type"`
	Access    string      `json:"access"`
	Timestamp string      `json:"timestamp"`
	Status    string      `json:"status"`
}

// ToPointInfo 将 PointRuntime 转换为 PointInfo（用于 API 返回）。
// 这体现了“运行态内部结构”和“接口返回结构”分离：
// 内部结构偏重并发与性能，HTTP 返回结构偏重稳定和易序列化。
func (p *PointRuntime) ToPointInfo() PointInfo {
	p.mu.RLock()
	defer p.mu.RUnlock()

	timestamp := "-"
	if !p.LastUpdatedAt.IsZero() {
		timestamp = p.LastUpdatedAt.Format("2006-01-02 15:04:05")
	}

	return PointInfo{
		PointID:   0, // 需要外部设置
		Name:      p.Name,
		Value:     p.Value.AsAny(),
		ValueKind: string(p.Value.Kind),
		Unit:      p.Unit,
		DataType:  p.DataType,
		Access:    p.Access,
		Timestamp: timestamp,
		Status:    "正常",
	}
}
