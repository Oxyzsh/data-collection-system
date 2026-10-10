package infrastructure

import (
	"database/sql"
	"fmt"
	"strings"

	deviceapp "data-collection-system/internal/device/application"
	runtimeinfra "data-collection-system/internal/runtime/infrastructure"
	templateruntime "data-collection-system/internal/template/runtime"
)

type SelectorProvider struct {
	configDB  *sql.DB
	runtimeDB *sql.DB
}

func NewSelectorProvider(configDB, runtimeDB *sql.DB) *SelectorProvider {
	return &SelectorProvider{configDB: configDB, runtimeDB: runtimeDB}
}

func (p *SelectorProvider) GetPointStates(deviceID int) ([]deviceapp.SelectorPointState, error) {
	var templateID int
	if err := p.configDB.QueryRow("SELECT template_id FROM devices WHERE id = ?", deviceID).Scan(&templateID); err != nil {
		return nil, err
	}

	cfg, err := templateruntime.GetTemplateConfig(templateID)
	if err != nil {
		return nil, fmt.Errorf("template %d not found", templateID)
	}
	deviceParams := p.loadDeviceParams(deviceID)

	deviceRuntime, running := runtimeinfra.GetDataManager().GetDeviceRuntime(deviceID)
	states := make([]deviceapp.SelectorPointState, 0, len(cfg.Points))
	for _, point := range cfg.Points {
		state := deviceapp.SelectorPointState{
			PointID:        point.PointID,
			Name:           point.Name,
			Unit:           point.Unit,
			DataType:       point.DataType,
			Access:         strings.TrimSpace(point.Access),
			ValueMappingCN: point.ValueMappingCN,
			ValueMappingEN: point.ValueMappingEN,
		}
		if point.LocalIO != nil {
			applyLocalIOSelectorParams(&state, deviceParams)
		}
		if running {
			if value, _, found := deviceRuntime.GetPointValue(point.PointID); found {
				state.CurrentValue = value
				state.HasCurrent = true
			}
		}
		states = append(states, state)
	}

	return states, nil
}

func makePlaceholders(n int) string {
	if n <= 0 {
		return ""
	}
	return strings.TrimRight(strings.Repeat("?,", n), ",")
}

func (p *SelectorProvider) loadDeviceParams(deviceID int) map[string]string {
	params := make(map[string]string)
	rows, err := p.configDB.Query("SELECT param_name, param_value FROM device_params WHERE device_id = ?", deviceID)
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

func applyLocalIOSelectorParams(state *deviceapp.SelectorPointState, params map[string]string) {
	if state == nil || params["collect_method"] != "gpio" {
		return
	}
	if channel := strings.ToUpper(strings.TrimSpace(params["io_channel"])); channel != "" {
		state.Name = channel
	}
	if params["io_direction"] == "di" {
		state.Access = "r"
	} else if params["io_direction"] == "do" {
		state.Access = "rw"
	}
	if mapping := strings.TrimSpace(params["io_value_mapping_zh"]); mapping != "" {
		state.ValueMappingCN = mapping
	}
	if mapping := strings.TrimSpace(params["io_value_mapping_en"]); mapping != "" {
		state.ValueMappingEN = mapping
	}
}
