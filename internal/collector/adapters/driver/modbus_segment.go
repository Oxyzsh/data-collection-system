package collector

import (
	"sort"

	"github.com/simonvetter/modbus"
	modbusspec "data-collection-system/internal/protocol/modbus"
	templatebiz "data-collection-system/internal/template/domain"
	"data-collection-system/internal/valueobj"
)

type modbusWindowKey struct {
	readFuncCode int
	regType      modbus.RegType
}

type modbusReadPoint struct {
	point         templatebiz.DevicePoint
	startAddress  uint16
	registerCount uint16
	readFuncCode  int
	regType       modbus.RegType
	dataType      string
	encodingOrder string
}

type modbusReadWindow struct {
	readFuncCode int
	regType      modbus.RegType
	startAddress uint16
	readCount    uint16
	points       []modbusReadPoint
}

const (
	modbusMaxReadCoilsOrInputs = 2000
	modbusMaxReadRegisters     = 125
)

func planModbusReadWindows(points []templatebiz.DevicePoint, maxContinuousCount int) []modbusReadWindow {
	if maxContinuousCount < 1 {
		maxContinuousCount = 1
	}

	candidates := make([]modbusReadPoint, 0, len(points))
	for _, point := range points {
		candidate, ok := buildModbusReadPoint(point)
		if !ok {
			continue
		}
		candidates = append(candidates, candidate)
	}

	windows := make([]modbusReadWindow, 0)

	grouped := make(map[modbusWindowKey][]modbusReadPoint)
	for _, candidate := range candidates {
		key := modbusWindowKey{readFuncCode: candidate.readFuncCode, regType: candidate.regType}
		grouped[key] = append(grouped[key], candidate)
	}

	for _, group := range grouped {
		effectiveMax := minModbusContinuousLimit(maxContinuousCount, modbusReadFuncMaxCount(group[0].readFuncCode))
		sort.SliceStable(group, func(i, j int) bool {
			if group[i].startAddress != group[j].startAddress {
				return group[i].startAddress < group[j].startAddress
			}
			return group[i].point.PointID < group[j].point.PointID
		})

		for _, candidate := range group {
			if len(windows) == 0 || windows[len(windows)-1].readFuncCode != candidate.readFuncCode || windows[len(windows)-1].regType != candidate.regType || !canMergeModbusWindow(windows[len(windows)-1], candidate, effectiveMax) {
				windows = append(windows, newModbusReadWindow(candidate))
				continue
			}
			windows[len(windows)-1] = mergeModbusWindow(windows[len(windows)-1], candidate)
		}
	}

	sort.SliceStable(windows, func(i, j int) bool {
		if windows[i].startAddress != windows[j].startAddress {
			return windows[i].startAddress < windows[j].startAddress
		}
		if windows[i].readFuncCode != windows[j].readFuncCode {
			return windows[i].readFuncCode < windows[j].readFuncCode
		}
		if windows[i].regType != windows[j].regType {
			return windows[i].regType < windows[j].regType
		}
		return len(windows[i].points) < len(windows[j].points)
	})

	return windows
}

func modbusReadFuncMaxCount(readFuncCode int) int {
	switch readFuncCode {
	case modbusspec.FuncCodeReadCoils, modbusspec.FuncCodeReadDiscreteInputs:
		return modbusMaxReadCoilsOrInputs
	case modbusspec.FuncCodeReadHoldingRegisters, modbusspec.FuncCodeReadInputRegisters:
		return modbusMaxReadRegisters
	default:
		return modbusMaxReadRegisters
	}
}

func minModbusContinuousLimit(requested, protocolMax int) int {
	if requested < 1 {
		requested = 1
	}
	if protocolMax < 1 {
		protocolMax = 1
	}
	if requested < protocolMax {
		return requested
	}
	return protocolMax
}

func buildModbusReadPoint(point templatebiz.DevicePoint) (modbusReadPoint, bool) {
	if point.PointID <= 0 || point.Modbus == nil || point.Modbus.RegisterAddress < 0 {
		return modbusReadPoint{}, false
	}

	dataType := point.OriginalType
	if dataType == "" {
		dataType = valueobj.RawTypeUINT16
	}

	readFuncCode := point.Modbus.ReadFuncCode
	if readFuncCode == 0 {
		readFuncCode = modbusspec.TypeDefaultFuncCode(dataType)
	}

	registerCount := point.Modbus.RegisterCount
	if registerCount == 0 {
		registerCount = modbusspec.TypeRegisterCount(dataType)
	}
	if registerCount < 1 {
		registerCount = 1
	}

	regType := modbus.RegType(0)
	switch readFuncCode {
	case modbusspec.FuncCodeReadHoldingRegisters:
		regType = modbus.HOLDING_REGISTER
	case modbusspec.FuncCodeReadInputRegisters:
		regType = modbus.INPUT_REGISTER
	}

	encodingOrder := point.Modbus.EncodingOrder
	if encodingOrder == "" {
		encodingOrder = modbusspec.EncodingABCD
	}

	return modbusReadPoint{
		point:         point,
		startAddress:  uint16(point.Modbus.RegisterAddress),
		registerCount: uint16(registerCount),
		readFuncCode:  readFuncCode,
		regType:       regType,
		dataType:      dataType,
		encodingOrder: encodingOrder,
	}, true
}

func newModbusReadWindow(point modbusReadPoint) modbusReadWindow {
	return modbusReadWindow{
		readFuncCode: point.readFuncCode,
		regType:      point.regType,
		startAddress: point.startAddress,
		readCount:    point.registerCount,
		points:       []modbusReadPoint{point},
	}
}

func canMergeModbusWindow(window modbusReadWindow, point modbusReadPoint, maxContinuousCount int) bool {
	if window.readFuncCode != point.readFuncCode || window.regType != point.regType {
		return false
	}

	newStart := uint32(window.startAddress)
	windowEnd := uint32(window.startAddress) + uint32(window.readCount)
	pointEnd := uint32(point.startAddress) + uint32(point.registerCount)
	if pointEnd > windowEnd {
		windowEnd = pointEnd
	}

	return windowEnd-newStart <= uint32(maxContinuousCount)
}

func mergeModbusWindow(window modbusReadWindow, point modbusReadPoint) modbusReadWindow {
	windowEnd := uint32(window.startAddress) + uint32(window.readCount)
	pointEnd := uint32(point.startAddress) + uint32(point.registerCount)
	if pointEnd > windowEnd {
		window.readCount = uint16(pointEnd - uint32(window.startAddress))
	}
	window.points = append(window.points, point)
	return window
}
