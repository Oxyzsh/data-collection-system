package domain

import (
	"fmt"
	"strings"

	snmpproto "data-collection-system/internal/protocol/snmp"
	"data-collection-system/internal/valueobj"
)

func ValidatePointsForTemplateType(templateType string, points []DevicePoint) error {
	for _, point := range points {
		if err := ValidatePointForTemplateType(templateType, point); err != nil {
			return err
		}
	}
	return nil
}

// ValidatePointForTemplateType 按模板类型校验单个测点。
// 这一步相当于模板入库前的语义检查，防止后面解析链路拿到脏数据。
func ValidatePointForTemplateType(templateType string, point DevicePoint) error {
	if point.PointID <= 0 {
		return fmt.Errorf("测点 PointID 必须大于 0")
	}
	if point.Type == "" {
		point.Type = PointTypePOINT
	}
	if point.DataType == "" {
		return fmt.Errorf("测点[%d:%s] 最终数据类型不能为空", point.PointID, point.Name)
	}
	if valueobj.NormalizeDataType(point.DataType) == "" {
		return fmt.Errorf("测点[%d:%s] 最终数据类型不受支持：%s", point.PointID, point.Name, point.DataType)
	}

	switch strings.ToLower(strings.TrimSpace(templateType)) {
	case "modbus":
		return validateModbusPoint(point)
	case "snmp":
		return validateSNMPPoint(point)
	case "local_io", "local_di", "local_do":
		return validateLocalIOPoint(strings.ToLower(strings.TrimSpace(templateType)), point)
	default:
		return validateGenericPoint(point)
	}
}

// validateGenericPoint 校验通用模板测点。
func validateGenericPoint(point DevicePoint) error {
	if point.Modbus != nil && point.SNMP != nil {
		return fmt.Errorf("测点[%d:%s] 不允许同时包含 Modbus 和 SNMP 专用配置", point.PointID, point.Name)
	}
	return nil
}

func validateLocalIOPoint(templateType string, point DevicePoint) error {
	if point.Modbus != nil || point.SNMP != nil {
		return fmt.Errorf("测点[%d:%s] 本地 IO 模板不允许包含 Modbus 或 SNMP 配置", point.PointID, point.Name)
	}
	if point.Type == PointTypeMETRIC {
		return fmt.Errorf("测点[%d:%s] 本地 IO 模板不支持 METRIC 点", point.PointID, point.Name)
	}
	if point.LocalIO == nil || strings.TrimSpace(point.LocalIO.Channel) == "" {
		return fmt.Errorf("测点[%d:%s] 缺少通道名称", point.PointID, point.Name)
	}
	if valueobj.NormalizeDataType(point.DataType) != valueobj.DataTypeBOOL {
		return fmt.Errorf("测点[%d:%s] 本地 IO 测点最终类型必须为 BOOL", point.PointID, point.Name)
	}
	channel := strings.ToUpper(strings.TrimSpace(point.LocalIO.Channel))
	access := strings.ToLower(strings.TrimSpace(point.Access))
	if templateType == "local_di" {
		if access != "r" {
			return fmt.Errorf("测点[%d:%s] DI 模板只允许只读", point.PointID, point.Name)
		}
		if channel != "AUTO" && !strings.HasPrefix(channel, "DI") {
			return fmt.Errorf("测点[%d:%s] DI 模板通道名必须以 DI 开头", point.PointID, point.Name)
		}
		return nil
	}
	if templateType == "local_do" {
		if access != "w" && access != "rw" {
			return fmt.Errorf("测点[%d:%s] DO 模板必须可写", point.PointID, point.Name)
		}
		if channel != "AUTO" && !strings.HasPrefix(channel, "DO") {
			return fmt.Errorf("测点[%d:%s] DO 模板通道名必须以 DO 开头", point.PointID, point.Name)
		}
		return nil
	}
	if channel == "AUTO" {
		if access != "rw" {
			return fmt.Errorf("测点[%d:%s] 动态 DIDO 通道必须读写", point.PointID, point.Name)
		}
		return nil
	}
	switch {
	case strings.HasPrefix(channel, "DI"):
		if access != "r" {
			return fmt.Errorf("测点[%d:%s] DI 通道只允许只读", point.PointID, point.Name)
		}
	case strings.HasPrefix(channel, "DO"):
		if access != "w" && access != "rw" {
			return fmt.Errorf("测点[%d:%s] DO 通道必须可写", point.PointID, point.Name)
		}
	default:
		return fmt.Errorf("测点[%d:%s] 通道名称必须以 DI 或 DO 开头", point.PointID, point.Name)
	}
	return nil
}

// validateModbusPoint 校验 Modbus 模板测点。
func validateModbusPoint(point DevicePoint) error {
	if point.SNMP != nil {
		return fmt.Errorf("测点[%d:%s] Modbus 模板不允许包含 SNMP 专用配置", point.PointID, point.Name)
	}
	if point.Type == PointTypeMETRIC {
		if point.Modbus != nil {
			return fmt.Errorf("测点[%d:%s] Modbus 指标点不应包含寄存器配置", point.PointID, point.Name)
		}
		return nil
	}
	if point.Modbus == nil {
		return fmt.Errorf("测点[%d:%s] 缺少 Modbus 专用配置", point.PointID, point.Name)
	}
	if valueobj.NormalizeRawType(point.OriginalType) == "" {
		return fmt.Errorf("测点[%d:%s] 原始数据类型不受支持：%s", point.PointID, point.Name, point.OriginalType)
	}
	if point.Modbus.RegisterAddress < 0 {
		return fmt.Errorf("测点[%d:%s] 寄存器地址不能小于 0", point.PointID, point.Name)
	}
	return nil
}

// validateSNMPPoint 校验 SNMP 模板测点。
func validateSNMPPoint(point DevicePoint) error {
	if point.Modbus != nil {
		return fmt.Errorf("测点[%d:%s] SNMP 模板不允许包含 Modbus 专用配置", point.PointID, point.Name)
	}
	if point.Type == PointTypeMETRIC {
		if point.SNMP != nil {
			return fmt.Errorf("测点[%d:%s] SNMP 指标点不应包含 OID 配置", point.PointID, point.Name)
		}
		return nil
	}
	if point.SNMP == nil {
		return fmt.Errorf("测点[%d:%s] 缺少 SNMP 专用配置", point.PointID, point.Name)
	}
	if strings.TrimSpace(point.SNMP.OID) == "" {
		return fmt.Errorf("测点[%d:%s] OID 不能为空", point.PointID, point.Name)
	}
	if snmpproto.NormalizeOriginalType(point.OriginalType) == "" {
		return fmt.Errorf("测点[%d:%s] 原始数据类型不受支持：%s", point.PointID, point.Name, point.OriginalType)
	}
	return nil
}
