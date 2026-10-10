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

const (
	// NOTE:
	// 当前 PointPersistence/PersistenceQueue 尚未接入主采集链路。
	// 本文件作为预备实现保留，后续启用需单独评审接入路径与线程治理策略。
	batchWriteSize  = 10
	flushInterval   = 5 * time.Second
	maxFlushRetries = 3
	flushRetryDelay = 100 * time.Millisecond
	maxBufferSize   = batchWriteSize * 10

	historyFlushSize     = 50
	historyFlushInterval = 10 * time.Second
)

var (
	dbHealthChecker *DBHealthChecker
	healthCheckOnce sync.Once
)

// DBHealthChecker 数据库健康检查器
type DBHealthChecker struct {
	interval time.Duration
	status   bool
	mu       sync.RWMutex
}

// GetDBHealthChecker 获取数据库健康检查器单例
func GetDBHealthChecker() *DBHealthChecker {
	healthCheckOnce.Do(func() {
		dbHealthChecker = &DBHealthChecker{
			interval: 5 * time.Second,
			status:   true,
		}
		go dbHealthChecker.check()
	})
	return dbHealthChecker
}

// check 定期检查数据库连接状态
func (c *DBHealthChecker) check() {
	ticker := time.NewTicker(c.interval)
	for range ticker.C {
		db := database.RuntimeDB
		if db == nil {
			c.mu.Lock()
			c.status = false
			c.mu.Unlock()
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		err := db.PingContext(ctx)
		cancel()

		c.mu.Lock()
		c.status = (err == nil)
		c.mu.Unlock()

		if err != nil {
			logger.Warnf("[DBHealth] 数据库连接异常: %v", err)
		}
	}
}

// IsHealthy 检查数据库是否健康
func (c *DBHealthChecker) IsHealthy() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.status
}

// CheckDBHealth 检查数据库连接是否健康
func CheckDBHealth(db *sql.DB) bool {
	if db == nil {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return db.PingContext(ctx) == nil
}

// PersistPointData 持久化数据点
type PersistPointData struct {
	PointID   int
	Value     float64
	Unit      string
	DataType  string
	Timestamp string
}

// PointPersistence 设备数据持久化器
// 将采集数据批量写入 device_point_cache 表
type PointPersistence struct {
	deviceID      int
	batchBuffer   []PersistPointData
	lastFlushTime time.Time
	retryCount    int // 连续 flush 失败次数
	mu            sync.Mutex
}

// NewPointPersistence 创建持久化器
func NewPointPersistence(deviceID int) *PointPersistence {
	return &PointPersistence{
		deviceID:      deviceID,
		batchBuffer:   make([]PersistPointData, 0, batchWriteSize),
		lastFlushTime: time.Now(),
	}
}

// Add 添加数据点到批量写入缓冲区，满足条件时自动刷写 DB
func (p *PointPersistence) Add(pointID int, value float64, unit string, dataType string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now().Format("2006-01-02 15:04:05")
	p.batchBuffer = append(p.batchBuffer, PersistPointData{
		PointID:   pointID,
		Value:     value,
		Unit:      unit,
		DataType:  dataType,
		Timestamp: now,
	})

	nowTime := time.Now()
	if len(p.batchBuffer) >= batchWriteSize || nowTime.Sub(p.lastFlushTime) >= flushInterval {
		p.flushBatch()
		p.lastFlushTime = nowTime
	}
}

// flushBatch 批量写入数据到 device_point_cache 表
// 失败时最多重试 maxFlushRetries 次，每次间隔 flushRetryDelay
// 超过重试上限后将数据写入持久化队列，避免数据丢失
func (p *PointPersistence) flushBatch() {
	if len(p.batchBuffer) == 0 {
		return
	}

	db := database.RuntimeDB

	if !CheckDBHealth(db) {
		logger.Warnf("[Persistence] 设备 %d 数据库连接不健康，将数据写入持久化队列", p.deviceID)
		p.saveToPersistenceQueue()
		return
	}

	for attempt := 0; attempt <= maxFlushRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(flushRetryDelay * time.Duration(attempt))
		}

		if !CheckDBHealth(db) {
			logger.Warnf("[Persistence] 设备 %d 第 %d 次重试时数据库连接异常", p.deviceID, attempt+1)
			continue
		}

		if err := writePersistBatch(db, p.deviceID, p.batchBuffer); err != nil {
			logger.Warnf("[Persistence] 设备 %d 写入数据失败 (第%d次): %v", p.deviceID, attempt+1, err)
			continue
		}

		p.batchBuffer = make([]PersistPointData, 0, batchWriteSize)
		p.retryCount = 0
		return
	}

	p.retryCount++
	logger.Warnf("[Persistence] 设备 %d flushBatch 连续失败 %d 次，将 %d 条数据写入持久化队列",
		p.deviceID, p.retryCount, len(p.batchBuffer))

	p.saveToPersistenceQueue()
}

// saveToPersistenceQueue 将数据保存到持久化队列
func (p *PointPersistence) saveToPersistenceQueue() {
	if len(p.batchBuffer) == 0 {
		return
	}

	queue := GetPersistenceQueue()
	if err := queue.AddBatch(p.deviceID, p.batchBuffer); err != nil {
		logger.Errorf("[Persistence] 设备 %d 写入持久化队列失败: %v，丢弃 %d 条数据",
			p.deviceID, err, len(p.batchBuffer))
	} else {
		logger.Warnf("[Persistence] 设备 %d 已将 %d 条数据写入持久化队列",
			p.deviceID, len(p.batchBuffer))
	}

	p.batchBuffer = make([]PersistPointData, 0, batchWriteSize)
}

// Clear 刷新缓冲区并清理 DB 中该设备的历史数据
func (p *PointPersistence) Clear() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.batchBuffer) > 0 {
		p.flushBatch()
	}
	p.batchBuffer = make([]PersistPointData, 0, batchWriteSize)

	clearPersistCacheByDevice(database.RuntimeDB, p.deviceID)
}

// Flush 手动刷新缓冲区
func (p *PointPersistence) Flush() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.flushBatch()
}

func writePersistBatch(db *sql.DB, deviceID int, batch []PersistPointData) error {
	if len(batch) == 0 {
		return nil
	}
	if db == nil {
		return fmt.Errorf("runtime db is nil")
	}

	return insertPersistBatchToHistoryTable(db, deviceID, batch)
}

func insertPersistBatchToHistoryTable(db *sql.DB, deviceID int, batch []PersistPointData) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}

	stmt, err := tx.Prepare("INSERT INTO device_data (device_id, point_id, value_type, value_num, value_text, unit, data_type, collected_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)")
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	defer stmt.Close()

	for _, d := range batch {
		if _, err := stmt.Exec(deviceID, d.PointID, 0, d.Value, nil, d.Unit, d.DataType, d.Timestamp); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func clearPersistCacheByDevice(db *sql.DB, deviceID int) {
	_ = db
	_ = deviceID
}

const (
	persistenceQueueFlushInterval = 5 * time.Second
	persistenceQueueMaxPoints     = maxBufferSize * 100
)

type persistBatchItem struct {
	deviceID int
	batch    []PersistPointData
}

// PersistenceQueue 失败写入兜底队列。
// 作用：
// 1) RuntimeDB 不可用时先缓存数据，避免立刻丢弃；
// 2) 后台定时重放，按入队顺序写回数据库；
// 3) 队列上限保护，防止内存无限增长。
type PersistenceQueue struct {
	mu           sync.Mutex
	pending      []persistBatchItem
	pendingCount int
}

var (
	persistenceQueueOnce     sync.Once
	persistenceQueueInstance *PersistenceQueue
)

// GetPersistenceQueue 获取持久化队列单例，并启动后台重放循环。
func GetPersistenceQueue() *PersistenceQueue {
	persistenceQueueOnce.Do(func() {
		persistenceQueueInstance = &PersistenceQueue{
			pending: make([]persistBatchItem, 0),
		}
		go persistenceQueueInstance.run()
	})
	return persistenceQueueInstance
}

// AddBatch 将一批数据放入队列。
func (q *PersistenceQueue) AddBatch(deviceID int, batch []PersistPointData) error {
	if len(batch) == 0 {
		return nil
	}

	copied := make([]PersistPointData, len(batch))
	copy(copied, batch)

	q.mu.Lock()
	q.pending = append(q.pending, persistBatchItem{
		deviceID: deviceID,
		batch:    copied,
	})
	q.pendingCount += len(copied)
	dropped := q.trimOverflowLocked()
	q.mu.Unlock()

	if dropped > 0 {
		logger.Errorf("[PersistenceQueue] 队列超限，丢弃 %d 条最旧缓存数据（上限=%d）", dropped, persistenceQueueMaxPoints)
	}
	return nil
}

func (q *PersistenceQueue) run() {
	ticker := time.NewTicker(persistenceQueueFlushInterval)
	defer ticker.Stop()

	for range ticker.C {
		q.flushOnce()
	}
}

func (q *PersistenceQueue) flushOnce() {
	q.mu.Lock()
	if len(q.pending) == 0 {
		q.mu.Unlock()
		return
	}

	items := make([]persistBatchItem, len(q.pending))
	copy(items, q.pending)
	q.pending = q.pending[:0]
	q.pendingCount = 0
	q.mu.Unlock()

	db := database.RuntimeDB
	if !CheckDBHealth(db) {
		q.requeueFront(items)
		return
	}

	for i, item := range items {
		if err := writePersistBatch(db, item.deviceID, item.batch); err != nil {
			logger.Warnf("[PersistenceQueue] 写回失败，设备 %d，批次大小 %d: %v", item.deviceID, len(item.batch), err)
			q.requeueFront(items[i:])
			return
		}
	}
}

func (q *PersistenceQueue) requeueFront(items []persistBatchItem) {
	if len(items) == 0 {
		return
	}

	q.mu.Lock()
	q.pending = append(items, q.pending...)
	for _, item := range items {
		q.pendingCount += len(item.batch)
	}
	dropped := q.trimOverflowLocked()
	q.mu.Unlock()

	if dropped > 0 {
		logger.Errorf("[PersistenceQueue] 回放失败后重排队超限，丢弃 %d 条最旧缓存数据（上限=%d）", dropped, persistenceQueueMaxPoints)
	}
}

// trimOverflowLocked 仅在持锁状态调用。
// 超限时优先丢弃最旧批次，保护进程内存。
func (q *PersistenceQueue) trimOverflowLocked() int {
	if q.pendingCount <= persistenceQueueMaxPoints {
		return 0
	}

	dropped := 0
	for q.pendingCount > persistenceQueueMaxPoints && len(q.pending) > 0 {
		first := q.pending[0]
		q.pending = q.pending[1:]
		q.pendingCount -= len(first.batch)
		dropped += len(first.batch)
	}
	if q.pendingCount < 0 {
		q.pendingCount = 0
	}
	return dropped
}

// ---------------------------------------------------------------------------
// HistoryPersistence 历史数据持久化器（device_data 表）
// 将 device_data 的写入从每轮直接 INSERT 改为批量缓冲，减少事务开销
// ---------------------------------------------------------------------------

// HistoryPoint 历史数据点
type HistoryPoint struct {
	DeviceID  int
	PointID   int
	Value     valueobj.CollectedValue
	Unit      string
	DataType  string
	Timestamp string
}

// HistoryPersistence 设备历史数据持久化器
type HistoryPersistence struct {
	deviceID      int
	buffer        []HistoryPoint
	lastFlushTime time.Time
	retryCount    int
	mu            sync.Mutex
}

// NewHistoryPersistence 创建历史数据持久化器
func NewHistoryPersistence(deviceID int) *HistoryPersistence {
	return &HistoryPersistence{
		deviceID:      deviceID,
		buffer:        make([]HistoryPoint, 0, historyFlushSize),
		lastFlushTime: time.Now(),
	}
}

// Add 添加数据点到缓冲区，满足条件时自动刷写 DB
func (h *HistoryPersistence) Add(deviceID, pointID int, value valueobj.CollectedValue, unit, dataType, timestamp string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.buffer = append(h.buffer, HistoryPoint{
		DeviceID:  deviceID,
		PointID:   pointID,
		Value:     value,
		Unit:      unit,
		DataType:  dataType,
		Timestamp: timestamp,
	})

	now := time.Now()
	if len(h.buffer) >= historyFlushSize || now.Sub(h.lastFlushTime) >= historyFlushInterval {
		h.flushBatch()
		h.lastFlushTime = now
	}
}

// flushBatch 批量写入 device_data 表
func (h *HistoryPersistence) flushBatch() {
	if len(h.buffer) == 0 {
		return
	}

	db := database.HistoryDB
	if db == nil {
		return
	}

	for attempt := 0; attempt <= maxFlushRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(flushRetryDelay * time.Duration(attempt))
		}

		if err := insertHistoryBatch(db, h.buffer); err != nil {
			logger.Warnf("[HistoryPersistence] 设备 %d 批量写入失败 (第%d次): %v", h.deviceID, attempt+1, err)
			continue
		}

		// 写入成功
		h.buffer = make([]HistoryPoint, 0, historyFlushSize)
		h.retryCount = 0
		return
	}

	// 所有重试均失败，丢弃旧数据
	h.retryCount++
	discardCount := len(h.buffer) / 2
	logger.Errorf("[HistoryPersistence] 设备 %d flushBatch 连续失败 %d 次，丢弃 %d 条旧数据", h.deviceID, h.retryCount, discardCount)
	if discardCount > 0 {
		h.buffer = h.buffer[discardCount:]
	}
}

func insertHistoryBatch(db *sql.DB, rows []HistoryPoint) error {
	if len(rows) == 0 {
		return nil
	}
	placeholders := make([]string, len(rows))
	args := make([]interface{}, 0, len(rows)*8)
	for i, d := range rows {
		placeholders[i] = "(?, ?, ?, ?, ?, ?, ?, ?)"
		switch d.Value.Kind {
		case valueobj.KindText:
			args = append(args, d.DeviceID, d.PointID, 1, nil, d.Value.Text, d.Unit, d.DataType, d.Timestamp)
		case valueobj.KindBool:
			num := 0.0
			if d.Value.Bool {
				num = 1.0
			}
			args = append(args, d.DeviceID, d.PointID, 0, num, nil, d.Unit, d.DataType, d.Timestamp)
		default:
			args = append(args, d.DeviceID, d.PointID, 0, d.Value.Number, nil, d.Unit, d.DataType, d.Timestamp)
		}
	}
	sqlStr := "INSERT INTO device_data (device_id, point_id, value_type, value_num, value_text, unit, data_type, collected_at) VALUES " + strings.Join(placeholders, ", ")
	_, err := db.Exec(sqlStr, args...)
	return err
}

// Flush 手动刷新缓冲区
func (h *HistoryPersistence) Flush() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.flushBatch()
}

func parseHistoryTime(timestamp string) time.Time {
	timestamp = strings.TrimSpace(timestamp)
	if timestamp == "" {
		return time.Now()
	}
	if t, err := time.ParseInLocation("2006-01-02 15:04:05", timestamp, time.Local); err == nil {
		return t
	}
	if t, err := time.Parse(time.RFC3339, timestamp); err == nil {
		return t
	}
	return time.Now()
}
