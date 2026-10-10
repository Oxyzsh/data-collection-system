package collector

import (
	"testing"

	"github.com/simonvetter/modbus"
	modbusspec "data-collection-system/internal/protocol/modbus"
	templatebiz "data-collection-system/internal/template/domain"
	"data-collection-system/internal/valueobj"
)

func TestPlanModbusReadWindows_MergeAdjacentRegisters(t *testing.T) {
	points := []templatebiz.DevicePoint{
		testModbusPoint(1, 10, 1, modbusspec.FuncCodeReadHoldingRegisters, valueobj.RawTypeUINT16),
		testModbusPoint(2, 11, 1, modbusspec.FuncCodeReadHoldingRegisters, valueobj.RawTypeUINT16),
	}

	windows := planModbusReadWindows(points, 100)
	if len(windows) != 1 {
		t.Fatalf("expected 1 window, got %d", len(windows))
	}
	if windows[0].startAddress != 10 {
		t.Fatalf("expected window start 10, got %d", windows[0].startAddress)
	}
	if windows[0].readCount != 2 {
		t.Fatalf("expected window count 2, got %d", windows[0].readCount)
	}
	if len(windows[0].points) != 2 {
		t.Fatalf("expected 2 points in window, got %d", len(windows[0].points))
	}
}

func TestPlanModbusReadWindows_MergeGapWithinLimit(t *testing.T) {
	points := []templatebiz.DevicePoint{
		testModbusPoint(1, 10, 1, modbusspec.FuncCodeReadHoldingRegisters, valueobj.RawTypeUINT16),
		testModbusPoint(2, 20, 1, modbusspec.FuncCodeReadHoldingRegisters, valueobj.RawTypeUINT16),
	}

	windows := planModbusReadWindows(points, 100)
	if len(windows) != 1 {
		t.Fatalf("expected 1 window, got %d", len(windows))
	}
	if windows[0].readCount != 11 {
		t.Fatalf("expected window count 11, got %d", windows[0].readCount)
	}
}

func TestPlanModbusReadWindows_SplitBeyondLimit(t *testing.T) {
	points := []templatebiz.DevicePoint{
		testModbusPoint(1, 10, 1, modbusspec.FuncCodeReadHoldingRegisters, valueobj.RawTypeUINT16),
		testModbusPoint(2, 120, 1, modbusspec.FuncCodeReadHoldingRegisters, valueobj.RawTypeUINT16),
	}

	windows := planModbusReadWindows(points, 100)
	if len(windows) != 2 {
		t.Fatalf("expected 2 windows, got %d", len(windows))
	}
	if windows[0].startAddress != 10 || windows[1].startAddress != 120 {
		t.Fatalf("unexpected window starts: %d, %d", windows[0].startAddress, windows[1].startAddress)
	}
}

func TestPlanModbusReadWindows_RegistersRespectProtocolMax(t *testing.T) {
	points := []templatebiz.DevicePoint{
		testModbusPoint(1, 10, 1, modbusspec.FuncCodeReadHoldingRegisters, valueobj.RawTypeUINT16),
		testModbusPoint(2, 140, 1, modbusspec.FuncCodeReadHoldingRegisters, valueobj.RawTypeUINT16),
	}

	windows := planModbusReadWindows(points, 2000)
	if len(windows) != 2 {
		t.Fatalf("expected 2 windows due to protocol max, got %d", len(windows))
	}
}

func TestPlanModbusReadWindows_CoilAndDiscreteFollowSameLimitRule(t *testing.T) {
	coilPoints := []templatebiz.DevicePoint{
		testModbusPoint(1, 1, 1, modbusspec.FuncCodeReadCoils, valueobj.RawTypeBOOL),
		testModbusPoint(2, 80, 1, modbusspec.FuncCodeReadCoils, valueobj.RawTypeBOOL),
	}
	coilWindows := planModbusReadWindows(coilPoints, 100)
	if len(coilWindows) != 1 {
		t.Fatalf("expected 1 coil window, got %d", len(coilWindows))
	}

	discretePoints := []templatebiz.DevicePoint{
		testModbusPoint(3, 1, 1, modbusspec.FuncCodeReadDiscreteInputs, valueobj.RawTypeBOOL),
		testModbusPoint(4, 120, 1, modbusspec.FuncCodeReadDiscreteInputs, valueobj.RawTypeBOOL),
	}
	discreteWindows := planModbusReadWindows(discretePoints, 100)
	if len(discreteWindows) != 2 {
		t.Fatalf("expected 2 discrete windows, got %d", len(discreteWindows))
	}
}

func TestPlanModbusReadWindows_DifferentFuncCodesDoNotMerge(t *testing.T) {
	points := []templatebiz.DevicePoint{
		testModbusPoint(1, 10, 1, modbusspec.FuncCodeReadHoldingRegisters, valueobj.RawTypeUINT16),
		testModbusPoint(2, 11, 1, modbusspec.FuncCodeReadInputRegisters, valueobj.RawTypeUINT16),
	}

	windows := planModbusReadWindows(points, 100)
	if len(windows) != 2 {
		t.Fatalf("expected 2 windows, got %d", len(windows))
	}
	if windows[0].readFuncCode == windows[1].readFuncCode {
		t.Fatalf("expected different function codes, got %d and %d", windows[0].readFuncCode, windows[1].readFuncCode)
	}
}

func testModbusPoint(pointID, addr, count, readFuncCode int, originalType string) templatebiz.DevicePoint {
	return templatebiz.DevicePoint{
		PointID:      pointID,
		Type:         templatebiz.PointTypePOINT,
		Name:         "point",
		OriginalType: originalType,
		Modbus: &templatebiz.ModbusPointSpec{
			RegisterAddress: addr,
			RegisterCount:   count,
			ReadFuncCode:    readFuncCode,
		},
	}
}

func TestBuildModbusReadPoint_RegistersDefaultEncodingAndType(t *testing.T) {
	point := testModbusPoint(1, 100, 0, 0, "")
	spec, ok := buildModbusReadPoint(point)
	if !ok {
		t.Fatal("expected point to be readable")
	}
	if spec.readFuncCode != modbusspec.FuncCodeReadHoldingRegisters {
		t.Fatalf("expected default holding register function code, got %d", spec.readFuncCode)
	}
	if spec.regType != modbus.HOLDING_REGISTER {
		t.Fatalf("expected holding register type, got %v", spec.regType)
	}
	if spec.encodingOrder != modbusspec.EncodingABCD {
		t.Fatalf("expected default encoding order ABCD, got %q", spec.encodingOrder)
	}
	if spec.registerCount != 1 {
		t.Fatalf("expected default register count 1, got %d", spec.registerCount)
	}
}
