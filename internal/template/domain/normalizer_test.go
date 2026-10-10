package domain

import "testing"

// TestNormalizeModbusRegisterAddressZero 回归模板预览把 0 号寄存器显示成“-”的问题：
// MODBUS 的 0 号寄存器是合法地址，必须用 HasRegisterAddr 和“无寄存器配置”区分开。
func TestNormalizeModbusRegisterAddressZero(t *testing.T) {
	points := []DevicePoint{
		{
			PointID: 1,
			Type:    PointTypePOINT,
			Name:    "温度",
			Modbus:  &ModbusPointSpec{RegisterAddress: 0, RegisterCount: 1, ReadFuncCode: 3},
		},
		{
			// 模板缓存注入的内置点没有协议配置，不能显示成 0 号寄存器。
			PointID: 0,
			Type:    PointTypePOINT,
			Name:    "设备在线状态",
		},
	}

	got := NormalizePointsWithLang("modbus", points, false)
	if !got[0].HasRegisterAddr || got[0].RegisterAddr != 0 {
		t.Fatalf("0 号寄存器应保留: has_register_addr=%v register_addr=%d", got[0].HasRegisterAddr, got[0].RegisterAddr)
	}
	if got[1].HasRegisterAddr {
		t.Fatalf("无 MODBUS 配置的内置点不应标记 has_register_addr")
	}
}

func TestNormalizeSNMPPointHasNoRegisterAddress(t *testing.T) {
	points := []DevicePoint{
		{PointID: 1, Type: PointTypePOINT, Name: "CPU", SNMP: &SNMPPointSpec{OID: "1.3.6.1.2.1.1.3.0"}},
	}

	got := NormalizePointsWithLang("snmp", points, false)
	if got[0].HasRegisterAddr {
		t.Fatalf("SNMP 测点不应标记 has_register_addr")
	}
	if got[0].OID != "1.3.6.1.2.1.1.3.0" {
		t.Fatalf("SNMP OID 应保留: %q", got[0].OID)
	}
}
