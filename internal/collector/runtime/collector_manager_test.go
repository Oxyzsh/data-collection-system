package runtime

import (
	"context"
	"math"
	"sync/atomic"
	"testing"
	"time"

	runtimeinfra "data-collection-system/internal/runtime/infrastructure"
	templatebiz "data-collection-system/internal/template/domain"
	"data-collection-system/internal/valueobj"
)

// TestApplyPrecision 测试动态精度处理
func TestApplyPrecision(t *testing.T) {
	tests := []struct {
		name      string
		value     float64
		precision int
		want      float64
	}{
		{"保留2位小数", 3.14159, 2, 3.14},
		{"保留0位小数", 3.14159, 0, 3.0},
		{"保留4位小数", 3.14159, 4, 3.1416},
		{"保留1位小数", 9.876, 1, 9.9},
		{"负精度默认取整", 9.876, -1, 10.0},
		{"整数值", 42.0, 2, 42.0},
		{"零值", 0.0, 2, 0.0},
		{"小值保留3位", 0.12345, 3, 0.123},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := applyPrecision(tt.value, tt.precision)
			if math.Abs(got-tt.want) > 1e-9 {
				t.Errorf("applyPrecision(%v, %d) = %v, want %v", tt.value, tt.precision, got, tt.want)
			}
		})
	}
}

// TestPointRuntimeRingBuffer 测试 PointRuntime 环形缓冲区滚动
func TestPointRuntimeRingBuffer(t *testing.T) {
	base := time.Date(2026, 4, 18, 12, 0, 0, 0, time.Local)
	var tick int64
	runtimeinfra.SetPointNowFunc(func() time.Time {
		n := atomic.AddInt64(&tick, 1) - 1
		return base.Add(time.Duration(n) * time.Second)
	})
	t.Cleanup(func() {
		runtimeinfra.SetPointNowFunc(nil)
	})

	rt := &runtimeinfra.PointRuntime{}

	// 写入 310 个值（超过缓冲区大小 300）
	for i := 0; i < 310; i++ {
		rt.Update(valueobj.NewNumberValue(float64(i)))
	}

	// 最新值应为 309
	if got := rt.GetValue(); got.Kind != valueobj.KindNumber || got.Number != 309 {
		t.Errorf("GetValue() = %+v, want number 309", got)
	}

	// 获取最近 300 条历史，应为 10..309
	history := rt.GetHistory(300)
	if len(history) != 300 {
		t.Fatalf("GetHistory(300) length = %d, want 300", len(history))
	}
	if history[0] != 10 {
		t.Errorf("history[0] = %v, want 10", history[0])
	}
	if history[299] != 309 {
		t.Errorf("history[299] = %v, want 309", history[299])
	}

	// 获取最近 10 条历史，应为 300..309
	recent := rt.GetHistory(10)
	if len(recent) != 10 {
		t.Fatalf("GetHistory(10) length = %d, want 10", len(recent))
	}
	if recent[0] != 300 {
		t.Errorf("recent[0] = %v, want 300", recent[0])
	}
	if recent[9] != 309 {
		t.Errorf("recent[9] = %v, want 309", recent[9])
	}

	// 请求超过缓冲区大小，应返回 300 条
	all := rt.GetHistory(500)
	if len(all) != 300 {
		t.Errorf("GetHistory(500) length = %d, want 300", len(all))
	}
}

// TestPointRuntimeTimeBuckets 验证秒级/分钟级时间桶只保留最后值。
func TestPointRuntimeTimeBuckets(t *testing.T) {
	times := []time.Time{
		time.Date(2026, 4, 18, 12, 0, 0, 0, time.Local),
		time.Date(2026, 4, 18, 12, 0, 0, 200*1e6, time.Local),
		time.Date(2026, 4, 18, 12, 0, 1, 0, time.Local),
		time.Date(2026, 4, 18, 12, 1, 1, 0, time.Local),
	}
	var idx int64
	runtimeinfra.SetPointNowFunc(func() time.Time {
		i := atomic.AddInt64(&idx, 1) - 1
		if int(i) >= len(times) {
			return times[len(times)-1]
		}
		return times[i]
	})
	t.Cleanup(func() {
		runtimeinfra.SetPointNowFunc(nil)
	})

	rt := &runtimeinfra.PointRuntime{}
	rt.Update(valueobj.NewNumberValue(1))
	rt.Update(valueobj.NewNumberValue(2))
	rt.Update(valueobj.NewNumberValue(3))
	rt.Update(valueobj.NewNumberValue(4))

	secondSeries := rt.GetRecentSeries("second", 10)
	if len(secondSeries) != 3 {
		t.Fatalf("second series len = %d, want 3", len(secondSeries))
	}
	if secondSeries[0].Value != float64(2) || secondSeries[1].Value != float64(3) || secondSeries[2].Value != float64(4) {
		t.Fatalf("second series values = %#v, want [2,3,4]", secondSeries)
	}
	if secondSeries[0].Timestamp != "2026-04-18 12:00:00" {
		t.Fatalf("second series first timestamp = %s, want 2026-04-18 12:00:00", secondSeries[0].Timestamp)
	}

	minuteSeries := rt.GetRecentSeries("minute", 10)
	if len(minuteSeries) != 2 {
		t.Fatalf("minute series len = %d, want 2", len(minuteSeries))
	}
	if minuteSeries[0].Value != float64(3) || minuteSeries[1].Value != float64(4) {
		t.Fatalf("minute series values = %#v, want [3,4]", minuteSeries)
	}
	if minuteSeries[0].Timestamp != "2026-04-18 12:00:00" || minuteSeries[1].Timestamp != "2026-04-18 12:01:00" {
		t.Fatalf("minute series timestamps = %#v, want [12:00,12:01]", minuteSeries)
	}
}

// TestResolveCollectorInterval 测试设备参数中的 interval 覆盖模板默认值
func TestResolveCollectorInterval(t *testing.T) {
	tests := []struct {
		name   string
		def    int
		params map[string]string
		want   int
	}{
		{
			name:   "设备 interval 覆盖默认值",
			def:    1000,
			params: map[string]string{"interval": "3000"},
			want:   3000,
		},
		{
			name:   "无 interval 时使用默认值",
			def:    1000,
			params: map[string]string{"timeout": "500"},
			want:   1000,
		},
		{
			name:   "非法 interval 时使用默认值",
			def:    1000,
			params: map[string]string{"interval": "abc"},
			want:   1000,
		},
		{
			name:   "零值 interval 时使用默认值",
			def:    1000,
			params: map[string]string{"interval": "0"},
			want:   1000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveCollectorInterval(tt.def, tt.params)
			if got != tt.want {
				t.Errorf("resolveCollectorInterval() = %d, want %d", got, tt.want)
			}
		})
	}
}

type writeCaptureCollector struct {
	lastPoint templatebiz.DevicePoint
	lastRaw   interface{}
}

func (c *writeCaptureCollector) Collect(context.Context, int, []templatebiz.DevicePoint, map[string]string) (map[int]valueobj.CollectedValue, error) {
	return nil, nil
}

func (c *writeCaptureCollector) Stop() {}

func (c *writeCaptureCollector) Write(_ context.Context, _ int, point templatebiz.DevicePoint, rawValue interface{}) error {
	c.lastPoint = point
	c.lastRaw = rawValue
	return nil
}

func TestWritePointRejectsOutOfRangeRawValue(t *testing.T) {
	m := &CollectorManager{
		collectors: map[int]*DeviceCollector{
			1: {
				DeviceID: 1,
				Points: []templatebiz.DevicePoint{
					{
						PointID:      7,
						Access:       "rw",
						OriginalType: "UINT16",
						DataType:     valueobj.DataTypeINT64,
					},
				},
				Collector: &writeCaptureCollector{},
			},
		},
	}

	err := m.WritePoint(1, 7, valueobj.NewNumberValue(70000))
	if err == nil {
		t.Fatal("超出 UINT16 范围的写入应返回错误")
	}
}

func TestWritePointAllowsTextForStringPoint(t *testing.T) {
	mock := &writeCaptureCollector{}
	m := &CollectorManager{
		collectors: map[int]*DeviceCollector{
			1: {
				DeviceID: 1,
				Points: []templatebiz.DevicePoint{
					{
						PointID:      8,
						Access:       "rw",
						OriginalType: "STRING",
						DataType:     valueobj.DataTypeSTRING,
					},
				},
				Collector: mock,
			},
		},
	}

	if err := m.WritePoint(1, 8, valueobj.NewTextValue("northbound")); err != nil {
		t.Fatalf("STRING 写入应成功: %v", err)
	}
	if mock.lastRaw != "northbound" {
		t.Fatalf("raw value = %#v, want string", mock.lastRaw)
	}
}

// TestPointRuntimeConcurrent 测试 PointRuntime 并发安全性
func TestPointRuntimeConcurrent(t *testing.T) {
	base := time.Date(2026, 4, 18, 13, 0, 0, 0, time.Local)
	var tick int64
	runtimeinfra.SetPointNowFunc(func() time.Time {
		n := atomic.AddInt64(&tick, 1) - 1
		return base.Add(time.Duration(n) * time.Second)
	})
	t.Cleanup(func() {
		runtimeinfra.SetPointNowFunc(nil)
	})

	rt := &runtimeinfra.PointRuntime{}
	done := make(chan struct{})

	// 启动多个 goroutine 并发写入
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				rt.Update(valueobj.NewNumberValue(float64(id*100 + j)))
			}
			done <- struct{}{}
		}(i)
	}

	// 等待所有写入完成
	for i := 0; i < 10; i++ {
		<-done
	}

	// 验证最终值合法（应为某个 goroutine 写入的最后一个值）
	val := rt.GetValue()
	if val.Kind != valueobj.KindNumber || val.Number < 0 || val.Number > 9999 {
		t.Errorf("GetValue() = %v, out of valid range", val)
	}

	// 验证历史数据长度正确
	history := rt.GetHistory(300)
	if len(history) != 300 {
		t.Errorf("GetHistory(300) length = %d, want 300", len(history))
	}
}

// panicCollector 模拟驱动层 panic（例如对已被置空的连接解引用）。
type panicCollector struct{}

func (panicCollector) Collect(ctx context.Context, deviceID int, points []templatebiz.DevicePoint, params map[string]string) (map[int]valueobj.CollectedValue, error) {
	panic("驱动层 panic")
}

func (panicCollector) Write(ctx context.Context, deviceID int, point templatebiz.DevicePoint, rawValue interface{}) error {
	return nil
}

func (panicCollector) Stop() {}

// 单设备采集 panic 必须被拦在“该设备本轮失败”这一层：没有 recover 时，
// 一台设备的 panic 会带走整个进程，而配置仍在库里，systemd 的 Restart=always
// 会把它变成永久崩溃循环（现场表现：从“读不到数据”升级成“整机宕机”）。
func TestDeviceCollectorRecoversFromPanic(t *testing.T) {
	dc := &DeviceCollector{
		DeviceID:   7,
		Collector:  panicCollector{},
		Interval:   3000,
		maxRetries: 5,
		online:     true,
		ctx:        context.Background(),
	}

	dc.collectSafely()

	if dc.consecFailures != 1 {
		t.Fatalf("panic 后应按一次采集失败计数，consecFailures = %d", dc.consecFailures)
	}
	if !dc.online {
		t.Fatal("失败 1 次未超过 maxRetries，设备不应转离线")
	}
}
