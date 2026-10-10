package infrastructure

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"data-collection-system/internal/database"
	"data-collection-system/internal/valueobj"
)

func testWriter() *HourlyHistoryWriter {
	w := NewHourlyHistoryWriter(func() time.Time { return time.Date(2026, 10, 8, 9, 53, 0, 0, time.Local) })
	return w
}

// Add 后 latest 存最新值与所属小时桶；同小时再次 Add 覆盖为新值。
func TestAddKeepsLatestValueWithHourBucket(t *testing.T) {
	w := testWriter()
	w.Add(1, 2, valueobj.CollectedValue{Kind: valueobj.KindNumber, Number: 10.5}, "℃", "float", "2026-10-08 09:10:00")
	w.Add(1, 2, valueobj.CollectedValue{Kind: valueobj.KindNumber, Number: 11.5}, "℃", "float", "2026-10-08 09:41:00")

	v := w.latest[pointKey{1, 2}]
	if v.value.Number != 11.5 {
		t.Fatalf("latest = %v, 期望 11.5", v.value.Number)
	}
	if v.hourBucket != "2026-10-08 09:00:00" {
		t.Fatalf("hourBucket = %q", v.hourBucket)
	}
}

// 数值型：当日 min/max 随刷新更新，并写入待落盘快照；未刷新不入快照。
func TestAddTracksDailyExtremumAndDirty(t *testing.T) {
	w := testWriter()
	w.Add(1, 2, valueobj.CollectedValue{Kind: valueobj.KindNumber, Number: 10}, "", "float", "2026-10-08 09:10:00")
	w.Add(1, 2, valueobj.CollectedValue{Kind: valueobj.KindNumber, Number: 12}, "", "float", "2026-10-08 09:11:00")
	w.Add(1, 2, valueobj.CollectedValue{Kind: valueobj.KindNumber, Number: 9}, "", "float", "2026-10-08 09:12:00")

	d := w.daily[pointKey{1, 2}]
	if d.date != "2026-10-08" || d.min != 9 || d.max != 12 {
		t.Fatalf("daily = %+v", d)
	}
	snap, ok := w.dirty[dailyKey{1, 2, "2026-10-08"}]
	if !ok || snap.min != 9 || snap.max != 12 {
		t.Fatalf("极值刷新后应写入快照: %+v ok=%v", snap, ok)
	}
	// 中间值不刷新极值：清掉 dirty 后再 Add 一个中间值
	w.dirty = map[dailyKey]dailyExtremum{}
	w.Add(1, 2, valueobj.CollectedValue{Kind: valueobj.KindNumber, Number: 10}, "", "float", "2026-10-08 09:13:00")
	if _, ok := w.dirty[dailyKey{1, 2, "2026-10-08"}]; ok {
		t.Fatal("中间值不应写入快照")
	}
}

// 文本型：不参与极值、不进 daily。
func TestAddTextValueSkipsExtremum(t *testing.T) {
	w := testWriter()
	w.Add(1, 3, valueobj.CollectedValue{Kind: valueobj.KindText, Text: "on"}, "", "string", "2026-10-08 09:10:00")
	if _, ok := w.daily[pointKey{1, 3}]; ok {
		t.Fatal("文本型不应有极值状态")
	}
}

// 跨天：date 变化时极值重置为新一天的起点。
func TestAddAcrossDayResetsExtremum(t *testing.T) {
	w := testWriter()
	w.Add(1, 2, valueobj.CollectedValue{Kind: valueobj.KindNumber, Number: 99}, "", "float", "2026-10-08 23:59:00")
	w.Add(1, 2, valueobj.CollectedValue{Kind: valueobj.KindNumber, Number: 5}, "", "float", "2026-10-09 00:01:00")
	d := w.daily[pointKey{1, 2}]
	if d.date != "2026-10-09" || d.min != 5 || d.max != 5 {
		t.Fatalf("跨天应重置极值: %+v", d)
	}
}

// 23:59 刷新的前一日极值在跨天后仍以待落盘快照保留——spec 4.2 跨天落盘保证。
func TestAddAcrossDayKeepsPreviousDaySnapshot(t *testing.T) {
	w := testWriter()
	w.Add(1, 2, valueobj.CollectedValue{Kind: valueobj.KindNumber, Number: 99}, "℃", "float", "2026-10-08 23:59:00")
	w.Add(1, 2, valueobj.CollectedValue{Kind: valueobj.KindNumber, Number: 5}, "℃", "float", "2026-10-09 00:01:00")

	if snap := w.dirty[dailyKey{1, 2, "2026-10-08"}]; snap.max != 99 {
		t.Fatalf("前一日快照应保留 23:59 的刷新: %+v", snap)
	}
	if snap := w.dirty[dailyKey{1, 2, "2026-10-09"}]; snap.min != 5 || snap.max != 5 {
		t.Fatalf("新一日应独立建快照: %+v", snap)
	}
}

// Add 递增版本号，供落盘侧判定丢更新。
func TestAddBumpsVersion(t *testing.T) {
	w := testWriter()
	w.Add(1, 2, valueobj.CollectedValue{Kind: valueobj.KindNumber, Number: 1}, "", "float", "2026-10-08 09:10:00")
	v1 := w.latest[pointKey{1, 2}].version
	w.Add(1, 2, valueobj.CollectedValue{Kind: valueobj.KindNumber, Number: 2}, "", "float", "2026-10-08 09:11:00")
	v2 := w.latest[pointKey{1, 2}].version
	if v2 <= v1 {
		t.Fatalf("version 应递增: %d -> %d", v1, v2)
	}
}

// 槽位推导纯函数：窗口外 ok=false；:50 起返回当前槽位序号（0-9）。
func TestSlotOffsetOf(t *testing.T) {
	cases := []struct {
		minute     int
		wantOffset int
		wantOK     bool
	}{
		{49, 0, false}, // 窗口外
		{50, 0, true},
		{53, 3, true},
		{59, 9, true},
	}
	for _, c := range cases {
		at := time.Date(2026, 10, 8, 9, c.minute, 0, 0, time.Local)
		offset, ok := slotOffsetOf(at, 10)
		if offset != c.wantOffset || ok != c.wantOK {
			t.Fatalf("分 %d → (%d,%v), 期望 (%d,%v)", c.minute, offset, ok, c.wantOffset, c.wantOK)
		}
	}
}

// 已落盘且无新 Add 的项不再收集（稀疏写）。
func TestCollectAllSkipsFlushed(t *testing.T) {
	w := NewHourlyHistoryWriter(nil)
	w.Add(13, 1, valueobj.CollectedValue{Kind: valueobj.KindNumber, Number: 1}, "", "float", "2026-10-08 09:10:00")
	items := w.collectAll(13)
	if len(items) != 1 {
		t.Fatalf("应收集 1 项, got %d", len(items))
	}
	w.markFlushed(items)
	if got := w.collectAll(13); len(got) != 0 {
		t.Fatalf("已落盘项不应重复收集, got %d", len(got))
	}
	w.Add(13, 1, valueobj.CollectedValue{Kind: valueobj.KindNumber, Number: 2}, "", "float", "2026-10-08 09:11:00")
	if got := w.collectAll(13); len(got) != 1 {
		t.Fatalf("新值应重新收集, got %d", len(got))
	}
}

// 并发丢更新防护：落盘期间 Add 更新了同点位，markFlushed 必须不标记（版本不等）。
func TestMarkFlushedDoesNotSkipConcurrentUpdate(t *testing.T) {
	w := NewHourlyHistoryWriter(nil)
	w.Add(13, 1, valueobj.CollectedValue{Kind: valueobj.KindNumber, Number: 1}, "", "float", "2026-10-08 09:10:00")
	items := w.collectAll(13)                                                                                       // 收集（版本 v1）
	w.Add(13, 1, valueobj.CollectedValue{Kind: valueobj.KindNumber, Number: 2}, "", "float", "2026-10-08 09:11:00") // 落盘在途时更新 → v2
	w.markFlushed(items)                                                                                            // 携带的是 v1，不得把 v2 标为已落
	// 断言用 valueNum（deviceDataRecord 的数值字段），语义等价于「收集到的最新值是 2」。
	if got := w.collectAll(13); len(got) != 1 || got[0].valueNum == nil || *got[0].valueNum != 2 {
		t.Fatalf("并发更新不得丢失: %+v", got)
	}
}

// 非法入参在 Start 入口归一化，不得让调度 goroutine panic（除零 / NewTicker(0)）。
func TestStartNormalizesInvalidArgs(t *testing.T) {
	w := NewHourlyHistoryWriter(nil)
	w.Start(0, 0) // 归一化为 1 分钟间隔与 10 分钟窗口
	w.Stop()
	w.Start(0, 99)
	w.Stop()
}

// 极值 dirty 由 Start 的周期协程落盘（spec 4.2 事件驱动落盘的周期化）。
func TestStartFlushesExtremumPeriodically(t *testing.T) {
	db := openTestRuntimeDB(t)
	mustExec(t, db, dailyExtremumDDLForTest)
	old := database.HistoryDB
	database.HistoryDB = db
	t.Cleanup(func() { database.HistoryDB = old })

	w := NewHourlyHistoryWriter(nil)
	w.extremumInterval = 20 * time.Millisecond
	w.Add(1, 1, valueobj.CollectedValue{Kind: valueobj.KindNumber, Number: 7}, "℃", "float", "2026-10-08 09:10:00")
	w.Start(time.Hour, 10) // 落盘调度间隔取长，避免干扰
	defer w.Stop()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		var cnt int
		if err := db.QueryRow(`SELECT COUNT(*) FROM device_daily_extremum`).Scan(&cnt); err == nil && cnt == 1 {
			return // 已落盘
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("极值未在期限内落盘")
}

// deviceDataDDLForTest 与 history/bootstrap.go 的 device_data 建表语句一致（含 CHECK），
// 另加 UPSERT 冲突目标唯一索引（生产由 ensureDeviceDataUniqueIndex 保证）。
const deviceDataDDLForTest = `
	CREATE TABLE IF NOT EXISTS device_data (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		device_id INTEGER NOT NULL,
		point_id INTEGER NOT NULL,
		value_type INTEGER NOT NULL DEFAULT 0,
		value_num REAL,
		value_text TEXT,
		unit VARCHAR(20),
		data_type VARCHAR(50),
		collected_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		CHECK (
			(value_type = 0 AND value_num IS NOT NULL AND value_text IS NULL) OR
			(value_type = 1 AND value_num IS NULL AND value_text IS NOT NULL)
		)
	);
	CREATE UNIQUE INDEX idx_data_device_point_time
		ON device_data(device_id, point_id, collected_at);
`

// dailyExtremumDDLForTest 与 history/bootstrap.go 的 device_daily_extremum 建表语句一致
// （含 UPSERT 冲突目标 UNIQUE(device_id, point_id, date)）。
const dailyExtremumDDLForTest = `
	CREATE TABLE IF NOT EXISTS device_daily_extremum (
		device_id INTEGER NOT NULL,
		point_id  INTEGER NOT NULL,
		date      TEXT NOT NULL,
		min_value REAL,
		max_value REAL,
		unit      VARCHAR(20),
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(device_id, point_id, date)
	);
`

// openTestRuntimeDB 打开 t.TempDir() 下的文件库（:memory: 多连接不可靠），并限制单连接。
func openTestRuntimeDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "runtime.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	return db
}

func mustExec(t *testing.T, db *sql.DB, sqlStr string) {
	t.Helper()
	if _, err := db.Exec(sqlStr); err != nil {
		t.Fatalf("exec %q: %v", sqlStr, err)
	}
}

// 落盘幂等：同键两次 UPSERT 覆盖为最新值，行数不变。
func TestUpsertDeviceDataIsIdempotent(t *testing.T) {
	db := openTestRuntimeDB(t)
	mustExec(t, db, deviceDataDDLForTest)

	num1, num2 := 1.0, 2.0
	recs := []deviceDataRecord{{deviceID: 1, pointID: 1, valueType: 0, valueNum: &num1,
		unit: "℃", dataType: "float", collectedAt: "2026-10-08 09:00:00", version: 1}}
	if err := upsertDeviceData(context.Background(), db, recs); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	recs[0].valueNum = &num2
	if err := upsertDeviceData(context.Background(), db, recs); err != nil {
		t.Fatalf("upsert 覆盖: %v", err)
	}
	var cnt int
	var got float64
	if err := db.QueryRow(`SELECT COUNT(*) FROM device_data`).Scan(&cnt); err != nil || cnt != 1 {
		t.Fatalf("行数 = %d, err = %v", cnt, err)
	}
	if err := db.QueryRow(`SELECT value_num FROM device_data`).Scan(&got); err != nil || got != 2 {
		t.Fatalf("value_num = %v, err = %v", got, err)
	}
}

// 分块：250 行拆两条语句，全部写入。
func TestUpsertDeviceDataChunks(t *testing.T) {
	db := openTestRuntimeDB(t)
	mustExec(t, db, deviceDataDDLForTest)
	recs := make([]deviceDataRecord, 0, 250)
	for i := 0; i < 250; i++ {
		num := float64(i)
		recs = append(recs, deviceDataRecord{deviceID: 1, pointID: i, valueType: 0, valueNum: &num,
			collectedAt: "2026-10-08 09:00:00", version: 1})
	}
	if err := upsertDeviceData(context.Background(), db, recs); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	var cnt int
	_ = db.QueryRow(`SELECT COUNT(*) FROM device_data`).Scan(&cnt)
	if cnt != 250 {
		t.Fatalf("行数 = %d, 期望 250", cnt)
	}
}

// 极值落盘：dirty 项批量 UPSERT；落盘后 dirty 清空。
func TestFlushDailyExtremumUpsertsDirty(t *testing.T) {
	db := openTestRuntimeDB(t)
	mustExec(t, db, dailyExtremumDDLForTest)
	w := NewHourlyHistoryWriter(func() time.Time { return time.Date(2026, 10, 8, 9, 0, 0, 0, time.Local) })
	w.Add(1, 1, valueobj.CollectedValue{Kind: valueobj.KindNumber, Number: 12}, "℃", "float", "2026-10-08 09:10:00")
	w.Add(1, 1, valueobj.CollectedValue{Kind: valueobj.KindNumber, Number: 7}, "℃", "float", "2026-10-08 09:11:00")
	if err := w.flushDailyExtremum(context.Background(), db); err != nil {
		t.Fatalf("flush: %v", err)
	}
	var minV, maxV float64
	if err := db.QueryRow(`SELECT min_value, max_value FROM device_daily_extremum WHERE device_id=1 AND point_id=1 AND date='2026-10-08'`).Scan(&minV, &maxV); err != nil {
		t.Fatalf("query: %v", err)
	}
	if minV != 7 || maxV != 12 {
		t.Fatalf("极值 = %v/%v", minV, maxV)
	}
	if len(w.dirty) != 0 {
		t.Fatal("落盘后 dirty 应清空")
	}
}

// 合并语义：库中已有 10/20，新落 5/15 → 5/20（跨重启安全）。
func TestFlushDailyExtremumMergesWithDB(t *testing.T) {
	db := openTestRuntimeDB(t)
	mustExec(t, db, dailyExtremumDDLForTest)
	mustExec(t, db, `INSERT INTO device_daily_extremum (device_id, point_id, date, min_value, max_value, unit) VALUES (1,1,'2026-10-08',10,20,'℃')`)
	w := NewHourlyHistoryWriter(nil)
	w.Add(1, 1, valueobj.CollectedValue{Kind: valueobj.KindNumber, Number: 5}, "℃", "float", "2026-10-08 09:10:00")
	w.Add(1, 1, valueobj.CollectedValue{Kind: valueobj.KindNumber, Number: 15}, "℃", "float", "2026-10-08 09:11:00")
	if err := w.flushDailyExtremum(context.Background(), db); err != nil {
		t.Fatalf("flush: %v", err)
	}
	var minV, maxV float64
	_ = db.QueryRow(`SELECT min_value, max_value FROM device_daily_extremum`).Scan(&minV, &maxV)
	if minV != 5 || maxV != 20 {
		t.Fatalf("合并结果 = %v/%v, 期望 5/20", minV, maxV)
	}
}

// 重启继承：LoadDailyExtremum 批量装载当天行。
func TestLoadDailyExtremumRestoresToday(t *testing.T) {
	db := openTestRuntimeDB(t)
	mustExec(t, db, dailyExtremumDDLForTest)
	mustExec(t, db, `INSERT INTO device_daily_extremum (device_id, point_id, date, min_value, max_value, unit) VALUES (2,3,'2026-10-08',3,9,'%')`)
	mustExec(t, db, `INSERT INTO device_daily_extremum (device_id, point_id, date, min_value, max_value, unit) VALUES (2,4,'2026-10-07',1,1,'%')`)
	w := NewHourlyHistoryWriter(func() time.Time { return time.Date(2026, 10, 8, 10, 0, 0, 0, time.Local) })
	if err := w.LoadDailyExtremum(context.Background(), db); err != nil {
		t.Fatalf("load: %v", err)
	}
	if d := w.daily[pointKey{2, 3}]; d.min != 3 || d.max != 9 || d.date != "2026-10-08" || d.unit != "%" {
		t.Fatalf("应继承当天行: %+v", d)
	}
	if _, ok := w.daily[pointKey{2, 4}]; ok {
		t.Fatal("昨天的行不应装载")
	}
}

// 失败放回：合并语义——Add 期间写入的更新快照不被旧快照覆盖（取更宽极值）。
func TestRestoreDirtyMergesInsteadOfOverwrite(t *testing.T) {
	w := NewHourlyHistoryWriter(nil)
	w.mu.Lock()
	w.dirty[dailyKey{1, 1, "2026-10-08"}] = dailyExtremum{date: "2026-10-08", min: 3, max: 10, ok: true}
	w.mu.Unlock()
	// 旧批次（收集时不大于新快照）放回：min 不得被覆盖回 5
	w.restoreDirty([]dailyFlushItem{{key: dailyKey{1, 1, "2026-10-08"}, d: dailyExtremum{date: "2026-10-08", min: 5, max: 10, ok: true}}})
	if d := w.dirty[dailyKey{1, 1, "2026-10-08"}]; d.min != 3 {
		t.Fatalf("更新的快照被覆盖: %+v", d)
	}
	// 反之：放回的批次更宽时合并保留
	w.restoreDirty([]dailyFlushItem{{key: dailyKey{1, 1, "2026-10-08"}, d: dailyExtremum{date: "2026-10-08", min: 1, max: 20, ok: true}}})
	if d := w.dirty[dailyKey{1, 1, "2026-10-08"}]; d.min != 1 || d.max != 20 {
		t.Fatalf("合并应取更宽: %+v", d)
	}
}

// 落盘失败：快照放回 dirty，不静默丢弃。
func TestFlushDailyExtremumRestoresOnFailure(t *testing.T) {
	db := openTestRuntimeDB(t)
	mustExec(t, db, dailyExtremumDDLForTest)
	w := NewHourlyHistoryWriter(nil)
	w.Add(1, 1, valueobj.CollectedValue{Kind: valueobj.KindNumber, Number: 10}, "", "float", "2026-10-08 09:10:00")
	if err := db.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if err := w.flushDailyExtremum(context.Background(), db); err == nil {
		t.Fatal("关库后 flush 应失败")
	}
	if len(w.dirty) != 1 {
		t.Fatalf("失败后快照应放回, dirty=%d", len(w.dirty))
	}
}

// 关机 Flush 必须同时落 dirty 极值（不再只落 latest）。
func TestFlushLandsDailyExtremum(t *testing.T) {
	db := openTestRuntimeDB(t)
	mustExec(t, db, dailyExtremumDDLForTest)
	mustExec(t, db, deviceDataDDLForTest) // Flush 也会落 latest，两张表都要建
	old := database.HistoryDB
	database.HistoryDB = db
	t.Cleanup(func() { database.HistoryDB = old })

	w := NewHourlyHistoryWriter(nil)
	w.Add(1, 1, valueobj.CollectedValue{Kind: valueobj.KindNumber, Number: 7}, "℃", "float", "2026-10-08 09:10:00")
	if err := w.Flush(context.Background()); err != nil {
		t.Fatalf("flush: %v", err)
	}
	var cnt int
	if err := db.QueryRow(`SELECT COUNT(*) FROM device_daily_extremum`).Scan(&cnt); err != nil || cnt != 1 {
		t.Fatalf("极值应随 Flush 落盘: cnt=%d err=%v", cnt, err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM device_data`).Scan(&cnt); err != nil || cnt != 1 {
		t.Fatalf("最新值应随 Flush 落盘: cnt=%d err=%v", cnt, err)
	}
	if len(w.dirty) != 0 {
		t.Fatal("Flush 后 dirty 应清空")
	}
}

// flushSlot 按 deviceID % window 选设备，且用 <= 补落：槽位已过的设备一并落盘，槽位未到的不落。
func TestFlushSlotSelectsDevicesByMod(t *testing.T) {
	db := openTestRuntimeDB(t)
	mustExec(t, db, deviceDataDDLForTest)
	old := database.HistoryDB
	database.HistoryDB = db
	t.Cleanup(func() { database.HistoryDB = old })

	// 水位时钟 :53：窗口 10 分钟下槽位序号 3（:50 起算）。
	w := NewHourlyHistoryWriter(func() time.Time { return time.Date(2026, 10, 8, 9, 53, 0, 0, time.Local) })
	w.Add(13, 1, valueobj.CollectedValue{Kind: valueobj.KindNumber, Number: 1}, "", "float", "2026-10-08 09:10:00") // 槽 3
	w.Add(21, 1, valueobj.CollectedValue{Kind: valueobj.KindNumber, Number: 2}, "", "float", "2026-10-08 09:10:00") // 槽 1
	w.flushSlot(context.Background(), 3, 10)

	for _, id := range []int{13, 21} {
		var cnt int
		if err := db.QueryRow(`SELECT COUNT(*) FROM device_data WHERE device_id = ?`, id).Scan(&cnt); err != nil || cnt != 1 {
			t.Fatalf("槽位 ≤ 3 的设备 %d 应落盘: cnt=%d err=%v", id, cnt, err)
		}
	}
	// 槽位 5 的设备不在本轮范围。
	w.Add(25, 1, valueobj.CollectedValue{Kind: valueobj.KindNumber, Number: 3}, "", "float", "2026-10-08 09:10:00")
	w.flushSlot(context.Background(), 3, 10)
	var cnt int
	if err := db.QueryRow(`SELECT COUNT(*) FROM device_data WHERE device_id = 25`).Scan(&cnt); err != nil || cnt != 0 {
		t.Fatalf("槽位未到的设备不应落盘: cnt=%d err=%v", cnt, err)
	}
}

// FlushDevice 立即落盘该设备全部未落值并回标（collectAll 清空），新值重新进入待落集合。
func TestFlushDeviceWritesAndClears(t *testing.T) {
	db := openTestRuntimeDB(t)
	mustExec(t, db, deviceDataDDLForTest)
	old := database.HistoryDB
	database.HistoryDB = db
	t.Cleanup(func() { database.HistoryDB = old })

	w := NewHourlyHistoryWriter(nil)
	w.Add(13, 1, valueobj.CollectedValue{Kind: valueobj.KindNumber, Number: 1}, "℃", "float", "2026-10-08 09:10:00")
	w.Add(13, 2, valueobj.CollectedValue{Kind: valueobj.KindText, Text: "on"}, "", "string", "2026-10-08 09:10:00")
	if err := w.FlushDevice(context.Background(), 13); err != nil {
		t.Fatalf("flush device: %v", err)
	}
	var cnt int
	if err := db.QueryRow(`SELECT COUNT(*) FROM device_data WHERE device_id = 13`).Scan(&cnt); err != nil || cnt != 2 {
		t.Fatalf("全部未落值应落盘: cnt=%d err=%v", cnt, err)
	}
	if got := w.collectAll(13); len(got) != 0 {
		t.Fatalf("落盘后应无可收集项, got %d", len(got))
	}
	w.Add(13, 1, valueobj.CollectedValue{Kind: valueobj.KindNumber, Number: 2}, "℃", "float", "2026-10-08 09:11:00")
	if got := w.collectAll(13); len(got) != 1 {
		t.Fatalf("新值应重新进入待落集合, got %d", len(got))
	}
}
