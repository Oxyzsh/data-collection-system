package domain

import (
	"encoding/json"
	"fmt"

	alarmdomain "data-collection-system/internal/templatealarm"
)

// TemplateData 是模板在数据库中的 JSON 存储格式。
// 一条模板记录会把 points 和 alarms 一起序列化到 data 字段里。
type TemplateData struct {
	Points []DevicePoint             `json:"points"`
	Alarms []alarmdomain.DeviceAlarm `json:"alarms"`
}

// Serialize 将测点和告警序列化为模板 data JSON。
func Serialize(points []DevicePoint, alarms []alarmdomain.DeviceAlarm) ([]byte, error) {
	data := TemplateData{Points: points, Alarms: alarms}
	if err := data.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(data)
}

// Deserialize 将数据库中的模板 JSON 还原为结构体。
func Deserialize(jsonStr string) (*TemplateData, error) {
	if jsonStr == "" {
		return &TemplateData{}, nil
	}
	var data TemplateData
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		return nil, fmt.Errorf("模板数据反序列化失败: %w", err)
	}
	return &data, nil
}

// Validate 校验模板 JSON 的内部一致性。
// 这里主要检查测点编号唯一性，以及告警引用的 PointID 是否存在。
func (d *TemplateData) Validate() error {
	if d.Points == nil {
		d.Points = make([]DevicePoint, 0)
	}
	if d.Alarms == nil {
		d.Alarms = make([]alarmdomain.DeviceAlarm, 0)
	}

	seen := make(map[int]bool)
	for _, p := range d.Points {
		if p.PointID <= 0 {
			return fmt.Errorf("测点 PointID 必须大于 0，当前值: %d", p.PointID)
		}
		if seen[p.PointID] {
			return fmt.Errorf("测点 PointID 重复: %d", p.PointID)
		}
		seen[p.PointID] = true
	}

	pointSet := make(map[int]bool)
	for _, p := range d.Points {
		pointSet[p.PointID] = true
	}
	for _, a := range d.Alarms {
		if a.PointID <= 0 {
			return fmt.Errorf("告警 PointID 必须大于 0，当前值: %d", a.PointID)
		}
		if !pointSet[a.PointID] {
			return fmt.Errorf("告警引用的 PointID %d 不存在于测点列表中", a.PointID)
		}
	}
	return nil
}
