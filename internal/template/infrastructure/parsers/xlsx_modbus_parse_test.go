package parsers

import (
	"strings"
	"testing"
)

// modbusHeaders 与官方模板（tp_modbus_温湿度.xlsx）测点 sheet 表头保持一致。
var modbusHeaders = map[string]int{
	"ID":     0,
	"类型":     1,
	"读写属性":   2,
	"原始数据类型": 3,
	"最终数据类型": 4,
	"编码顺序":   5,
	"寄存器地址":  6,
	"寄存器数量":  7,
	"读功能码":   8,
	"写功能码":   9,
	"中文名称":   10,
	"英文名称":   11,
	"中文单位":   12,
	"英文单位":   13,
	"系数":     14,
	"偏移量":    15,
	"浮点精度":   16,
	"自定义表达式": 17,
	"数值中文含义": 18,
	"数值英文含义": 19,
	"备注":     20,
	"是否启用":   21,
}

func row(values ...string) []string {
	row := make([]string, 22)
	for i, v := range values {
		if i < len(row) {
			row[i] = v
		}
	}
	return row
}

// headerRow 根据 modbusHeaders 生成表头行，供 parseModbusPoints 在 rows 内识别。
func headerRow(modbusHeaders map[string]int) []string {
	rows := make([]string, len(modbusHeaders))
	for name, idx := range modbusHeaders {
		rows[idx] = name
	}
	return rows
}

// TestParseModbusPointsOfficialStyle 验证官方模板典型行（原始 INT16 + 最终 FLOAT64）
// 与 0x 十六进制读功能码可正常解析。
func TestParseModbusPointsOfficialStyle(t *testing.T) {
	rows := [][]string{
		headerRow(modbusHeaders),
		row("1", "POINT", "r", "INT16", "FLOAT64", "ABCD", "0x00", "1", "0x03", "", "温度", "Temperature", "℃", "℃", "1", "0", "2", "", "", "", "", "Y"),
	}
	points, err := parseModbusPoints(rows, modbusHeaders)
	if err != nil {
		t.Fatalf("parseModbusPoints() error = %v", err)
	}
	if len(points) != 1 {
		t.Fatalf("points = %d, want 1", len(points))
	}
	p := points[0]
	if p.DataType != "FLOAT64" {
		t.Errorf("DataType = %q, want FLOAT64", p.DataType)
	}
	if p.Modbus.RegisterAddress != 0 {
		t.Errorf("RegisterAddress = %d, want 0", p.Modbus.RegisterAddress)
	}
}

// TestParseModbusPointsHexAddress 验证 0x 十六进制寄存器地址解析。
func TestParseModbusPointsHexAddress(t *testing.T) {
	rows := [][]string{
		headerRow(modbusHeaders),
		row("1", "POINT", "r", "INT16", "", "", "0x100", "1", "", "", "测试", "Test", "", "", "", "", "", "", "", "", "", "Y"),
	}
	points, err := parseModbusPoints(rows, modbusHeaders)
	if err != nil {
		t.Fatalf("parseModbusPoints() error = %v", err)
	}
	p := points[0]
	if p.Modbus.RegisterAddress != 0x100 {
		t.Errorf("RegisterAddress = %d, want 256", p.Modbus.RegisterAddress)
	}
}

// TestParseModbusPointsFinalTypeNormalized 验证最终数据类型 INT32 归一到 INT64。
func TestParseModbusPointsFinalTypeNormalized(t *testing.T) {
	rows := [][]string{
		headerRow(modbusHeaders),
		row("1", "POINT", "r", "INT32", "INT32", "ABCD", "0x10", "2", "0x03", "", "测试", "Test", "", "", "", "", "", "", "", "", "", "Y"),
	}
	points, err := parseModbusPoints(rows, modbusHeaders)
	if err != nil {
		t.Fatalf("parseModbusPoints() error = %v", err)
	}
	if points[0].DataType != "INT64" {
		t.Errorf("DataType = %q, want INT64", points[0].DataType)
	}
}

// TestDetectHeaderRowSkipsExplanationRowAboveHeader 锁住一条真实踩过的坑：
// 老模板会在表头上方放一行“说明行”，而说明文字里常出现“测点/名称”等关键字。
// 旧实现按关键字 Contains 取第一个匹配行，于是把说明行当成表头、把真表头当成数据行，
// 报出“Modbus测点第2行解析失败: ID 必须大于 0”。现在按“精确列名打分”选表头，必须选中真表头。
func TestDetectHeaderRowSkipsExplanationRowAboveHeader(t *testing.T) {
	explanation := make([]string, len(modbusHeaders))
	explanation[0] = "必须从1开始，连续递增，用于自定义表达式中的数据编号。"
	explanation[1] = "内置“测点”不用设置“自定义表达式”，自定义“指标”不用设置“功能码、地址、系数”。"

	rows := [][]string{
		explanation,
		headerRow(modbusHeaders),
		row("1", "POINT", "r", "INT16", "FLOAT64", "ABCD", "0x00", "1", "0x03", "", "温度", "Temperature", "℃", "℃", "1", "0", "2", "", "", "", "", "Y"),
	}

	idx, ok := detectHeaderRow(rows)
	if !ok || idx != 1 {
		t.Fatalf("detectHeaderRow() = (%d, %v), want (1, true)", idx, ok)
	}

	headers := make(map[string]int, len(modbusHeaders))
	for j, cell := range rows[idx] {
		headers[strings.TrimSpace(cell)] = j
	}
	points, err := parseModbusPoints(rows, headers)
	if err != nil {
		t.Fatalf("parseModbusPoints() 不应被说明行影响, error = %v", err)
	}
	if len(points) != 1 || points[0].PointID != 1 || points[0].Name != "温度" {
		t.Fatalf("parseModbusPoints() points = %+v", points)
	}
}

// TestParseModbusPointsLegacyEnums 锁住老动环模板的枚举兼容：
// 老模板用“编号-中文”写法（类型 0-测点/1-指标、原始类型 0-整数/1-浮点数），
// 且指标行不填“最终数据类型”。兼容规则：
//   - 0-测点 → POINT、1-指标 → METRIC（无歧义）
//   - 0-整数 按“寄存器数量”推断位宽（1 个寄存器 = 16 位 → INT16，有符号）
//   - 最终数据类型为空时由原始类型推导（整数 → INT64、浮点 → FLOAT64）
func TestParseModbusPointsLegacyEnums(t *testing.T) {
	rows := [][]string{
		headerRow(modbusHeaders),
		row("1", "0-测点", "r", "0-整数", "", "", "0x00", "1", "0x03", "", "公共报警", "", "", "", "1", "0", "2", "", "", "", "", "Y"),
		row("2", "1-指标", "", "0-整数", "", "", "", "", "", "", "累计运行小时", "", "h", "", "1", "0", "2", "a100*10000+a101", "", "", "", "Y"),
	}

	points, err := parseModbusPoints(rows, modbusHeaders)
	if err != nil {
		t.Fatalf("parseModbusPoints(legacy) error = %v", err)
	}
	if len(points) != 2 {
		t.Fatalf("parseModbusPoints(legacy) len = %d, want 2", len(points))
	}

	legacyPoint := points[0]
	if legacyPoint.Type != "POINT" {
		t.Fatalf("老格式 0-测点 应归一为 POINT, got %q", legacyPoint.Type)
	}
	if legacyPoint.OriginalType != "INT16" {
		t.Fatalf("老格式 0-整数 + 1 个寄存器 应归一为 INT16, got %q", legacyPoint.OriginalType)
	}
	if legacyPoint.DataType != "INT64" {
		t.Fatalf("老格式测点最终类型应为 INT64, got %q", legacyPoint.DataType)
	}

	legacyMetric := points[1]
	if legacyMetric.Type != "METRIC" {
		t.Fatalf("老格式 1-指标 应归一为 METRIC, got %q", legacyMetric.Type)
	}
	if legacyMetric.DataType != "INT64" {
		t.Fatalf("老格式指标未填最终类型时应按原始类型推导为 INT64, got %q", legacyMetric.DataType)
	}
}
