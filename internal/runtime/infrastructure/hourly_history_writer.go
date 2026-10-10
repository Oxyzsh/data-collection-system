package infrastructure

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"

	"data-collection-system/internal/database"
	"data-collection-system/internal/shared/logger"
	"data-collection-system/internal/valueobj"
)

// defaultExtremumFlushInterval 极值 dirty 的默认落盘周期（spec 4.2「秒级」）。
const defaultExtremumFlushInterval = 5 * time.Second

// HourlyHistoryWriter 统一历史写入器：内存汇总每点位最新值与当日极值，
// 由落盘调度在每小时 :50–:59 按设备取模错峰批量写入 device_data（spec 4.1/4.2）。
// 采集线程只调用 Add（纯内存），不持数据库写锁。
type HourlyHistoryWriter struct {
	mu     sync.Mutex
	now    func() time.Time
	latest map[pointKey]latestValue
	daily  map[pointKey]dailyExtremum
	// dirty 是待落盘极值快照（值拷贝而非键集合）：日期随键携带，
	// 跨天时前一日未落盘的极值仍保留在此，保证 23:59 的刷新在 00:00 后写入前一日行（spec 4.2）。
	dirty map[dailyKey]dailyExtremum

	// 生命周期字段独立于 mu：调度协程持 lifecycleMu 启动/停止，
	// 落盘过程只短暂持有 mu 做收集/回标，两把锁不嵌套（避免持值锁等数据库写）。
	lifecycleMu      sync.Mutex
	started          bool
	stopCh           chan struct{}
	wg               sync.WaitGroup
	extremumInterval time.Duration // 极值落盘周期，测试可改小；Start 内防御性归一化
}

type pointKey struct{ deviceID, pointID int }

// dailyKey 极值落盘键：日期必须随键携带（跨天快照语义，见 dirty 字段注释）。
type dailyKey struct {
	deviceID, pointID int
	date              string
}

type latestValue struct {
	value          valueobj.CollectedValue
	unit           string
	dataType       string
	hourBucket     string // 值所属小时起点，格式 2006-01-02 15:04:05
	version        int64  // 每次 Add 递增
	flushedVersion int64  // 落盘成功时对齐的版本；两者相等表示无未落更新
}

type dailyExtremum struct {
	date     string // YYYY-MM-DD（本地日）
	min, max float64
	unit     string // 随最新值更新，供极值页展示
	ok       bool   // 当日已有数值
}

func NewHourlyHistoryWriter(now func() time.Time) *HourlyHistoryWriter {
	if now == nil {
		now = time.Now
	}
	return &HourlyHistoryWriter{
		now:              now,
		latest:           map[pointKey]latestValue{},
		daily:            map[pointKey]dailyExtremum{},
		dirty:            map[dailyKey]dailyExtremum{},
		extremumInterval: defaultExtremumFlushInterval,
	}
}

var (
	hourlyHistoryWriterOnce sync.Once
	hourlyHistoryWriterInst *HourlyHistoryWriter
)

// GetHourlyHistoryWriter 返回进程级单例（sync.Once + 懒加载，装配于 services 惯例之外，
// 与旧 GetMinuteHistoryWriter 保持一致——采集链路直接取用）。
func GetHourlyHistoryWriter() *HourlyHistoryWriter {
	hourlyHistoryWriterOnce.Do(func() {
		hourlyHistoryWriterInst = NewHourlyHistoryWriter(nil)
	})
	return hourlyHistoryWriterInst
}

// Add 汇总一个采集值：更新最新值（附小时桶）并维护当日极值（仅数值型）。
func (w *HourlyHistoryWriter) Add(deviceID, pointID int, value valueobj.CollectedValue, unit, dataType, timestamp string) {
	recordTime := parseHistoryTime(timestamp)
	bucket := recordTime.Truncate(time.Hour)
	key := pointKey{deviceID, pointID}

	w.mu.Lock()
	defer w.mu.Unlock()

	cur := w.latest[key]
	cur.value = value
	cur.unit = unit
	cur.dataType = dataType
	cur.hourBucket = bucket.Format("2006-01-02 15:04:05")
	cur.version++
	w.latest[key] = cur

	if value.Kind == valueobj.KindText {
		return // 文本型不参与极值（spec 4.2）
	}
	num := value.Number
	if value.Kind == valueobj.KindBool {
		if value.Bool {
			num = 1
		} else {
			num = 0
		}
	}
	date := recordTime.Format("2006-01-02")
	d := w.daily[key]
	if !d.ok || d.date != date {
		d = dailyExtremum{date: date, min: num, max: num, unit: unit, ok: true}
	} else if num < d.min || num > d.max {
		if num < d.min {
			d.min = num
		}
		if num > d.max {
			d.max = num
		}
		d.unit = unit
	} else {
		w.daily[key] = d
		return // 中间值：不刷新极值，也不写快照
	}
	w.daily[key] = d
	w.dirty[dailyKey{deviceID, pointID, date}] = d // 快照（值拷贝，跨天安全）
}

// deviceDataRecord 一次待落盘的 device_data 行（含版本号用于回标）。
type deviceDataRecord struct {
	deviceID, pointID int
	valueType         int
	valueNum          *float64
	valueText         *string
	unit, dataType    string
	collectedAt       string // 小时起点
	version           int64
}

// slotOffsetOf 返回 now 所在分钟对应的槽位序号（0 起）；窗口外 ok=false。
// 纯函数：窗口语义只在调度层（Start 的 ticker 循环），收集/落盘函数（collectAll/FlushDevice）不做窗口判断。
func slotOffsetOf(now time.Time, windowMinutes int) (int, bool) {
	if windowMinutes <= 0 || windowMinutes >= 60 {
		windowMinutes = 10
	}
	minute := now.Minute()
	if minute < 60-windowMinutes {
		return 0, false
	}
	return minute - (60 - windowMinutes), true
}

// markFlushed 落盘成功后回标；仅当内存中版本与收集时一致才标记（幂等且防丢更新）。
func (w *HourlyHistoryWriter) markFlushed(items []deviceDataRecord) {
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, it := range items {
		key := pointKey{it.deviceID, it.pointID}
		cur, ok := w.latest[key]
		if ok && cur.version == it.version {
			cur.flushedVersion = it.version
			w.latest[key] = cur
		}
	}
}

// FlushDevice 立即落盘指定设备（设备停止/关机路径使用，不受窗口限制）。
func (w *HourlyHistoryWriter) FlushDevice(ctx context.Context, deviceID int) error {
	items := w.collectAll(deviceID)
	if len(items) == 0 {
		return nil
	}
	if err := upsertDeviceData(ctx, database.HistoryDB, items); err != nil {
		return err // 失败不回标：值留在内存，下一窗口重试
	}
	w.markFlushed(items)
	return nil
}

// Flush 落盘全部未落项（关机钩子使用）：先极值快照、再最新值；
// 任一失败记日志继续，返回首个错误。
func (w *HourlyHistoryWriter) Flush(ctx context.Context) error {
	var firstErr error
	// 先落极值：优雅停机时最后 ≤5s 内的极值刷新不落会永久丢失（重启装载的是不含峰值的行）。
	// 失败不直接返回，最新值仍应尝试落盘。
	if err := w.flushDailyExtremum(ctx, database.HistoryDB); err != nil {
		logger.Warnf("[History] 关机极值落盘存在失败项: %v\n", err)
		firstErr = err
	}
	seen := map[int]struct{}{}
	w.mu.Lock()
	for key := range w.latest {
		seen[key.deviceID] = struct{}{}
	}
	w.mu.Unlock()
	for id := range seen {
		if err := w.FlushDevice(ctx, id); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// collectAll 不做窗口判断的收集（Flush/FlushDevice 路径）。
func (w *HourlyHistoryWriter) collectAll(deviceID int) []deviceDataRecord {
	w.mu.Lock()
	defer w.mu.Unlock()
	out := make([]deviceDataRecord, 0)
	for key, v := range w.latest {
		if key.deviceID != deviceID || v.version == v.flushedVersion {
			continue
		}
		out = append(out, buildRecord(key, v))
	}
	return out
}

// buildRecord 把内存值转换为待落盘行（collectAll 共用）。
func buildRecord(key pointKey, v latestValue) deviceDataRecord {
	rec := deviceDataRecord{
		deviceID: key.deviceID, pointID: key.pointID,
		unit: v.unit, dataType: v.dataType, collectedAt: v.hourBucket, version: v.version,
	}
	if v.value.Kind == valueobj.KindText {
		rec.valueType = 1
		text := v.value.Text
		rec.valueText = &text
	} else {
		rec.valueType = 0
		num := v.value.Number
		if v.value.Kind == valueobj.KindBool {
			if v.value.Bool {
				num = 1
			} else {
				num = 0
			}
		}
		rec.valueNum = &num
	}
	return rec
}

const (
	upsertChunkSize     = 200 // 单条语句行数上限，防绑定参数超限（spec 4.1）
	upsertDeviceDataSQL = `INSERT INTO device_data
		(device_id, point_id, value_type, value_num, value_text, unit, data_type, collected_at)
		VALUES %s
		ON CONFLICT(device_id, point_id, collected_at) DO UPDATE SET
			value_type=excluded.value_type, value_num=excluded.value_num,
			value_text=excluded.value_text, unit=excluded.unit, data_type=excluded.data_type`
)

// upsertDeviceData 分批写 device_data：单条语句最多 upsertChunkSize 行，防绑定参数超限。
// 唯一索引由 bootstrap 保证（history/bootstrap.go 缺索引且库内已有数据时拒绝启动），
// 落到这里时索引必然就绪——无回退路径（spec 4.4 裁定）。
func upsertDeviceData(ctx context.Context, db *sql.DB, records []deviceDataRecord) error {
	if len(records) == 0 || db == nil {
		return nil
	}
	for start := 0; start < len(records); start += upsertChunkSize {
		end := start + upsertChunkSize
		if end > len(records) {
			end = len(records)
		}
		chunk := records[start:end]
		holders := make([]string, 0, len(chunk))
		args := make([]any, 0, len(chunk)*8)
		for _, r := range chunk {
			holders = append(holders, "(?,?,?,?,?,?,?,?)")
			args = append(args, r.deviceID, r.pointID, r.valueType, r.valueNum, r.valueText, r.unit, r.dataType, r.collectedAt)
		}
		sqlStr := fmt.Sprintf(upsertDeviceDataSQL, strings.Join(holders, ","))
		if _, err := db.ExecContext(ctx, sqlStr, args...); err != nil {
			return err
		}
	}
	return nil
}

const extremumUpsertSQL = `INSERT INTO device_daily_extremum
	(device_id, point_id, date, min_value, max_value, unit)
	VALUES (?,?,?,?,?,?)
	ON CONFLICT(device_id, point_id, date) DO UPDATE SET
		min_value = MIN(device_daily_extremum.min_value, excluded.min_value),
		max_value = MAX(device_daily_extremum.max_value, excluded.max_value),
		unit = excluded.unit,
		updated_at = CURRENT_TIMESTAMP`

// dailyFlushItem 一次极值落盘的快照项（收集时拷贝，供失败放回使用）。
type dailyFlushItem struct {
	key dailyKey
	d   dailyExtremum
}

// flushDailyExtremum 将 dirty 快照批量落盘；失败时把快照放回，下一周期重试。
// 快照自带 (deviceID, pointID, date) 与极值，直接 UPSERT——不读 w.daily（跨天安全，spec 4.2）。
func (w *HourlyHistoryWriter) flushDailyExtremum(ctx context.Context, db *sql.DB) error {
	if db == nil {
		return nil
	}
	w.mu.Lock()
	items := make([]dailyFlushItem, 0, len(w.dirty))
	for key, d := range w.dirty {
		if d.ok {
			items = append(items, dailyFlushItem{key, d})
		}
	}
	w.dirty = map[dailyKey]dailyExtremum{}
	w.mu.Unlock()
	if len(items) == 0 {
		return nil
	}

	var firstErr error
	for start := 0; start < len(items); start += upsertChunkSize {
		end := start + upsertChunkSize
		if end > len(items) {
			end = len(items)
		}
		for _, it := range items[start:end] {
			if _, err := db.ExecContext(ctx, extremumUpsertSQL,
				it.key.deviceID, it.key.pointID, it.key.date, it.d.min, it.d.max, it.d.unit); err != nil && firstErr == nil {
				firstErr = err
			}
		}
	}
	if firstErr != nil {
		w.restoreDirty(items)
	}
	return firstErr
}

// restoreDirty 把落盘失败的快照合并回 dirty：已有键取更宽极值。
// 不得直接覆盖——收集与放回之间 Add 可能已用同一累加器写入更新的快照，
// 覆盖会让新观测的极值永不落盘（spec 4.2 重启继承红线）。
func (w *HourlyHistoryWriter) restoreDirty(items []dailyFlushItem) {
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, it := range items {
		cur, exists := w.dirty[it.key]
		if !exists {
			w.dirty[it.key] = it.d
			continue
		}
		if it.d.min < cur.min {
			cur.min = it.d.min
		}
		if it.d.max > cur.max {
			cur.max = it.d.max
		}
		w.dirty[it.key] = cur
	}
}

// LoadDailyExtremum 启动时批量装载当天极值行，重启后继续累加（spec 4.2）。
func (w *HourlyHistoryWriter) LoadDailyExtremum(ctx context.Context, db *sql.DB) error {
	if db == nil {
		return nil
	}
	date := w.now().Format("2006-01-02")
	rows, err := db.QueryContext(ctx,
		`SELECT device_id, point_id, min_value, max_value, COALESCE(unit, '') FROM device_daily_extremum WHERE date = ?`, date)
	if err != nil {
		return err
	}
	defer rows.Close()
	w.mu.Lock()
	defer w.mu.Unlock()
	for rows.Next() {
		var deviceID, pointID int
		var minV, maxV sql.NullFloat64
		var unit string
		if err := rows.Scan(&deviceID, &pointID, &minV, &maxV, &unit); err != nil {
			return err
		}
		if !minV.Valid || !maxV.Valid {
			continue
		}
		w.daily[pointKey{deviceID, pointID}] = dailyExtremum{date: date, min: minV.Float64, max: maxV.Float64, unit: unit, ok: true}
	}
	return rows.Err()
}

// SetExtremumFlushInterval 设置极值 dirty 落盘周期（storage.history.extremum_flush_interval）。
// 须在 Start 之前调用：ticker 在 Start 内创建，启动后再改不生效；<=0 由 Start 归一化回默认。
func (w *HourlyHistoryWriter) SetExtremumFlushInterval(interval time.Duration) {
	w.extremumInterval = interval
}

// Start 启动调度协程：每分钟检查窗口，到达设备槽位即落盘该设备（失败不清标记，后续分钟自然补落）；
// 同协程内按 extremumInterval 周期落盘极值 dirty（spec 4.2 秒级事件落盘，与落盘调度共享 stopCh/wg）。
// 生命周期骨架（started/stopCh/wg/lifecycleMu，与 w.mu 分离避免持有值锁等落盘）
// 与 upload_temp_cleaner.go 一致——结构体需补这四个字段。
func (w *HourlyHistoryWriter) Start(interval time.Duration, windowMinutes int) {
	// 入口归一化，与 slotOffsetOf 同规则：非法间隔会 panic NewTicker、
	// 非法窗口会除零——配置缺键（零值）不得杀死调度 goroutine。
	if interval <= 0 {
		interval = time.Minute
	}
	if windowMinutes <= 0 || windowMinutes >= 60 {
		windowMinutes = 10
	}
	if w.extremumInterval <= 0 {
		w.extremumInterval = defaultExtremumFlushInterval
	}
	w.lifecycleMu.Lock()
	if w.started {
		w.lifecycleMu.Unlock()
		return
	}
	w.stopCh = make(chan struct{})
	stopCh := w.stopCh
	w.started = true
	w.wg.Add(1)
	w.lifecycleMu.Unlock()

	// 打印归一化后的生效参数：启动日志是运维侧确认落盘调度已就位的唯一入口。
	logger.Infof("[History] 落盘调度已启动：interval=%s window=%dmin\n", interval, windowMinutes)

	go func() {
		defer w.wg.Done()
		scheduleTicker := time.NewTicker(interval)
		defer scheduleTicker.Stop()
		extremumTicker := time.NewTicker(w.extremumInterval)
		defer extremumTicker.Stop()
		for {
			select {
			case <-stopCh:
				return
			case <-scheduleTicker.C:
				offset, ok := slotOffsetOf(w.now(), windowMinutes)
				if !ok {
					continue // 窗口外
				}
				w.flushSlot(context.Background(), offset, windowMinutes)
			case <-extremumTicker.C:
				if err := w.flushDailyExtremum(context.Background(), database.HistoryDB); err != nil {
					logger.Warnf("[History] 极值落盘失败（下一周期重试）: %v\n", err)
				}
			}
		}
	}()
}

// flushSlot 落盘「deviceID % windowMinutes <= 当前槽位序号」的全部设备——
// 用 <= 而非 ==：槽位已过但未落成功的设备在后续分钟补落（写失败/进程重启场景）。
func (w *HourlyHistoryWriter) flushSlot(ctx context.Context, slotOffset, windowMinutes int) {
	seen := map[int]struct{}{}
	w.mu.Lock()
	for key := range w.latest {
		if key.deviceID%windowMinutes <= slotOffset {
			seen[key.deviceID] = struct{}{}
		}
	}
	w.mu.Unlock()
	for id := range seen {
		if err := w.FlushDevice(ctx, id); err != nil {
			logger.Warnf("[History] 落盘失败 device=%d（后续分钟重试）: %v\n", id, err)
		}
	}
}

// Stop 停止调度并等待在途落盘收敛（幂等，关机钩子惯例）。
func (w *HourlyHistoryWriter) Stop() {
	w.lifecycleMu.Lock()
	if !w.started {
		w.lifecycleMu.Unlock()
		return
	}
	stopCh := w.stopCh
	w.stopCh = nil
	w.started = false
	w.lifecycleMu.Unlock()
	close(stopCh)
	w.wg.Wait()
}
