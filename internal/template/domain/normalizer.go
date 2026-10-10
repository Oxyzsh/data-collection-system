package domain

import (
	"strconv"
	"sync"

	alarmdomain "data-collection-system/internal/templatealarm"
)

// NormalizedPoint 是模板详情接口使用的统一测点视图。
type NormalizedPoint struct {
	PointID      int     `json:"point_id"`
	Type         string  `json:"type"`
	Name         string  `json:"name"`
	NameEN       string  `json:"name_en"`
	DataType     string  `json:"data_type"`
	Unit         string  `json:"unit"`
	UnitEN       string  `json:"unit_en"`
	Coefficient  float64 `json:"coefficient"`
	Precision    int     `json:"precision"`
	Access       string  `json:"access"`
	Addr         string  `json:"addr"`
	OID          string  `json:"oid"`
	RegisterAddr int     `json:"register_addr"`
	// HasRegisterAddr 区分“寄存器地址就是 0”和“没有寄存器配置”。
	// MODBUS 的 0 号寄存器是合法地址，前端不能再用 register_addr == 0 判断是否配置。
	HasRegisterAddr bool   `json:"has_register_addr"`
	Expression      string `json:"expression"`
	ValueMappingCN  string `json:"value_mapping_cn"`
	ValueMappingEN  string `json:"value_mapping_en"`
	Enabled         int    `json:"enabled"`
	Description     string `json:"description"`
}

// NormalizedAlarm 是模板详情接口使用的统一告警视图。
type NormalizedAlarm struct {
	PointID        int     `json:"point_id"`
	Name           string  `json:"name"`
	NameEN         string  `json:"name_en"`
	Level          string  `json:"level"`
	Enabled        bool    `json:"enabled"`
	StartLowLimit  float64 `json:"start_low_limit"`
	StartHighLimit float64 `json:"start_high_limit"`
	StartDelay     int     `json:"start_delay"`
	EndLowLimit    float64 `json:"end_low_limit"`
	EndHighLimit   float64 `json:"end_high_limit"`
	EndDelay       int     `json:"end_delay"`
	Description    string  `json:"description"`
}

type TemplateDetailResponse struct {
	Template   DeviceTemplate      `json:"template"`
	Points     []NormalizedPoint   `json:"points"`
	Alarms     []NormalizedAlarm   `json:"alarms"`
	References []TemplateDeviceRef `json:"references"`
}

type TemplateDeviceRef struct {
	DeviceID int    `json:"device_id"`
	Name     string `json:"name"`
	Status   string `json:"status"`
	Enabled  int    `json:"enabled"`
}

type PointNormalizer interface {
	// Type 返回该归一化器支持的模板类型。
	Type() string
	// NormalizePoint 转成统一测点模型。
	NormalizePoint(p DevicePoint) NormalizedPoint
	// NormalizeAlarm 转成统一告警模型。
	NormalizeAlarm(a alarmdomain.DeviceAlarm) NormalizedAlarm
	// NormalizePointWithLang 根据语言偏好调整展示文本。
	NormalizePointWithLang(p DevicePoint, isEnglish bool) NormalizedPoint
	// NormalizeAlarmWithLang 根据语言偏好调整展示文本。
	NormalizeAlarmWithLang(a alarmdomain.DeviceAlarm, isEnglish bool) NormalizedAlarm
}

var (
	normalizerRegistry map[string]PointNormalizer
	normalizerOnce     sync.Once
)

func initNormalizerRegistry() { normalizerRegistry = make(map[string]PointNormalizer) }

// RegisterNormalizer 注册模板类型对应的归一化器。
func RegisterNormalizer(n PointNormalizer) {
	normalizerOnce.Do(initNormalizerRegistry)
	normalizerRegistry[n.Type()] = n
}

// GetNormalizer 按模板类型获取归一化器。
func GetNormalizer(typ string) (PointNormalizer, bool) {
	normalizerOnce.Do(initNormalizerRegistry)
	n, ok := normalizerRegistry[typ]
	return n, ok
}

// NormalizePointsWithLang 将模板测点转换成前端可展示的统一结构。
func NormalizePointsWithLang(typ string, points []DevicePoint, isEnglish bool) []NormalizedPoint {
	normalizer, ok := GetNormalizer(typ)
	if !ok {
		normalizer = &defaultNormalizer{}
	}
	result := make([]NormalizedPoint, len(points))
	for i, p := range points {
		result[i] = normalizer.NormalizePointWithLang(p, isEnglish)
	}
	return result
}

// NormalizeAlarmsWithLang 将模板告警转换成前端可展示的统一结构。
func NormalizeAlarmsWithLang(typ string, alarms []alarmdomain.DeviceAlarm, isEnglish bool) []NormalizedAlarm {
	normalizer, ok := GetNormalizer(typ)
	if !ok {
		normalizer = &defaultNormalizer{}
	}
	result := make([]NormalizedAlarm, len(alarms))
	for i, a := range alarms {
		result[i] = normalizer.NormalizeAlarmWithLang(a, isEnglish)
	}
	return result
}

type modbusNormalizer struct{}

func (m *modbusNormalizer) Type() string { return "modbus" }
func (m *modbusNormalizer) NormalizePoint(p DevicePoint) NormalizedPoint {
	registerAddr := 0
	hasRegisterAddr := false
	if p.Modbus != nil {
		registerAddr = p.Modbus.RegisterAddress
		hasRegisterAddr = true
	}
	np := baseNormalizedPoint(p, strconv.Itoa(registerAddr), registerAddr, "")
	np.HasRegisterAddr = hasRegisterAddr
	return np
}
func (m *modbusNormalizer) NormalizeAlarm(a alarmdomain.DeviceAlarm) NormalizedAlarm {
	return normalizeAlarmCommon(a)
}
func (m *modbusNormalizer) NormalizePointWithLang(p DevicePoint, isEnglish bool) NormalizedPoint {
	return adjustPointLang(m.NormalizePoint(p), p, isEnglish)
}
func (m *modbusNormalizer) NormalizeAlarmWithLang(a alarmdomain.DeviceAlarm, isEnglish bool) NormalizedAlarm {
	return adjustAlarmLang(m.NormalizeAlarm(a), a, isEnglish)
}

type snmpNormalizer struct{}

func (s *snmpNormalizer) Type() string { return "snmp" }
func (s *snmpNormalizer) NormalizePoint(p DevicePoint) NormalizedPoint {
	oid := ""
	if p.SNMP != nil {
		oid = p.SNMP.OID
	}
	return baseNormalizedPoint(p, oid, 0, oid)
}
func (s *snmpNormalizer) NormalizeAlarm(a alarmdomain.DeviceAlarm) NormalizedAlarm {
	return normalizeAlarmCommon(a)
}
func (s *snmpNormalizer) NormalizePointWithLang(p DevicePoint, isEnglish bool) NormalizedPoint {
	return adjustPointLang(s.NormalizePoint(p), p, isEnglish)
}
func (s *snmpNormalizer) NormalizeAlarmWithLang(a alarmdomain.DeviceAlarm, isEnglish bool) NormalizedAlarm {
	return adjustAlarmLang(s.NormalizeAlarm(a), a, isEnglish)
}

type localIONormalizer struct{ typ string }

func (l *localIONormalizer) Type() string { return l.typ }
func (l *localIONormalizer) NormalizePoint(p DevicePoint) NormalizedPoint {
	channel := ""
	if p.LocalIO != nil {
		channel = p.LocalIO.Channel
	}
	return baseNormalizedPoint(p, channel, 0, "")
}
func (l *localIONormalizer) NormalizeAlarm(a alarmdomain.DeviceAlarm) NormalizedAlarm {
	return normalizeAlarmCommon(a)
}
func (l *localIONormalizer) NormalizePointWithLang(p DevicePoint, isEnglish bool) NormalizedPoint {
	return adjustPointLang(l.NormalizePoint(p), p, isEnglish)
}
func (l *localIONormalizer) NormalizeAlarmWithLang(a alarmdomain.DeviceAlarm, isEnglish bool) NormalizedAlarm {
	return adjustAlarmLang(l.NormalizeAlarm(a), a, isEnglish)
}

type defaultNormalizer struct{}

func (d *defaultNormalizer) Type() string { return "default" }
func (d *defaultNormalizer) NormalizePoint(p DevicePoint) NormalizedPoint {
	return baseNormalizedPoint(p, "", 0, "")
}
func (d *defaultNormalizer) NormalizeAlarm(a alarmdomain.DeviceAlarm) NormalizedAlarm {
	return normalizeAlarmCommon(a)
}
func (d *defaultNormalizer) NormalizePointWithLang(p DevicePoint, isEnglish bool) NormalizedPoint {
	return adjustPointLang(d.NormalizePoint(p), p, isEnglish)
}
func (d *defaultNormalizer) NormalizeAlarmWithLang(a alarmdomain.DeviceAlarm, isEnglish bool) NormalizedAlarm {
	return adjustAlarmLang(d.NormalizeAlarm(a), a, isEnglish)
}

func baseNormalizedPoint(p DevicePoint, addr string, registerAddr int, oid string) NormalizedPoint {
	return NormalizedPoint{
		PointID:        p.PointID,
		Type:           p.Type,
		Name:           p.Name,
		NameEN:         p.NameEN,
		DataType:       p.DataType,
		Unit:           p.Unit,
		UnitEN:         p.UnitEN,
		Coefficient:    p.Coefficient,
		Precision:      p.Precision,
		Access:         p.Access,
		Addr:           addr,
		OID:            oid,
		RegisterAddr:   registerAddr,
		Expression:     p.Expression,
		ValueMappingCN: p.ValueMappingCN,
		ValueMappingEN: p.ValueMappingEN,
		Enabled:        p.Enabled,
		Description:    p.Description,
	}
}

func adjustPointLang(np NormalizedPoint, p DevicePoint, isEnglish bool) NormalizedPoint {
	if !isEnglish {
		return np
	}
	if p.NameEN != "" {
		np.Name = p.NameEN
	}
	if p.UnitEN != "" {
		np.Unit = p.UnitEN
	}
	if p.ValueMappingEN != "" {
		np.ValueMappingCN = p.ValueMappingEN
	}
	return np
}

func normalizeAlarmCommon(a alarmdomain.DeviceAlarm) NormalizedAlarm {
	na := NormalizedAlarm{
		PointID:     a.PointID,
		Name:        a.Name,
		NameEN:      a.NameEN,
		Level:       a.Level,
		Enabled:     a.Enabled,
		StartDelay:  a.StartDelay,
		EndDelay:    a.EndDelay,
		Description: a.Description,
	}
	if a.StartLowLimit != nil {
		na.StartLowLimit = *a.StartLowLimit
	}
	if a.StartHighLimit != nil {
		na.StartHighLimit = *a.StartHighLimit
	}
	if a.EndLowLimit != nil {
		na.EndLowLimit = *a.EndLowLimit
	}
	if a.EndHighLimit != nil {
		na.EndHighLimit = *a.EndHighLimit
	}
	return na
}

func adjustAlarmLang(na NormalizedAlarm, a alarmdomain.DeviceAlarm, isEnglish bool) NormalizedAlarm {
	if isEnglish && a.NameEN != "" {
		na.Name = a.NameEN
	}
	return na
}

func init() {
	RegisterNormalizer(&modbusNormalizer{})
	RegisterNormalizer(&snmpNormalizer{})
	RegisterNormalizer(&localIONormalizer{typ: "local_io"})
	RegisterNormalizer(&localIONormalizer{typ: "local_di"})
	RegisterNormalizer(&localIONormalizer{typ: "local_do"})
}
