package infrastructure

import (
	"database/sql"
	"strconv"
	"strings"

	"data-collection-system/internal/device/application"
	"data-collection-system/internal/valueobj"
)

type SQLQueryRepository struct {
	configDB  *sql.DB
	historyDB *sql.DB
}

// SQLQueryRepository 把“配置库查询”和“历史库查询”聚合到一个查询仓储里。
// 对查询场景来说，调用方更关心“拿到设备详情/历史”这个结果，
// 不必关心中间跨了几张表、用了哪个数据库连接。
func NewSQLQueryRepository(configDB, historyDB *sql.DB) *SQLQueryRepository {
	return &SQLQueryRepository{configDB: configDB, historyDB: historyDB}
}

// ListDevicesWithTemplate 直接返回列表用例需要的扁平结果。
// Enabled 在数据库里是 int，这里先转成 bool，再交给上层使用。
func (r *SQLQueryRepository) ListDevicesWithTemplate() ([]application.DeviceListItem, error) {
	rows, err := r.configDB.Query(`
		SELECT d.id, d.name, d.template_id, d.status, d.description, d.created_at, d.updated_at, d.enabled, t.name,
		       COALESCE(io.param_value, '')
		FROM devices d
		LEFT JOIN device_templates t ON d.template_id = t.id
		LEFT JOIN device_params io ON io.device_id = d.id AND io.param_name = 'io_channel'
		ORDER BY d.id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]application.DeviceListItem, 0)
	for rows.Next() {
		var item application.DeviceListItem
		var enabled int
		if err := rows.Scan(&item.ID, &item.Name, &item.TemplateID, &item.Status, &item.Description, &item.CreatedAt, &item.UpdatedAt, &enabled, &item.TemplateName, &item.IOChannel); err != nil {
			return nil, err
		}
		item.Enabled = enabled == 1
		items = append(items, item)
	}
	return items, nil
}

// GetDeviceDetail 查询设备详情主记录。
// 注意这里只查“详情主干”，参数表拆到 GetDeviceParams 里查，
// 由 QueryService 负责把两部分拼成最终 DeviceDetail。
func (r *SQLQueryRepository) GetDeviceDetail(deviceID int) (application.DeviceDetail, error) {
	var detail application.DeviceDetail
	var enabled int
	err := r.configDB.QueryRow(`
		SELECT d.id, d.name, d.template_id, d.status, d.description, d.created_at, d.updated_at, d.enabled,
		       COALESCE(t.name, ''), COALESCE(t.type, ''), COALESCE(t.collect_method, '')
		FROM devices d
		LEFT JOIN device_templates t ON d.template_id = t.id
		WHERE d.id = ?
	`, deviceID).Scan(
		&detail.ID,
		&detail.Name,
		&detail.TemplateID,
		&detail.Status,
		&detail.Description,
		&detail.CreatedAt,
		&detail.UpdatedAt,
		&enabled,
		&detail.TemplateName,
		&detail.TemplateType,
		&detail.CollectMethod,
	)
	if err != nil {
		return application.DeviceDetail{}, err
	}
	detail.Enabled = enabled == 1
	return detail, nil
}

// GetDeviceParams 单独查询参数键值表。
// 这种拆分能让 repository 方法保持单一职责，也便于其他用例复用。
func (r *SQLQueryRepository) GetDeviceParams(deviceID int) ([]application.ParamEntry, error) {
	rows, err := r.configDB.Query(`
		SELECT param_name, param_value
		FROM device_params
		WHERE device_id = ?
	`, deviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	params := make([]application.ParamEntry, 0)
	for rows.Next() {
		var p application.ParamEntry
		if err := rows.Scan(&p.Name, &p.Value); err != nil {
			continue
		}
		params = append(params, p)
	}
	return params, nil
}

// GetDeviceHistory 从历史库读取点位历史。
// 数据库存储时把 text/number/bool 拆成了不同列或标记，这里要重新还原成统一的 HistoryPoint。
func (r *SQLQueryRepository) GetDeviceHistory(deviceID int, pointID, limit, start, end string) ([]application.HistoryPoint, error) {
	// limit 归一化：默认 100、上限 5000，避免一次查询拉取过多数据。
	if n, err := strconv.Atoi(strings.TrimSpace(limit)); err != nil || n <= 0 {
		limit = "100"
	} else if n > 5000 {
		limit = "5000"
	}

	// 过滤条件全部可选：按测点、按时间范围（collected_at 为 "2006-01-02 15:04:05" 本地时间字符串）。
	conds := []string{"device_id = ?"}
	args := []interface{}{deviceID}
	if strings.TrimSpace(pointID) != "" {
		conds = append(conds, "point_id = ?")
		args = append(args, pointID)
	}
	if s := strings.TrimSpace(start); s != "" {
		conds = append(conds, "collected_at >= ?")
		args = append(args, s)
	}
	if e := strings.TrimSpace(end); e != "" {
		conds = append(conds, "collected_at <= ?")
		args = append(args, e)
	}

	query := `
		SELECT collected_at, value_type, value_num, value_text, data_type
		FROM device_data
		WHERE ` + strings.Join(conds, " AND ") + `
		ORDER BY collected_at DESC LIMIT ?`
	args = append(args, limit)

	rows, err := r.historyDB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	history := make([]application.HistoryPoint, 0)
	for rows.Next() {
		var h application.HistoryPoint
		var valueType int
		var valueNum sql.NullFloat64
		var valueText sql.NullString
		var dataType string
		if err := rows.Scan(&h.Timestamp, &valueType, &valueNum, &valueText, &dataType); err != nil {
			continue
		}
		if valueType == 1 {
			h.ValueKind = "text"
			if valueText.Valid {
				h.Value = valueText.String
			} else {
				h.Value = ""
			}
		} else if strings.EqualFold(strings.TrimSpace(dataType), valueobj.DataTypeBOOL) {
			h.ValueKind = "bool"
			h.Value = valueNum.Valid && valueNum.Float64 != 0
		} else {
			h.ValueKind = "number"
			if valueNum.Valid {
				h.Value = valueNum.Float64
			} else {
				h.Value = 0.0
			}
		}
		history = append(history, h)
	}

	// SQL 里按 DESC 取最近 N 条更高效，但前端通常希望时间从旧到新展示，
	// 所以这里再原地反转一次切片顺序。
	for i, j := 0, len(history)-1; i < j; i, j = i+1, j-1 {
		history[i], history[j] = history[j], history[i]
	}

	return history, nil
}
