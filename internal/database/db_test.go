package database

import (
	"path/filepath"
	"testing"
)

// 全新文件库：openDB 后 auto_vacuum==2（设置须在 journal_mode=WAL 等写 PRAGMA 之前
// 执行，否则只进入 pending 静默失效——spec 6.1 的时序回归）。
func TestOpenDBNewFileEnablesIncrementalVacuum(t *testing.T) {
	path := filepath.Join(t.TempDir(), "new.db")
	profile := profileHistory
	profile.autoVacuumIncremental = true
	db, err := openDB(path, profile)
	if err != nil {
		t.Fatalf("openDB: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	var v int
	if err := db.QueryRow("PRAGMA auto_vacuum").Scan(&v); err != nil || v != 2 {
		t.Fatalf("auto_vacuum = %d err=%v, 期望 2", v, err)
	}
}

// 已存在非空库：openDB 不得改变其模式（保持 0），不得触发 VACUUM——
// 存量库若被置为 pending，会在将来某次 VACUUM 时意外切换模式（spec 6.1 末条，R9）。
func TestOpenDBExistingFileUntouched(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.db")

	// 先用未启用增量回收的档案建库并写入一行数据（文件非 0 字节），关闭
	first, err := openDB(path, profileRuntime)
	if err != nil {
		t.Fatalf("openDB(建库): %v", err)
	}
	if _, err := first.Exec("CREATE TABLE t (id INTEGER)"); err != nil {
		t.Fatalf("建表: %v", err)
	}
	if _, err := first.Exec("INSERT INTO t VALUES (1)"); err != nil {
		t.Fatalf("写入数据: %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("关闭建库连接: %v", err)
	}

	// 再以 autoVacuumIncremental=true 的档案打开既有库 → 模式应保持 0。
	// 连接池钉为单连接：pending 是「设置时那条连接」的内存态（不 VACUUM 则文件无痕，
	// 实测），只有让后续 VACUUM 落在同一条连接上，下方判别断言才可能显形——多连接
	// 池中 VACUUM 会落到别的连接，pending 永远观测不到。
	profile := profileHistory
	profile.autoVacuumIncremental = true
	profile.maxOpenConns = 1
	profile.maxIdleConns = 1
	db, err := openDB(path, profile)
	if err != nil {
		t.Fatalf("openDB(重开): %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	var v int
	if err := db.QueryRow("PRAGMA auto_vacuum").Scan(&v); err != nil || v != 0 {
		t.Fatalf("auto_vacuum = %d err=%v, 既有库应保持 0", v, err)
	}

	// 判别力加固：若实现误对非空库设置了 auto_vacuum（pending），VACUUM 会使其显形为 2；
	// 正确实现（stat 守卫在位）下 VACUUM 后仍为 0。
	if _, err := db.Exec("VACUUM"); err != nil {
		t.Fatalf("vacuum: %v", err)
	}
	var after int
	if err := db.QueryRow("PRAGMA auto_vacuum").Scan(&after); err != nil || after != 0 {
		t.Fatalf("VACUUM 后 auto_vacuum = %d err=%v, 期望 0（不得对既有库留下 pending）", after, err)
	}
}
