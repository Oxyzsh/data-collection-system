package modbus

import "data-collection-system/internal/valueobj"

// 编码顺序常量
const (
	EncodingABCD = "ABCD" // Big-Endian（大端序）
	EncodingCDAB = "CDAB" // 字交换大端
	EncodingBADC = "BADC" // 字节交换大端
	EncodingDCBA = "DCBA" // Little-Endian（小端序）
)

// Modbus 功能码
const (
	FuncCodeReadCoils              = 0x01 // 读线圈
	FuncCodeReadDiscreteInputs     = 0x02 // 读离散输入
	FuncCodeReadHoldingRegisters   = 0x03 // 读保持寄存器
	FuncCodeReadInputRegisters     = 0x04 // 读输入寄存器
	FuncCodeWriteSingleCoil        = 0x05 // 写单个线圈
	FuncCodeWriteSingleRegister    = 0x06 // 写单个寄存器
	FuncCodeWriteMultipleCoils     = 0x0F // 写多个线圈
	FuncCodeWriteMultipleRegisters = 0x10 // 写多个寄存器
)

// TypeRegisterCount 返回数据类型对应的寄存器数量
func TypeRegisterCount(dataType string) int {
	switch dataType {
	case valueobj.RawTypeINT16, valueobj.RawTypeUINT16, valueobj.RawTypeBOOL:
		return 1
	case valueobj.RawTypeINT32, valueobj.RawTypeUINT32, valueobj.RawTypeFLOAT32:
		return 2
	case valueobj.RawTypeINT64, valueobj.RawTypeUINT64, valueobj.RawTypeFLOAT64:
		return 4
	default:
		return 1
	}
}

// TypeDefaultFuncCode 返回数据类型对应的默认读功能码
func TypeDefaultFuncCode(dataType string) int {
	if dataType == valueobj.RawTypeBOOL {
		return FuncCodeReadCoils
	}
	return FuncCodeReadHoldingRegisters
}

// TypeDefaultEncodingOrder 返回数据类型对应的默认编码顺序
func TypeDefaultEncodingOrder(dataType string) string {
	switch dataType {
	case valueobj.RawTypeINT16, valueobj.RawTypeUINT16, valueobj.RawTypeBOOL, valueobj.RawTypeSTRING:
		return ""
	default:
		return EncodingABCD
	}
}
