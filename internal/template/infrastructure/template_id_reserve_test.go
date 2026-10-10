package infrastructure

import (
	"database/sql"
	"testing"
	"time"

	templatebiz "data-collection-system/internal/template/domain"

	_ "modernc.org/sqlite"
)

// newTemplateTestDB 建一个只用于模板表的内存库。
// 注意 :memory: 每个连接都是独立库，必须限制为单连接。
func newTemplateTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	return db
}

func templateIDByName(t *testing.T, db *sql.DB, name string) int {
	t.Helper()
	var id int
	if err := db.QueryRow("SELECT id FROM device_templates WHERE name = ?", name).Scan(&id); err != nil {
		t.Fatalf("查询模板 %s 的 ID 失败: %v", name, err)
	}
	return id
}

// seedLegacyDatabase 把空库改造成老库形态：内置模板只有 1、2，
// 导入模板占了 3、4、5（与本机现网配置库一致），另有一条已经落在导入区的 1001，
// 用来区分「预留区空位」和「自增」两种兜底结果。
func seedLegacyDatabase(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.Exec("DELETE FROM device_templates"); err != nil {
		t.Fatalf("清空模板表失败: %v", err)
	}
	dataJSON, err := templatebiz.Serialize(nil, nil)
	if err != nil {
		t.Fatalf("序列化模板数据失败: %v", err)
	}
	legacy := []struct {
		id      int
		name    string
		builtIn int
	}{
		{1, "采集器", 1},
		{2, "模拟温湿度", 1},
		{3, "SNMP-HOST", 0},
		{4, "modbus-温湿度", 0},
		{5, "北向Modbus-温湿度1", 0},
		{1001, "已迁移模板", 0},
	}
	for _, row := range legacy {
		if _, err := db.Exec(
			"INSERT INTO device_templates (id, name, type, collect_method, is_built_in, description, data, created_at, updated_at) VALUES (?, ?, 'modbus', 'modbus', ?, '', ?, '', '')",
			row.id, row.name, row.builtIn, string(dataJSON),
		); err != nil {
			t.Fatalf("插入老库模板 %s 失败: %v", row.name, err)
		}
	}
}

// 空库上新建/导入模板从内置预留区之后开始分配，第一条就是 1001。
func TestInsertTemplate_IDStartsAfterBuiltInReserve(t *testing.T) {
	db := newTemplateTestDB(t)
	if err := InitOnStartup(db); err != nil {
		t.Fatalf("InitOnStartup() error = %v", err)
	}
	repo := NewSQLConfigRepository(db)

	first, err := repo.InsertTemplate("导入模板1", "modbus", "modbus", "", nil, nil, time.Now())
	if err != nil {
		t.Fatalf("InsertTemplate() error = %v", err)
	}
	if want := int64(builtInTemplateIDMax + 1); first != want {
		t.Fatalf("首个导入模板 ID = %d, want %d", first, want)
	}

	second, err := repo.InsertTemplate("导入模板2", "modbus", "modbus", "", nil, nil, time.Now())
	if err != nil {
		t.Fatalf("InsertTemplate() error = %v", err)
	}
	if second != first+1 {
		t.Fatalf("第二个导入模板 ID = %d, want %d", second, first+1)
	}
}

// 老库（导入模板占了预留区低号）上：存量记录保持原 ID —— 程序不做迁移，
// 内置模板退到预留区里的第一个空位，新建模板仍从预留区之后开始。
func TestInitBuiltInTemplates_LegacyImportedTemplatesKeepIDs(t *testing.T) {
	db := newTemplateTestDB(t)
	if err := InitOnStartup(db); err != nil {
		t.Fatalf("首次 InitOnStartup() error = %v", err)
	}
	seedLegacyDatabase(t, db)

	if err := InitOnStartup(db); err != nil {
		t.Fatalf("老库 InitOnStartup() error = %v", err)
	}

	if id := templateIDByName(t, db, "开关量输入"); id != 6 {
		t.Fatalf("开关量输入 ID = %d, want 6（3 被导入模板占用后的预留区第一个空位）", id)
	}
	if id := templateIDByName(t, db, "开关量输出"); id != 7 {
		t.Fatalf("开关量输出 ID = %d, want 7", id)
	}
	if id := templateIDByName(t, db, "SNMP-HOST"); id != 3 {
		t.Fatalf("存量导入模板 SNMP-HOST ID = %d, want 3（本改动不做迁移）", id)
	}

	newID, err := NewSQLConfigRepository(db).InsertTemplate("新导入模板", "modbus", "modbus", "", nil, nil, time.Now())
	if err != nil {
		t.Fatalf("InsertTemplate() error = %v", err)
	}
	if want := int64(builtInTemplateIDMax + 2); newID != want {
		t.Fatalf("老库上新建模板 ID = %d, want %d（接在已有导入区模板 1001 之后）", newID, want)
	}
}

// 预留区占满时不再分配，避免内置模板落进导入区。
func TestFirstFreeBuiltInTemplateID_ReserveFull(t *testing.T) {
	db := newTemplateTestDB(t)
	if err := InitOnStartup(db); err != nil {
		t.Fatalf("InitOnStartup() error = %v", err)
	}
	if _, err := db.Exec("DELETE FROM device_templates"); err != nil {
		t.Fatalf("清空模板表失败: %v", err)
	}
	for id := 1; id <= builtInTemplateIDMax; id++ {
		if _, err := db.Exec("INSERT INTO device_templates (id, name, type) VALUES (?, ?, 'modbus')", id, "占位模板"); err != nil {
			t.Fatalf("插入占位模板 %d 失败: %v", id, err)
		}
	}

	if _, err := firstFreeBuiltInTemplateID(db); err == nil {
		t.Fatalf("预留区 1..%d 占满时应返回错误", builtInTemplateIDMax)
	}
}
