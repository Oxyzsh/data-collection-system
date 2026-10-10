package infrastructure

import (
	"database/sql"
	"time"

	alarmdomain "data-collection-system/internal/templatealarm"
	templatebiz "data-collection-system/internal/template/domain"
)

type SQLConfigRepository struct {
	db *sql.DB
}

func NewSQLConfigRepository(db *sql.DB) *SQLConfigRepository {
	return &SQLConfigRepository{db: db}
}

// GetTemplateMeta 读取模板的元信息。
func (r *SQLConfigRepository) GetTemplateMeta(templateID int) (templatebiz.TemplateMeta, error) {
	var meta templatebiz.TemplateMeta
	var isBuiltIn int
	err := r.db.QueryRow(
		"SELECT id, name, type, collect_method, is_built_in, description FROM device_templates WHERE id = ?",
		templateID,
	).Scan(&meta.ID, &meta.Name, &meta.Type, &meta.CollectMethod, &isBuiltIn, &meta.Description)
	if err != nil {
		return meta, err
	}
	meta.IsBuiltIn = isBuiltIn == 1
	return meta, nil
}

// ListTemplateReferences 读取引用该模板的设备列表。
func (r *SQLConfigRepository) ListTemplateReferences(templateID int) ([]templatebiz.TemplateReference, error) {
	rows, err := r.db.Query(
		"SELECT id, name, status, enabled FROM devices WHERE template_id = ? ORDER BY id ASC",
		templateID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	refs := make([]templatebiz.TemplateReference, 0)
	for rows.Next() {
		var ref templatebiz.TemplateReference
		if err := rows.Scan(&ref.DeviceID, &ref.Name, &ref.Status, &ref.Enabled); err != nil {
			return nil, err
		}
		refs = append(refs, ref)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return refs, nil
}

// DeleteTemplate 删除模板主表记录。
func (r *SQLConfigRepository) DeleteTemplate(templateID int) error {
	_, err := r.db.Exec("DELETE FROM device_templates WHERE id = ?", templateID)
	return err
}

// InsertTemplate 新建模板并持久化测点/告警 JSON。
// ID 在这里显式分配，不交给 SQLite 自增：内置模板预留了 1..builtInTemplateIDMax，
// 导入/新建模板必须从预留区之后开始，否则新模板会占掉内置模板的 ID 空间
// （老库上已经出现过导入模板占住 3、4，把内置模板挤走的情况）。
// 取 MAX(库内最大 ID, 预留上限) + 1 而不是固定 1001，是为了兼容尚未手动迁移的老库：
// 库里若只有 3、4、5 这类低号导入模板，第一条新模板同样直接落到 1001 起；
// 若库里已经有 1001+ 的模板，则接在它们后面，不撞号。
func (r *SQLConfigRepository) InsertTemplate(
	name, templateType, collectMethod, description string,
	points []templatebiz.DevicePoint,
	alarms []alarmdomain.DeviceAlarm,
	now time.Time,
) (int64, error) {
	nowStr := now.Format("2006-01-02 15:04:05")
	dataJSON, err := templatebiz.Serialize(points, alarms)
	if err != nil {
		return 0, err
	}

	result, err := r.db.Exec(
		`INSERT INTO device_templates (id, name, type, collect_method, is_built_in, description, data, point_count, alarm_count, created_at, updated_at)
		 SELECT MAX((SELECT COALESCE(MAX(id), 0) FROM device_templates), ?) + 1, ?, ?, ?, 0, ?, ?, ?, ?, ?, ?`,
		builtInTemplateIDMax, name, templateType, collectMethod, description, string(dataJSON), len(points), len(alarms), nowStr, nowStr,
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// ReplaceTemplate 替换模板内容。
// 这里在事务内更新模板主表，并清理相关设备的告警覆盖配置。
func (r *SQLConfigRepository) ReplaceTemplate(
	templateID int,
	name, templateType, collectMethod, description string,
	points []templatebiz.DevicePoint,
	alarms []alarmdomain.DeviceAlarm,
	now time.Time,
) error {
	nowStr := now.Format("2006-01-02 15:04:05")
	dataJSON, err := templatebiz.Serialize(points, alarms)
	if err != nil {
		return err
	}

	tx, err := r.db.Begin()
	if err != nil {
		return err
	}

	if _, err := tx.Exec(
		"DELETE FROM cfg_dev_warn WHERE device_id IN (SELECT id FROM devices WHERE template_id = ?)",
		templateID,
	); err != nil {
		_ = tx.Rollback()
		return err
	}

	if _, err := tx.Exec(
		"UPDATE device_templates SET name = ?, type = ?, collect_method = ?, is_built_in = ?, description = ?, data = ?, point_count = ?, alarm_count = ?, updated_at = ? WHERE id = ?",
		name, templateType, collectMethod, 0, description, string(dataJSON), len(points), len(alarms), nowStr, templateID,
	); err != nil {
		_ = tx.Rollback()
		return err
	}

	return tx.Commit()
}
