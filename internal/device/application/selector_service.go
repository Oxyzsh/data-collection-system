package application

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	devicedomain "data-collection-system/internal/device/domain"
	"data-collection-system/internal/valueobj"
)

type SelectorType struct {
	TypeID    int    `json:"type_id"`
	Key       string `json:"key"`
	Name      string `json:"name"`
	NameEN    string `json:"name_en"`
	ValueKind string `json:"value_kind"`
}

// SelectorPoint / SelectorValueResult 是 selector 用例的输出模型。
// selector 的核心思想是：同一个点位，可以衍生出“当前值 / 映射值 / 告警数”等不同可绑定值。
type SelectorPoint struct {
	PointID       int            `json:"point_id"`
	Name          string         `json:"name"`
	Unit          string         `json:"unit"`
	DataType      string         `json:"data_type"`
	SelectorTypes []SelectorType `json:"selector_types"`
}

type SelectorValueRequest struct {
	DeviceID int `json:"device_id"`
	PointID  int `json:"point_id"`
	TypeID   int `json:"type_id"`
}

// SelectorValueResult 是 selector 的查询结果。
type SelectorValueResult struct {
	DeviceID int         `json:"device_id"`
	PointID  int         `json:"point_id"`
	TypeID   int         `json:"type_id"`
	Value    interface{} `json:"value"`
	Unit     string      `json:"unit"`
	Valid    bool        `json:"valid"`
}

// SelectorPointState 是从配置态和运行态拼出来的测点快照。
// 这是 provider 返回给 service 的“中间态”：
// - 既带模板/配置里的静态信息；
// - 也带当前实时值和告警计数所需的运行态信息。
type SelectorPointState struct {
	PointID        int
	Name           string
	Unit           string
	DataType       string
	Access         string
	ValueMappingCN string
	ValueMappingEN string
	CurrentValue   valueobj.CollectedValue
	HasCurrent     bool
}

// SelectorCountsProvider 提供 selector 所需的点状态。
type SelectorCountsProvider interface {
	// provider 是 selector 的数据聚合入口。
	// service 不关心这些数据来自 SQL 还是 runtime 缓存。
	GetPointStates(deviceID int) ([]SelectorPointState, error)
}

type SelectorService struct {
	provider SelectorCountsProvider
}

func NewSelectorService(provider SelectorCountsProvider) *SelectorService {
	return &SelectorService{provider: provider}
}

// ListPoints 返回某个设备可用于 selector 的测点列表。
// 这里主要做筛选和转换：先拿点状态，再过滤 Access/数据类型，最后构建前端可选类型。
func (s *SelectorService) ListPoints(deviceID int, numericOnly bool) ([]SelectorPoint, error) {
	states, err := s.provider.GetPointStates(deviceID)
	if err != nil {
		return nil, err
	}

	points := make([]SelectorPoint, 0, len(states))
	for _, state := range states {
		if state.Access != "r" && state.Access != "rw" {
			continue
		}
		if numericOnly && !isNumericDataType(state.DataType) {
			continue
		}

		points = append(points, SelectorPoint{
			PointID:       state.PointID,
			Name:          state.Name,
			Unit:          state.Unit,
			DataType:      state.DataType,
			SelectorTypes: buildSelectorTypes(state),
		})
	}
	return points, nil
}

// GetValues 批量解析 selector 值。
// 先按 device 去重拿状态，再按请求逐条组装结果，避免重复查运行态。
// 这属于“批处理优化”：多条请求里如果指向同一 device，就只查一次点状态。
func (s *SelectorService) GetValues(items []SelectorValueRequest) ([]SelectorValueResult, error) {
	if len(items) == 0 {
		return []SelectorValueResult{}, nil
	}

	deviceSeen := make(map[int]struct{})
	pointStateByDevice := make(map[int]map[int]SelectorPointState)

	for _, item := range items {
		if _, ok := deviceSeen[item.DeviceID]; ok {
			continue
		}
		deviceSeen[item.DeviceID] = struct{}{}

		states, err := s.provider.GetPointStates(item.DeviceID)
		if err != nil {
			return nil, err
		}
		pointStateByDevice[item.DeviceID] = toPointStateMap(states)
	}

	results := make([]SelectorValueResult, 0, len(items))
	for _, item := range items {
		result := SelectorValueResult{
			DeviceID: item.DeviceID,
			PointID:  item.PointID,
			TypeID:   item.TypeID,
			Valid:    false,
			Unit:     "",
		}

		pointState, ok := pointStateByDevice[item.DeviceID][item.PointID]
		if !ok {
			results = append(results, result)
			continue
		}

		switch item.TypeID {
		case devicedomain.SelectorTypeCurrentValue:
			if pointState.HasCurrent {
				result.Value = pointState.CurrentValue.AsAny()
				result.Unit = pointState.Unit
				result.Valid = true
			}
		case devicedomain.SelectorTypeMappedValue:
			if pointState.HasCurrent {
				result.Value = resolveMappedText(pointState)
				result.Valid = true
			}
		default:
			return nil, fmt.Errorf("unsupported selector type id %d", item.TypeID)
		}

		results = append(results, result)
	}

	return results, nil
}

// buildSelectorTypes 为单个点生成前端可选的值类型列表。
// 这里把“一个点支持什么绑定方式”的规则集中起来，避免散落在 handler 或前端。
func buildSelectorTypes(state SelectorPointState) []SelectorType {
	types := []SelectorType{
		{
			TypeID:    devicedomain.SelectorTypeCurrentValue,
			Key:       "current_value",
			Name:      "当前值",
			NameEN:    "Current Value",
			ValueKind: selectorValueKind(state.CurrentValue, state.DataType),
		},
	}

	if hasValueMapping(state) {
		types = append(types, SelectorType{
			TypeID:    devicedomain.SelectorTypeMappedValue,
			Key:       "mapped_value",
			Name:      "值映射",
			NameEN:    "Mapped Value",
			ValueKind: string(valueobj.KindText),
		})
	}

	return types
}

// selectorValueKind 推导 selector 输出的值类型。
// 它会优先相信当前运行值的 Kind；如果当前还没有实时值，再回退到模板 dataType 推断。
func selectorValueKind(current valueobj.CollectedValue, dataType string) string {
	if current.Kind != "" {
		return string(current.Kind)
	}
	normalized := strings.ToUpper(strings.TrimSpace(dataType))
	switch normalized {
	case valueobj.DataTypeSTRING:
		return string(valueobj.KindText)
	case valueobj.DataTypeBOOL:
		return string(valueobj.KindBool)
	default:
		return string(valueobj.KindNumber)
	}
}

// hasValueMapping 判断这个点是否配置了文本映射。
func hasValueMapping(state SelectorPointState) bool {
	return strings.TrimSpace(state.ValueMappingCN) != "" || strings.TrimSpace(state.ValueMappingEN) != ""
}

// resolveMappedText 把当前值按映射表转换成展示文本。
// 这是“原始值 -> 展示值”的一步转换，常见于布尔点、枚举点。
func resolveMappedText(state SelectorPointState) string {
	mappings := parseValueMapping(state.ValueMappingCN)
	if len(mappings) == 0 {
		mappings = parseValueMapping(state.ValueMappingEN)
	}
	key := currentValueKey(state.CurrentValue)
	if label, ok := mappings[key]; ok {
		return label
	}
	return key
}

// currentValueKey 把当前值转换成映射表查找键。
func currentValueKey(value valueobj.CollectedValue) string {
	switch value.Kind {
	case valueobj.KindText:
		return value.Text
	case valueobj.KindBool:
		if value.Bool {
			return "1"
		}
		return "0"
	default:
		if math.Abs(value.Number-math.Round(value.Number)) < 1e-9 {
			return strconv.FormatInt(int64(math.Round(value.Number)), 10)
		}
		return strconv.FormatFloat(value.Number, 'f', -1, 64)
	}
}

// parseValueMapping 解析 "0:关闭,1:开启" 这类字符串映射。
func parseValueMapping(raw string) map[string]string {
	items := strings.Split(strings.TrimSpace(raw), ",")
	out := make(map[string]string, len(items))
	for _, item := range items {
		parts := strings.SplitN(strings.TrimSpace(item), ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		label := strings.TrimSpace(parts[1])
		if key == "" || label == "" {
			continue
		}
		out[key] = label
	}
	return out
}

func isNumericDataType(dataType string) bool {
	switch strings.ToUpper(strings.TrimSpace(dataType)) {
	case valueobj.DataTypeBOOL, valueobj.DataTypeSTRING:
		return false
	default:
		return true
	}
}

func toPointStateMap(states []SelectorPointState) map[int]SelectorPointState {
	out := make(map[int]SelectorPointState, len(states))
	for _, state := range states {
		out[state.PointID] = state
	}
	return out
}

