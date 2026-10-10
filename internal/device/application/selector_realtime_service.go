package application

import "time"

type SelectorRealtimeItem struct {
	// BindingKey 由前端订阅端传入，用来把返回值和页面绑定项重新对应上。
	BindingKey string
	DeviceID   int
	PointID    int
	TypeID     int
}

// SelectorRealtimeValue 是实时 selector 的返回值。
type SelectorRealtimeValue struct {
	BindingKey string      `json:"binding_key"`
	DeviceID   int         `json:"device_id"`
	PointID    int         `json:"point_id"`
	TypeID     int         `json:"type_id"`
	Value      interface{} `json:"value"`
	Unit       string      `json:"unit"`
	Valid      bool        `json:"valid"`
	Timestamp  string      `json:"timestamp"`
}

// SelectorRealtimeService 负责把实时订阅项转换为 selector 结果。
// 它不是重新实现一套 selector 逻辑，而是复用 SelectorService，再补上实时推送需要的字段。
type SelectorRealtimeService struct {
	selector *SelectorService
}

func NewSelectorRealtimeService(selector *SelectorService) *SelectorRealtimeService {
	return &SelectorRealtimeService{selector: selector}
}

// Resolve 先复用普通 selector 逻辑，再补上统一时间戳。
// 这样“普通查询”和“实时推送”共享同一套取值规则，避免两处逻辑逐渐漂移。
func (s *SelectorRealtimeService) Resolve(items []SelectorRealtimeItem) ([]SelectorRealtimeValue, error) {
	if len(items) == 0 {
		return []SelectorRealtimeValue{}, nil
	}

	requests := make([]SelectorValueRequest, 0, len(items))
	for _, item := range items {
		requests = append(requests, SelectorValueRequest{
			DeviceID: item.DeviceID,
			PointID:  item.PointID,
			TypeID:   item.TypeID,
		})
	}

	results, err := s.selector.GetValues(requests)
	if err != nil {
		return nil, err
	}

	nowStr := time.Now().Format(time.RFC3339)
	values := make([]SelectorRealtimeValue, 0, len(items))
	for i, item := range items {
		result := SelectorRealtimeValue{
			BindingKey: item.BindingKey,
			DeviceID:   item.DeviceID,
			PointID:    item.PointID,
			TypeID:     item.TypeID,
			Timestamp:  nowStr,
		}
		if i < len(results) {
			result.Value = results[i].Value
			result.Unit = results[i].Unit
			result.Valid = results[i].Valid
		}
		values = append(values, result)
	}

	return values, nil
}
