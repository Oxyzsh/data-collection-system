package device

import (
	"database/sql"
	"fmt"
	"strings"

	collectormodule "data-collection-system/internal/collector"
	collectorapp "data-collection-system/internal/collector/application"
	"data-collection-system/internal/database"
	templateruntime "data-collection-system/internal/template/runtime"
	"data-collection-system/internal/valueobj"
)

type PointTemplate struct {
	PointID        int
	Name           string
	Unit           string
	DataType       string
	Access         string
	Enabled        bool
	Description    string
	ValueMappingCN string
	ValueMappingEN string
	LocalIO        bool
}

type RuntimePointValue = collectorapp.RuntimePointValue

type DevicePointsSnapshot struct {
	DeviceID int
	Running  bool
	Points   []PointTemplate
	Values   map[int]RuntimePointValue
}

func GetDevicePointsSnapshot(deviceID int) (DevicePointsSnapshot, error) {
	db := database.ConfigDB

	var templateID int
	if err := db.QueryRow("SELECT template_id FROM devices WHERE id = ?", deviceID).Scan(&templateID); err != nil {
		return DevicePointsSnapshot{}, err
	}
	deviceParams := queryDeviceParamMap(db, deviceID)

	item, err := templateruntime.GetTemplateConfig(templateID)
	if err != nil {
		return DevicePointsSnapshot{}, fmt.Errorf("template %d not found", templateID)
	}

	points := make([]PointTemplate, 0, len(item.Points))
	for _, p := range item.Points {
		point := PointTemplate{
			PointID:        p.PointID,
			Name:           p.Name,
			Unit:           p.Unit,
			DataType:       p.DataType,
			Access:         p.Access,
			Enabled:        p.Enabled != 0,
			Description:    p.Description,
			ValueMappingCN: p.ValueMappingCN,
			ValueMappingEN: p.ValueMappingEN,
			LocalIO:        p.LocalIO != nil,
		}
		applyLocalIODisplayParams(&point, deviceParams)
		points = append(points, point)
	}

	pointIDs := make([]int, 0, len(points))
	for _, p := range points {
		pointIDs = append(pointIDs, p.PointID)
	}
	runtimeValues, running := collectormodule.GetService().GetPointValues(deviceID, pointIDs)

	values := make(map[int]RuntimePointValue, len(points))
	if running {
		for _, p := range points {
			if v, ok := runtimeValues[p.PointID]; ok && v.Found {
				values[p.PointID] = RuntimePointValue{Value: v.Value, Timestamp: v.Timestamp, Found: true}
			}
		}
	}

	return DevicePointsSnapshot{
		DeviceID: deviceID,
		Running:  running,
		Points:   points,
		Values:   values,
	}, nil
}

func GetTemplatePointByDevice(deviceID, pointID int) (PointTemplate, error) {
	db := database.ConfigDB
	var templateID int
	if err := db.QueryRow("SELECT template_id FROM devices WHERE id = ?", deviceID).Scan(&templateID); err != nil {
		return PointTemplate{}, err
	}

	item, err := templateruntime.GetTemplateConfig(templateID)
	if err != nil {
		return PointTemplate{}, fmt.Errorf("template %d not found", templateID)
	}

	deviceParams := queryDeviceParamMap(db, deviceID)
	for _, p := range item.Points {
		if p.PointID == pointID {
			point := PointTemplate{
				PointID:        p.PointID,
				Name:           p.Name,
				Unit:           p.Unit,
				DataType:       p.DataType,
				Access:         p.Access,
				Enabled:        p.Enabled != 0,
				Description:    p.Description,
				ValueMappingCN: p.ValueMappingCN,
				ValueMappingEN: p.ValueMappingEN,
				LocalIO:        p.LocalIO != nil,
			}
			applyLocalIODisplayParams(&point, deviceParams)
			return point, nil
		}
	}
	return PointTemplate{}, sql.ErrNoRows
}

func WriteDevicePoint(deviceID, pointID int, value valueobj.CollectedValue) error {
	return collectormodule.GetService().WritePoint(deviceID, pointID, value)
}

func queryDeviceParamMap(db *sql.DB, deviceID int) map[string]string {
	params := make(map[string]string)
	rows, err := db.Query("SELECT param_name, param_value FROM device_params WHERE device_id = ?", deviceID)
	if err != nil {
		return params
	}
	defer rows.Close()
	for rows.Next() {
		var name, value string
		if rows.Scan(&name, &value) == nil {
			params[name] = value
		}
	}
	return params
}

func applyLocalIODisplayParams(point *PointTemplate, params map[string]string) {
	if point == nil || !point.LocalIO || params["collect_method"] != "gpio" {
		return
	}
	if channel := strings.ToUpper(strings.TrimSpace(params["io_channel"])); channel != "" {
		point.Name = channel
	}
	if params["io_direction"] == "di" {
		point.Access = "r"
	} else if params["io_direction"] == "do" {
		point.Access = "rw"
	}
	if mapping := strings.TrimSpace(params["io_value_mapping_zh"]); mapping != "" {
		point.ValueMappingCN = mapping
	}
	if mapping := strings.TrimSpace(params["io_value_mapping_en"]); mapping != "" {
		point.ValueMappingEN = mapping
	}
}
