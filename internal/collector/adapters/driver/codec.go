package collector

import (
	"encoding/binary"
	"fmt"
	"math"

	modbusspec "data-collection-system/internal/protocol/modbus"
	"data-collection-system/internal/valueobj"
)

// DecodeRegister 将 Modbus 寄存器原始字节解码为 float64。
// 调用方通常会先根据测点配置拿到 dataType 和编码顺序，再调用这个函数。
func DecodeRegister(data []byte, dataType string, encodingOrder string) (float64, error) {
	if len(data) == 0 {
		return 0, fmt.Errorf("寄存器数据为空")
	}

	// 多寄存器值需要先按配置重排字节，再按具体类型解释。
	reordered := reorderBytes(data, encodingOrder)

	switch dataType {
	case valueobj.RawTypeBOOL:
		return decodeBool(data), nil
	case valueobj.RawTypeINT16:
		return float64(int16(binary.BigEndian.Uint16(reordered))), nil
	case valueobj.RawTypeUINT16:
		return float64(binary.BigEndian.Uint16(reordered)), nil
	case valueobj.RawTypeINT32:
		if len(reordered) < 4 {
			return 0, fmt.Errorf("INT32 需要至少 4 字节，实际 %d", len(reordered))
		}
		return float64(int32(binary.BigEndian.Uint32(reordered))), nil
	case valueobj.RawTypeUINT32:
		if len(reordered) < 4 {
			return 0, fmt.Errorf("UINT32 需要至少 4 字节，实际 %d", len(reordered))
		}
		return float64(binary.BigEndian.Uint32(reordered)), nil
	case valueobj.RawTypeINT64:
		if len(reordered) < 8 {
			return 0, fmt.Errorf("INT64 需要至少 8 字节，实际 %d", len(reordered))
		}
		return float64(int64(binary.BigEndian.Uint64(reordered))), nil
	case valueobj.RawTypeUINT64:
		if len(reordered) < 8 {
			return 0, fmt.Errorf("UINT64 需要至少 8 字节，实际 %d", len(reordered))
		}
		return float64(binary.BigEndian.Uint64(reordered)), nil
	case valueobj.RawTypeFLOAT32:
		if len(reordered) < 4 {
			return 0, fmt.Errorf("FLOAT32 需要至少 4 字节，实际 %d", len(reordered))
		}
		bits := binary.BigEndian.Uint32(reordered)
		return float64(math.Float32frombits(bits)), nil
	case valueobj.RawTypeFLOAT64:
		if len(reordered) < 8 {
			return 0, fmt.Errorf("FLOAT64 需要至少 8 字节，实际 %d", len(reordered))
		}
		bits := binary.BigEndian.Uint64(reordered)
		return math.Float64frombits(bits), nil
	default:
		// 默认按 UINT16 处理
		return float64(binary.BigEndian.Uint16(reordered)), nil
	}
}

// encodeToRegisters 将字节序列按编码顺序转换为 Modbus 寄存器值列表。
// 这是 reorderBytes 在写路径中的逆用：先将值编码为自然序（大端）字节，
// 再按 encodingOrder 重排为线路线序，最后以 []uint16 返回以便批量写入。
func encodeToRegisters(data []byte, encodingOrder string) []uint16 {
	reordered := reorderBytes(data, encodingOrder)
	registers := make([]uint16, len(reordered)/2)
	for i := range registers {
		registers[i] = binary.BigEndian.Uint16(reordered[i*2 : i*2+2])
	}
	return registers
}

// decodeBool 解析布尔值（线圈读取）。
func decodeBool(data []byte) float64 {
	for _, b := range data {
		if b != 0 {
			return 1
		}
	}
	return 0
}

// reorderBytes 根据编码顺序重排字节。
// Modbus 寄存器本身按大端解释，这里处理的是多寄存器拼接后的顺序。
func reorderBytes(data []byte, encodingOrder string) []byte {
	if len(data) < 4 || encodingOrder == "" || encodingOrder == modbusspec.EncodingABCD {
		return data // ABCD 无需重排
	}

	buf := make([]byte, len(data))
	copy(buf, data)

	switch encodingOrder {
	case modbusspec.EncodingCDAB:
		// 字/双字交换：按寄存器对（4 字节）为单位前后互换
		if len(buf) >= 8 && len(buf)%8 == 0 {
			// 8 字节及以上且为 8 的整数倍：以 8 字节为单位做全宽度字交换
			for i := 0; i < len(buf); i += 8 {
				var tmp [4]byte
				copy(tmp[:], buf[i:i+4])
				copy(buf[i:i+4], buf[i+4:i+8])
				copy(buf[i+4:i+8], tmp[:])
			}
		} else {
			// 4 字节数据：两个寄存器位置互换
			buf[0], buf[2] = buf[2], buf[0]
			buf[1], buf[3] = buf[3], buf[1]
		}
	case modbusspec.EncodingBADC:
		// 字节交换：每个寄存器内部 2 字节互换
		for i := 0; i+1 < len(buf); i += 2 {
			buf[i], buf[i+1] = buf[i+1], buf[i]
		}
	case modbusspec.EncodingDCBA:
		// 完全反转：整个字节序列逆序
		for i, j := 0, len(buf)-1; i < j; i, j = i+1, j-1 {
			buf[i], buf[j] = buf[j], buf[i]
		}
	}

	return buf
}
