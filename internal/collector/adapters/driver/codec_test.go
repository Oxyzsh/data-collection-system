package collector

import (
	"bytes"
	"math"
	"testing"

	modbusspec "data-collection-system/internal/protocol/modbus"
	"data-collection-system/internal/valueobj"
)

// ---------------------------------------------------------------------------
// reorderBytes 单元测试：4 字节（2 寄存器）
// ---------------------------------------------------------------------------

func TestReorderBytes_4Byte_ABCD(t *testing.T) {
	data := []byte{0x41, 0x42, 0x43, 0x44} // ABCD
	result := reorderBytes(data, modbusspec.EncodingABCD)
	if !bytes.Equal(result, []byte{0x41, 0x42, 0x43, 0x44}) {
		t.Errorf("ABCD: expected [41 42 43 44], got %v", result)
	}
}

func TestReorderBytes_4Byte_CDAB(t *testing.T) {
	data := []byte{0x41, 0x42, 0x43, 0x44} // ABCD
	result := reorderBytes(data, modbusspec.EncodingCDAB)
	if !bytes.Equal(result, []byte{0x43, 0x44, 0x41, 0x42}) {
		t.Errorf("CDAB(4B): expected [43 44 41 42], got %v", result)
	}
}

func TestReorderBytes_4Byte_BADC(t *testing.T) {
	data := []byte{0x41, 0x42, 0x43, 0x44} // ABCD
	result := reorderBytes(data, modbusspec.EncodingBADC)
	if !bytes.Equal(result, []byte{0x42, 0x41, 0x44, 0x43}) {
		t.Errorf("BADC(4B): expected [42 41 44 43], got %v", result)
	}
}

func TestReorderBytes_4Byte_DCBA(t *testing.T) {
	data := []byte{0x41, 0x42, 0x43, 0x44} // ABCD
	result := reorderBytes(data, modbusspec.EncodingDCBA)
	if !bytes.Equal(result, []byte{0x44, 0x43, 0x42, 0x41}) {
		t.Errorf("DCBA(4B): expected [44 43 42 41], got %v", result)
	}
}

// ---------------------------------------------------------------------------
// reorderBytes 单元测试：8 字节（4 寄存器）—— 方案 A 核心验证
// ---------------------------------------------------------------------------

func TestReorderBytes_8Byte_ABCD(t *testing.T) {
	data := []byte{0xA1, 0xB2, 0xC3, 0xD4, 0xE5, 0xF6, 0xA7, 0xB8}
	result := reorderBytes(data, modbusspec.EncodingABCD)
	if !bytes.Equal(result, data) {
		t.Errorf("ABCD(8B): expected no change, got %v", result)
	}
}

// CDAB 对 8 字节应做双字交换：[A B C D][E F G H] → [E F G H][A B C D]
func TestReorderBytes_8Byte_CDAB(t *testing.T) {
	data := []byte{0xA1, 0xB2, 0xC3, 0xD4, 0xE5, 0xF6, 0xA7, 0xB8}
	result := reorderBytes(data, modbusspec.EncodingCDAB)
	expected := []byte{0xE5, 0xF6, 0xA7, 0xB8, 0xA1, 0xB2, 0xC3, 0xD4}
	if !bytes.Equal(result, expected) {
		t.Errorf("CDAB(8B): expected %v, got %v", expected, result)
	}
}

// BADC 对 8 字节做字节交换：每个寄存器内部 2 字节互换
func TestReorderBytes_8Byte_BADC(t *testing.T) {
	data := []byte{0xA1, 0xB2, 0xC3, 0xD4, 0xE5, 0xF6, 0xA7, 0xB8}
	result := reorderBytes(data, modbusspec.EncodingBADC)
	expected := []byte{0xB2, 0xA1, 0xD4, 0xC3, 0xF6, 0xE5, 0xB8, 0xA7}
	if !bytes.Equal(result, expected) {
		t.Errorf("BADC(8B): expected %v, got %v", expected, result)
	}
}

// DCBA 对 8 字节做完全反转
func TestReorderBytes_8Byte_DCBA(t *testing.T) {
	data := []byte{0xA1, 0xB2, 0xC3, 0xD4, 0xE5, 0xF6, 0xA7, 0xB8}
	result := reorderBytes(data, modbusspec.EncodingDCBA)
	expected := []byte{0xB8, 0xA7, 0xF6, 0xE5, 0xD4, 0xC3, 0xB2, 0xA1}
	if !bytes.Equal(result, expected) {
		t.Errorf("DCBA(8B): expected %v, got %v", expected, result)
	}
}

// ---------------------------------------------------------------------------
// 端到端解码验证：INT64/FLOAT64 + DCBA = 小端序正确解析
// ---------------------------------------------------------------------------

// 验证 INT64 在小端序设备上的正确解码
func TestDecodeRegister_INT64_LittleEndian(t *testing.T) {
	// 值 0x0102030405060708 以大端存储在寄存器中：
	// 寄存器排列为 [01 02][03 04][05 06][07 08]
	// 设备实际字节序为小端，所以读回的原始数据是反转后的
	// 模拟 Modbus 读回的小端原始字节
	data := []byte{0x08, 0x07, 0x06, 0x05, 0x04, 0x03, 0x02, 0x01}
	val, err := DecodeRegister(data, valueobj.RawTypeINT64, modbusspec.EncodingDCBA)
	if err != nil {
		t.Fatalf("INT64 DCBA decode error: %v", err)
	}
	// DCBA 反转后得到 01 02 03 04 05 06 07 08，BigEndian.Uint64 = 0x0102030405060708
	expected := float64(int64(0x0102030405060708))
	if val != expected {
		t.Errorf("INT64 DCBA: expected %d, got %f", int64(expected), val)
	}
}

// 验证 FLOAT64 (double) 在小端序设备上的正确解码
func TestDecodeRegister_FLOAT64_LittleEndian(t *testing.T) {
	// 使用一个确定的 double 值：12345.6789
	expectedFloat := 12345.6789
	bits := math.Float64bits(expectedFloat)

	// 大端字节表示
	beBytes := make([]byte, 8)
	beBytes[0] = byte(bits >> 56)
	beBytes[1] = byte(bits >> 48)
	beBytes[2] = byte(bits >> 40)
	beBytes[3] = byte(bits >> 32)
	beBytes[4] = byte(bits >> 24)
	beBytes[5] = byte(bits >> 16)
	beBytes[6] = byte(bits >> 8)
	beBytes[7] = byte(bits)

	// 设备是小端序，Modbus 读回时字节完全反转
	leBytes := make([]byte, 8)
	for i := 0; i < 8; i++ {
		leBytes[i] = beBytes[7-i]
	}

	val, err := DecodeRegister(leBytes, valueobj.RawTypeFLOAT64, modbusspec.EncodingDCBA)
	if err != nil {
		t.Fatalf("FLOAT64 DCBA decode error: %v", err)
	}
	if math.Abs(val-expectedFloat)/expectedFloat > 1e-9 {
		t.Errorf("FLOAT64 DCBA: expected %f, got %f", expectedFloat, val)
	}
}

// 验证 INT64 在 CDAB 双字交换模式下的解码
func TestDecodeRegister_INT64_CDAB(t *testing.T) {
	// 值 0x0000000100000002 (4294967298)
	// 大端: 00 00 00 01 00 00 00 02
	// CDAB 后: 前4后4互换 → 00 00 00 02 00 00 00 01
	data := []byte{0x00, 0x00, 0x00, 0x02, 0x00, 0x00, 0x00, 0x01}
	val, err := DecodeRegister(data, valueobj.RawTypeINT64, modbusspec.EncodingCDAB)
	if err != nil {
		t.Fatalf("INT64 CDAB decode error: %v", err)
	}
	// CDAB 交换回 00 00 00 01 00 00 00 02
	expected := float64(int64(0x0000000100000002))
	if val != expected {
		t.Errorf("INT64 CDAB: expected %d, got %f", int64(expected), val)
	}
}

// ---------------------------------------------------------------------------
// 边界情况
// ---------------------------------------------------------------------------

func TestReorderBytes_Empty(t *testing.T) {
	result := reorderBytes([]byte{}, modbusspec.EncodingDCBA)
	if len(result) != 0 {
		t.Errorf("empty data should return empty")
	}
}

func TestReorderBytes_ShorterThan4(t *testing.T) {
	data := []byte{0x01, 0x02}
	result := reorderBytes(data, modbusspec.EncodingDCBA)
	if !bytes.Equal(result, data) {
		t.Errorf("<4 bytes: should return unchanged")
	}
}

func TestReorderBytes_NilEncoding(t *testing.T) {
	data := []byte{0x41, 0x42, 0x43, 0x44}
	result := reorderBytes(data, "")
	if !bytes.Equal(result, data) {
		t.Errorf("nil encoding: should return unchanged")
	}
}

// 并发安全测试：swapBuf 不应在并发调用中导致数据竞争
func TestReorderBytes_ConcurrentSafety(t *testing.T) {
	data := []byte{0xA1, 0xB2, 0xC3, 0xD4, 0xE5, 0xF6, 0xA7, 0xB8}
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func() {
			reorderBytes(data, modbusspec.EncodingCDAB)
			done <- true
		}()
	}
	for i := 0; i < 10; i++ {
		<-done
	}
}
