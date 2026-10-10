package history

import (
	"database/sql"
	"strings"
	"testing"

	"data-collection-system/internal/database"

	_ "modernc.org/sqlite"
)

// 全新空库：自动建唯一索引，不报错。
func TestEnsureUniqueIndexCreatesOnEmptyDB(t *testing.T) {
	db := openTestHistoryDB(t)
	t.Cleanup(func() { _ = db.Close() })
	if err := InitTables(); err != nil {
		t.Fatalf("InitTables() 空库不应报错: %v", err)
	}
	if !hasUniqueIndex(t, db) {
		t.Fatal("空库应自动创建唯一索引")
	}
}

// 存量库（有数据）且索引为普通索引：必须报错（拒绝启动），错误信息含迁移指引。
func TestEnsureUniqueIndexRejectsLegacyDB(t *testing.T) {
	db := openTestHistoryDB(t)
	t.Cleanup(func() { _ = db.Close() })
	// 模拟存量库：普通索引 + 一行数据
	mustExec(t, db, `CREATE TABLE device_data (
		id INTEGER PRIMARY KEY AUTOINCREMENT, device_id INTEGER NOT NULL, point_id INTEGER NOT NULL,
		value_type INTEGER NOT NULL DEFAULT 0, value_num REAL, value_text TEXT,
		unit VARCHAR(20), data_type VARCHAR(50), collected_at DATETIME DEFAULT CURRENT_TIMESTAMP)`)
	mustExec(t, db, `CREATE INDEX idx_data_device_point_time ON device_data(device_id, point_id, collected_at)`)
	mustExec(t, db, `INSERT INTO device_data (device_id, point_id, value_num, collected_at) VALUES (1, 1, 1.0, '2026-01-01 00:00:00')`)

	err := InitTables()
	if err == nil {
		t.Fatal("存量库缺唯一索引应拒绝启动")
	}
	if !strings.Contains(err.Error(), "去重与唯一索引迁移") {
		t.Fatalf("错误信息应包含迁移指引，实际: %v", err)
	}
}

// 已迁移库（唯一索引就绪）：正常通过。
func TestEnsureUniqueIndexAcceptsMigratedDB(t *testing.T) {
	db := openTestHistoryDB(t)
	t.Cleanup(func() { _ = db.Close() })
	mustExec(t, db, `CREATE TABLE device_data (
		id INTEGER PRIMARY KEY AUTOINCREMENT, device_id INTEGER NOT NULL, point_id INTEGER NOT NULL,
		value_type INTEGER NOT NULL DEFAULT 0, value_num REAL, value_text TEXT,
		unit VARCHAR(20), data_type VARCHAR(50), collected_at DATETIME DEFAULT CURRENT_TIMESTAMP)`)
	mustExec(t, db, `CREATE UNIQUE INDEX idx_data_device_point_time ON device_data(device_id, point_id, collected_at)`)
	if err := InitTables(); err != nil {
		t.Fatalf("已迁移库不应报错: %v", err)
	}
}

func openTestHistoryDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	// :memory: 每条连接是独立库，限制单连接确保测试全程读写同一个内存库。
	db.SetMaxOpenConns(1)
	old := database.HistoryDB
	database.HistoryDB = db
	t.Cleanup(func() { database.HistoryDB = old })
	return db
}

func mustExec(t *testing.T, db *sql.DB, sqlStr string) {
	t.Helper()
	if _, err := db.Exec(sqlStr); err != nil {
		t.Fatalf("exec %q: %v", sqlStr, err)
	}
}

func hasUniqueIndex(t *testing.T, db *sql.DB) bool {
	t.Helper()
	ready, err := hasDeviceDataUniqueIndex(db)
	if err != nil {
		t.Fatalf("hasDeviceDataUniqueIndex: %v", err)
	}
	return ready
}
