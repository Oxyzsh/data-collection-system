package database

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"

	"data-collection-system/internal/bootstrap"
	"data-collection-system/internal/shared/logger"

	_ "modernc.org/sqlite"
)

// 数据库文件路径按需解析。
//
// 必须是函数而不是包级变量：包级变量会在包 init 期调用 bootstrap.DataPath，
// 从而在任何测试代码运行之前就把配置快照固定下来（sync.Once），测试再也无法用
// GOLOOK_DATA_DIR 指定数据目录；配置缺失时还会在 init 期直接 logger.Fatal 退出进程。
func ConfigDBPath() string {
	return bootstrap.DataPath("backend", "db", "config.db")
}

func RuntimeDBPath() string {
	return bootstrap.DataPath("backend", "db", "runtime.db")
}

func HistoryDBPath() string {
	return bootstrap.DataPath("backend", "db", "history.db")
}

// 三个数据库连接，按职责隔离。
// database 包只负责连接、PRAGMA、健康检查和通用访问底座，不负责业务表定义。
var (
	// ConfigDB 配置持久化数据库（低频写入）
	ConfigDB *sql.DB

	// RuntimeDB 运行时缓存数据库（高频读写）
	RuntimeDB *sql.DB

	// HistoryDB 历史数据数据库（追加写入，可定期归档清理）
	HistoryDB *sql.DB
)

// dbProfile 数据库连接配置档案
// 不同职责的数据库使用不同的 PRAGMA 参数以获得最优性能
type dbProfile struct {
	journalMode  string
	synchronous  string
	cacheSizeKB  int
	busyTimeout  int
	maxOpenConns int
	maxIdleConns int

	// autoVacuumIncremental 新库是否启用增量回收（auto_vacuum=INCREMENTAL，spec 6.1）：
	// DELETE 退回 freelist 的页可被 incremental_vacuum 逐步释放、文件随之收缩。
	// 仅对空库设置；存量库保持 NONE，迁移由人工执行（R9）。
	autoVacuumIncremental bool
}

var (
	// profileConfig 配置库：低频写入，优先数据安全
	profileConfig = dbProfile{
		journalMode:           "WAL",
		synchronous:           "NORMAL",
		cacheSizeKB:           4096,
		busyTimeout:           5000,
		maxOpenConns:          5,
		maxIdleConns:          2,
		autoVacuumIncremental: true,
	}

	// profileRuntime 运行时库：高频读写，优先吞吐量；运行态数据有界，不启用增量回收（spec 6.1）
	profileRuntime = dbProfile{
		journalMode:  "WAL",
		synchronous:  "NORMAL",
		cacheSizeKB:  16384,
		busyTimeout:  1000,
		maxOpenConns: 10,
		maxIdleConns: 5,
	}

	// profileHistory 历史库：追加写入为主
	profileHistory = dbProfile{
		journalMode:           "WAL",
		synchronous:           "NORMAL",
		cacheSizeKB:           8192,
		busyTimeout:           3000,
		maxOpenConns:          10,
		maxIdleConns:          5,
		autoVacuumIncremental: true,
	}
)

// openDB 使用指定配置档案打开 SQLite 数据库连接。
//
// auto_vacuum 的建立时序（spec 6.1）：该设置只在库文件为空（不存在或 0 字节）时才能
// 直接生效；文件一旦被写过（哪怕只是 journal_mode=WAL 写入的首个页），再设置只会进入
// pending 状态、静默失效（需将来某次 VACUUM 才生效）。因此必须在任何写 PRAGMA（尤其
// journal_mode=WAL）之前执行；已含数据的库不设置——pending 会在将来某次 VACUUM 时
// 意外切换模式，存量迁移交由人工执行（R9）。
func openDB(path string, profile dbProfile) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}

	if profile.autoVacuumIncremental {
		// sql.Open 是惰性的，此刻文件尚未创建；判据是「文件不存在或 0 字节」，而非
		// 「无用户表」。失败仅告警不阻断：模式设置失败不应影响数据库可用性。
		if fi, statErr := os.Stat(path); statErr != nil || fi.Size() == 0 {
			if _, err := db.Exec("PRAGMA auto_vacuum=INCREMENTAL"); err != nil {
				logger.Warnf("[DB] 新库启用增量回收失败 %s: %v\n", path, err)
			}
		}
	}

	// 连接池配置（SQLite 使用 WAL 模式时可以支持多个读取连接）
	db.SetMaxOpenConns(profile.maxOpenConns)
	db.SetMaxIdleConns(profile.maxIdleConns)

	pragmas := buildPragmas(profile)

	// 先在当前连接上执行一次
	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			logger.Warnf("[DB] 设置 PRAGMA 失败: %s, 错误: %v\n", pragma, err)
		}
	}

	// 对连接池中的连接逐个预热，减少“新连接未携带 PRAGMA”概率
	if err := warmupPoolConnections(db, profile.maxOpenConns, pragmas); err != nil {
		logger.Warnf("[DB] 连接池预热失败: %v", err)
	}

	if err = db.Ping(); err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}

func buildPragmas(profile dbProfile) []string {
	return []string{
		"PRAGMA journal_mode=" + profile.journalMode,
		"PRAGMA synchronous=" + profile.synchronous,
		fmt.Sprintf("PRAGMA cache_size=-%d", profile.cacheSizeKB),
		fmt.Sprintf("PRAGMA busy_timeout=%d", profile.busyTimeout),
		"PRAGMA foreign_keys=ON",
		"PRAGMA temp_store=MEMORY",
		"PRAGMA mmap_size=268435456", // 256MB mmap
	}
}

func warmupPoolConnections(db *sql.DB, maxOpenConns int, pragmas []string) error {
	if maxOpenConns <= 0 {
		maxOpenConns = 1
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conns := make([]*sql.Conn, 0, maxOpenConns)
	for i := 0; i < maxOpenConns; i++ {
		conn, err := db.Conn(ctx)
		if err != nil {
			break
		}
		for _, pragma := range pragmas {
			if _, err := conn.ExecContext(ctx, pragma); err != nil {
				_ = conn.Close()
				return fmt.Errorf("conn pragma init failed: %w", err)
			}
		}
		conns = append(conns, conn)
	}

	for _, conn := range conns {
		_ = conn.Close()
	}
	return nil
}

// openDBWithRepair 带健康检查和自动修复的数据库打开函数
func openDBWithRepair(path string, profile dbProfile) (*sql.DB, error) {
	db, err := openDB(path, profile)
	if err == nil {
		if checkDatabaseIntegrity(db) {
			return db, nil
		}
		logger.Warnf("[DB] 数据库 %s 完整性检查失败，尝试修复...\n", path)
		db.Close()
	}

	if repairDatabase(path) {
		db, err = openDB(path, profile)
		if err == nil && checkDatabaseIntegrity(db) {
			logger.Infof("[DB] 数据库 %s 修复成功\n", path)
			return db, nil
		}
		if db != nil {
			db.Close()
		}
	}

	logger.Warnf("[DB] 数据库 %s 无法修复，执行重建\n", path)
	if err := backupAndRebuildDatabase(path); err != nil {
		return nil, fmt.Errorf("数据库 %s 重建失败: %v", path, err)
	}

	return openDB(path, profile)
}

// checkDatabaseIntegrity 检查数据库完整性
func checkDatabaseIntegrity(db *sql.DB) bool {
	var result string
	err := db.QueryRow("PRAGMA integrity_check").Scan(&result)
	if err != nil || result != "ok" {
		logger.Warnf("[DB] 完整性检查结果: %s\n", result)
		return false
	}
	return true
}

// repairDatabase 尝试修复损坏的数据库
func repairDatabase(path string) bool {
	tmpDB, err := sql.Open("sqlite", path)
	if err != nil {
		return false
	}
	defer tmpDB.Close()

	if _, err = tmpDB.Exec("REINDEX"); err == nil {
		logger.Infof("[DB] 索引重建成功\n")
		return true
	}

	logger.Errorf("[DB] 索引重建失败: %v\n", err)
	return false
}

// backupAndRebuildDatabase 备份旧数据库文件并创建新的空数据库
func backupAndRebuildDatabase(path string) error {
	timestamp := "20060102_150405"
	backupPath := path + fmt.Sprintf(".broken_%s", timestamp)

	if _, err := os.Stat(path); err == nil {
		if err := os.Rename(path, backupPath); err != nil {
			logger.Warnf("[DB] 无法备份数据库: %v，直接删除\n", err)
			os.Remove(path)
		} else {
			logger.Infof("[DB] 已备份损坏数据库到: %s\n", backupPath)
		}
	}

	// 同时清理 WAL 和 SHM 文件
	os.Remove(path + "-wal")
	os.Remove(path + "-shm")

	newDB, err := sql.Open("sqlite", path)
	if err != nil {
		return fmt.Errorf("创建新数据库失败: %v", err)
	}
	newDB.Close()
	logger.Infof("[DB] 已创建新数据库: %s\n", path)

	return nil
}

// InitDB 初始化三个数据库连接
func InitDB() error {
	if err := os.MkdirAll(bootstrap.DataPath("backend", "db"), 0o755); err != nil {
		return err
	}

	configPath := ConfigDBPath()
	runtimePath := RuntimeDBPath()
	historyPath := HistoryDBPath()

	var err error

	ConfigDB, err = openDB(configPath, profileConfig)
	if err != nil {
		return err
	}
	logger.Infof("配置库初始化成功: %s", configPath)

	RuntimeDB, err = openDBWithRepair(runtimePath, profileRuntime)
	if err != nil {
		return err
	}
	logger.Infof("运行时库初始化成功: %s", runtimePath)

	HistoryDB, err = openDBWithRepair(historyPath, profileHistory)
	if err != nil {
		return err
	}
	logger.Infof("历史库初始化成功: %s", historyPath)

	// 复验增量回收模式（spec 6.1）：新建库应在打开时已落进文件头；仍不是 INCREMENTAL
	// 说明是存量未迁移库（或设置静默失效），提示人工按手册迁移。runtime 库不参与。
	verifyIncrementalVacuum(ConfigDB, "配置库", configPath)
	verifyIncrementalVacuum(HistoryDB, "历史库", historyPath)

	return nil
}

// verifyIncrementalVacuum 复验库的 auto_vacuum 模式（2 即 INCREMENTAL），仅记录日志、
// 不做补救：存量迁移由人工在设备上执行（R9），主程序不自动迁移。
func verifyIncrementalVacuum(db *sql.DB, name, path string) {
	var v int
	if err := db.QueryRow("PRAGMA auto_vacuum").Scan(&v); err != nil {
		logger.Warnf("[DB] %s 增量回收模式查询失败 %s: %v", name, path, err)
		return
	}
	if v == 2 {
		logger.Infof("[DB] %s 已启用增量回收（auto_vacuum=INCREMENTAL）: %s", name, path)
		return
	}
	logger.Warnf("[DB] %s 未启用增量回收，建议按手册迁移: %s (auto_vacuum=%d)", name, path, v)
}

// CloseDB 关闭所有数据库连接
func CloseDB() {
	if ConfigDB != nil {
		ConfigDB.Close()
	}
	if RuntimeDB != nil {
		RuntimeDB.Close()
	}
	if HistoryDB != nil {
		HistoryDB.Close()
	}
}
