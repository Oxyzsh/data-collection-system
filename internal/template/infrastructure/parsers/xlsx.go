package parsers

import (
	"fmt"
	"strconv"
	"strings"

	alarmdomain "data-collection-system/internal/templatealarm"
	modbusspec "data-collection-system/internal/protocol/modbus"
	snmpproto "data-collection-system/internal/protocol/snmp"
	templatebiz "data-collection-system/internal/template/domain"
	"data-collection-system/internal/valueobj"

	"github.com/xuri/excelize/v2"
)

// TemplateImport 模板导入结果
// 注意：全局参数已从模板中移除，改为根据协议类型动态生成
type TemplateImport struct {
	TemplateName string                    `json:"template_name"`
	Description  string                    `json:"description"`
	TemplateType string                    `json:"template_type"` // "modbus" 或 "snmp"
	Points       []templatebiz.DevicePoint `json:"points"`
	Alarms       []alarmdomain.DeviceAlarm `json:"alarms"`
}

func ParseTemplateFile(filePath string) (*TemplateImport, error) {
	return ParseTemplateFileWithType(filePath, "")
}

// ParseTemplateFileWithType 解析模板文件，支持用户指定类型（验证模式）
// userType: 用户选择的模板类型，空字符串表示完全自动检测
// 返回：解析结果 + 错误（类型冲突时返回错误）
// 注意：全局参数已从模板中移除，改为根据协议类型动态生成
func ParseTemplateFileWithType(filePath, userType string) (*TemplateImport, error) {
	f, err := excelize.OpenFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("打开文件失败：%v", err)
	}
	defer f.Close()

	template := &TemplateImport{
		Points: make([]templatebiz.DevicePoint, 0),
		Alarms: make([]alarmdomain.DeviceAlarm, 0),
	}

	sheets := f.GetSheetList()
	for _, sheet := range sheets {
		switch strings.ToLower(sheet) {
		case "globalparams", "全局参数":
			// 全局参数已从模板中移除，忽略此 sheet
			// 参数改为根据协议类型动态生成
			continue
		case "points", "测点":
			rows, headers, err := parseSheetHeaders(f, sheet)
			if err != nil {
				return nil, fmt.Errorf("解析测点表头失败：%v", err)
			}
			if len(headers) == 0 {
				continue
			}

			// 自动检测模板类型
			detectedType := DetectType(headers)

			// 验证模式：用户指定类型与自动检测结果不一致时，返回错误
			if userType != "" && userType != detectedType {
				return nil, fmt.Errorf(
					"模板类型不匹配：您选择的是 %q，但文件内容检测为 %q，请检查文件是否正确",
					userType, detectedType,
				)
			}

			template.TemplateType = detectedType

			// 通过注册表获取对应的解析器
			if parser, ok := GetParser(detectedType); ok {
				points, err := parser.ParsePoints(rows, headers)
				if err != nil {
					return nil, err
				}
				template.Points = points
			}
		case "alarms", "告警":
			template.Alarms = parseAlarms(f, sheet)
		case "template", "模板信息":
			// 从模板信息 sheet 中提取模板名称和描述
			template.TemplateName, template.Description = parseTemplateInfo(f, sheet)
		}
	}

	// 默认模板类型为 modbus
	if template.TemplateType == "" {
		template.TemplateType = "modbus"
	}

	if template.TemplateName == "" {
		template.TemplateName = "导入的模板"
	}

	return template, nil
}

func parseTemplateInfo(f *excelize.File, sheet string) (string, string) {
	rows, err := f.GetRows(sheet)
	if err != nil {
		return "", ""
	}

	var templateName, description string
	for _, row := range rows {
		if len(row) < 2 {
			continue
		}
		key := strings.TrimSpace(row[0])
		value := strings.TrimSpace(row[1])

		switch key {
		case "模板名称", "Template Name", "名称":
			templateName = value
		case "描述", "Description", "模板描述":
			description = value
		}
	}

	return templateName, description
}

// parseSheetHeaders 从 sheet 中提取表头映射（自动检测表头行）
// 优先检测第一行是否为表头（包含ID、名称等关键字），否则使用第二行
func parseSheetHeaders(f *excelize.File, sheet string) ([][]string, map[string]int, error) {
	rows, err := f.GetRows(sheet)
	if err != nil {
		return nil, nil, err
	}
	if len(rows) < 1 {
		return rows, nil, nil
	}

	// 先用“精确列名打分”定位表头；打分不够可信时退回旧的关键字判定。
	// 这样表头之前的说明行（只会“包含”关键字）不会被误判成表头。
	headerRow, confident := detectHeaderRow(rows)
	if !confident && len(rows) >= 2 && !isHeaderRow(rows[0]) && isHeaderRow(rows[1]) {
		headerRow = 1
	}

	headers := make(map[string]int)
	for j, cell := range rows[headerRow] {
		headers[strings.TrimSpace(cell)] = j
	}
	return rows, headers, nil
}

// knownColumnNames 收录解析器真正读取的全部列名（测点表 + 告警表）。
// 表头识别用它做“精确匹配打分”，比按关键字 Contains 更不容易被说明行、数据行骗到。
var knownColumnNames = map[string]struct{}{
	// 测点表
	"ID": {}, "OID": {}, "类型": {}, "读写属性": {}, "读功能码": {}, "写功能码": {},
	"寄存器地址": {}, "寄存器数量": {}, "中文名称": {}, "英文名称": {}, "是否启用": {},
	"原始数据类型": {}, "最终数据类型": {}, "浮点精度": {}, "编码顺序": {}, "系数": {},
	"偏移量": {}, "修正值": {}, "中文单位": {}, "英文单位": {}, "自定义表达式": {},
	"数值中文含义": {}, "数值英文含义": {}, "备注": {},
	"通道名称": {}, "DI通道名称": {}, "DO通道名称": {}, "通道": {},
	// 告警表
	"级别": {}, "关联测点ID": {}, "关联测点 ID": {}, "测点值掩码": {}, "底限": {}, "高限": {},
	"延迟秒数": {}, "开始延迟秒数": {}, "结束延迟秒数": {}, "告警开始延时（秒）": {},
	"高于此值开始告警": {}, "低于此值开始告警": {}, "高于此值结束告警": {}, "低于此值结束告警": {},
}

const (
	// maxHeaderScanRows 只在前若干行里找表头（表头总在文件开头）。
	maxHeaderScanRows = 5
	// minHeaderColumnMatches 至少精确命中这么多个列名，才认为这一行是表头。
	// 正常模板的列名都是精确的（测点表 20+ 列、告警表 10+ 列），阈值取 3 足够稳。
	minHeaderColumnMatches = 3
)

// headerMatchScore 统计一行里有多少个单元格与已知列名“完全一致”。
func headerMatchScore(row []string) int {
	score := 0
	for _, cell := range row {
		if _, ok := knownColumnNames[strings.TrimSpace(cell)]; ok {
			score++
		}
	}
	return score
}

// detectHeaderRow 选出“最像表头”的一行：精确命中列名最多者，分数相同取更靠前的行。
// 返回 (行下标, 是否足够可信)；不可信时调用方可以退回旧的关键字判定。
func detectHeaderRow(rows [][]string) (int, bool) {
	bestIndex, bestScore := -1, 0
	limit := len(rows)
	if limit > maxHeaderScanRows {
		limit = maxHeaderScanRows
	}
	for i := 0; i < limit; i++ {
		if score := headerMatchScore(rows[i]); score > bestScore {
			bestIndex, bestScore = i, score
		}
	}
	if bestScore >= minHeaderColumnMatches {
		return bestIndex, true
	}
	return 0, false
}

// isHeaderRow 判断一行是否为表头行（包含ID、名称等关键字）
func isHeaderRow(row []string) bool {
	keywords := []string{"ID", "名称", "中文名称", "英文名称", "Name", "点号", "测点"}
	for _, cell := range row {
		cell = strings.TrimSpace(cell)
		for _, keyword := range keywords {
			if strings.Contains(cell, keyword) {
				return true
			}
		}
	}
	return false
}

// parseModbusPoints 解析 Modbus 协议的测点数据
func parseModbusPoints(rows [][]string, headers map[string]int) ([]templatebiz.DevicePoint, error) {
	points := make([]templatebiz.DevicePoint, 0)

	// 表头行：优先精确列名打分；打分不可信时退回关键字扫描。
	headerRowIndex, confident := detectHeaderRow(rows)
	if !confident {
		headerRowIndex = -1
		for i, row := range rows {
			if isHeaderRow(row) {
				headerRowIndex = i
				break
			}
		}
	}

	// 如果没有找到表头行，默认从第二行开始解析
	startRow := 1
	if headerRowIndex >= 0 {
		startRow = headerRowIndex + 1
	}

	for i := startRow; i < len(rows); i++ {
		row := rows[i]

		// 跳过注释行（包含"字段解释"关键字的行）
		if isCommentRow(row) {
			continue
		}

		reader := newPointRowReader(headers, row)
		if reader.isEmptyRow() {
			continue
		}

		point, err := parseCommonPointFields(reader)
		if err != nil {
			return nil, fmt.Errorf("Modbus测点第%d行解析失败: %w", i+1, err)
		}
		if err := parseModbusPointSpec(&point, reader); err != nil {
			return nil, fmt.Errorf("Modbus测点[%d:%s] 解析失败: %w", point.PointID, point.Name, err)
		}
		if err := validateModbusPoint(point); err != nil {
			return nil, fmt.Errorf("Modbus测点[%d:%s] 校验失败: %w", point.PointID, point.Name, err)
		}
		points = append(points, point)
	}

	return points, nil
}

// parseSNMPPoints 解析 SNMP 协议的测点数据
func parseSNMPPoints(rows [][]string, headers map[string]int) ([]templatebiz.DevicePoint, error) {
	points := make([]templatebiz.DevicePoint, 0)

	// 表头行：优先精确列名打分；打分不可信时退回关键字扫描。
	headerRowIndex, confident := detectHeaderRow(rows)
	if !confident {
		headerRowIndex = -1
		for i, row := range rows {
			if isHeaderRow(row) {
				headerRowIndex = i
				break
			}
		}
	}

	// 如果没有找到表头行，默认从第二行开始解析
	startRow := 1
	if headerRowIndex >= 0 {
		startRow = headerRowIndex + 1
	}

	for i := startRow; i < len(rows); i++ {
		row := rows[i]

		// 跳过注释行（包含"字段解释"关键字的行）
		if isCommentRow(row) {
			continue
		}

		reader := newPointRowReader(headers, row)
		if reader.isEmptyRow() {
			continue
		}

		point, err := parseCommonPointFields(reader)
		if err != nil {
			return nil, fmt.Errorf("SNMP测点第%d行解析失败: %w", i+1, err)
		}
		if err := parseSNMPPointSpec(&point, reader); err != nil {
			return nil, fmt.Errorf("SNMP测点[%d:%s] 解析失败: %w", point.PointID, point.Name, err)
		}
		if err := validateSNMPPoint(point); err != nil {
			return nil, fmt.Errorf("SNMP测点[%d:%s] 校验失败: %w", point.PointID, point.Name, err)
		}
		points = append(points, point)
	}

	return points, nil
}

// parseLocalIOPoints 解析本地 GPIO 模板的点位。
func parseLocalIOPoints(rows [][]string, headers map[string]int, channelHeader, direction, templateType string) ([]templatebiz.DevicePoint, error) {
	points := make([]templatebiz.DevicePoint, 0)
	headerRowIndex, confident := detectHeaderRow(rows)
	if !confident {
		headerRowIndex = -1
		for i, row := range rows {
			if isHeaderRow(row) {
				headerRowIndex = i
				break
			}
		}
	}
	startRow := 1
	if headerRowIndex >= 0 {
		startRow = headerRowIndex + 1
	}

	for i := startRow; i < len(rows); i++ {
		row := rows[i]
		if isCommentRow(row) {
			continue
		}
		reader := newPointRowReader(headers, row)
		if reader.isEmptyRow() {
			continue
		}
		point, err := parseCommonPointFields(reader)
		if err != nil {
			return nil, fmt.Errorf("local_io测点第%d行解析失败: %w", i+1, err)
		}
		if err := parseLocalIOPointSpec(&point, reader, channelHeader, direction); err != nil {
			return nil, fmt.Errorf("local_io测点[%d:%s] 解析失败: %w", point.PointID, point.Name, err)
		}
		if err := templatebiz.ValidatePointForTemplateType(templateType, point); err != nil {
			return nil, fmt.Errorf("local_io测点[%d:%s] 校验失败: %w", point.PointID, point.Name, err)
		}
		points = append(points, point)
	}
	return points, nil
}

func parseLocalIOPointSpec(point *templatebiz.DevicePoint, reader pointRowReader, channelHeader, direction string) error {
	channel := strings.TrimSpace(reader.getString(channelHeader))
	if channel == "" {
		channel = strings.TrimSpace(reader.getString("通道"))
	}
	if channel == "" {
		return fmt.Errorf("通道名称必填")
	}
	point.LocalIO = &templatebiz.LocalIOPointSpec{Channel: channel, Direction: direction}
	point.OriginalType = valueobj.RawTypeBOOL
	point.DataType = valueobj.DataTypeBOOL
	if strings.TrimSpace(point.Access) == "" {
		if direction == "input" {
			point.Access = "r"
		} else if direction == "output" {
			point.Access = "rw"
		} else if strings.HasPrefix(strings.ToUpper(channel), "DI") {
			point.Access = "r"
		} else {
			point.Access = "rw"
		}
	}
	return nil
}

// parseAlarms 从 excelize.File 解析告警 sheet
func parseAlarms(f *excelize.File, sheet string) []alarmdomain.DeviceAlarm {
	rows, err := f.GetRows(sheet)
	if err != nil {
		return make([]alarmdomain.DeviceAlarm, 0)
	}
	if len(rows) < 1 {
		return make([]alarmdomain.DeviceAlarm, 0)
	}

	// 表头行：优先精确列名打分；打分不可信时退回告警关键字扫描。
	headerRowIndex, confident := detectHeaderRow(rows)
	if !confident {
		headerRowIndex = -1
		for i, row := range rows {
			if isAlarmHeaderRow(row) {
				headerRowIndex = i
				break
			}
		}
	}

	// 如果没有找到表头行，默认使用第一行
	if headerRowIndex < 0 {
		headerRowIndex = 0
	}

	// 收集所有列位置（支持同名列，如"底限"/"高限"出现两次：触发+恢复）
	headerPositions := make(map[string][]int)
	for j, cell := range rows[headerRowIndex] {
		name := strings.TrimSpace(cell)
		headerPositions[name] = append(headerPositions[name], j)
	}

	return parseAlarmRows(rows, headerPositions, headerRowIndex)
}

// isAlarmHeaderRow 判断一行是否为告警表头行（包含告警相关关键字）
func isAlarmHeaderRow(row []string) bool {
	keywords := []string{"告警", "级别", "测点", "ID", "启用", "Alarm", "Level", "Point"}
	for _, cell := range row {
		cell = strings.TrimSpace(cell)
		for _, keyword := range keywords {
			if strings.Contains(cell, keyword) {
				return true
			}
		}
	}
	return false
}

// isCommentRow 判断一行是否为注释行（包含"字段解释"关键字）
func isCommentRow(row []string) bool {
	for _, cell := range row {
		cell = strings.TrimSpace(cell)
		if strings.Contains(cell, "字段解释") {
			return true
		}
	}
	return false
}

// parseAlarmRows 根据已解析的行数据和表头位置解析告警
// headerPositions 记录每个列名出现的所有位置索引，用于区分重复列（触发条件 vs 恢复条件）
// headerRowIndex 表头行的位置，数据行从表头行之后开始
func parseAlarmRows(rows [][]string, headerPositions map[string][]int, headerRowIndex int) []alarmdomain.DeviceAlarm {
	alarms := make([]alarmdomain.DeviceAlarm, 0)

	// getCol 返回 name 列的第一个位置，未找到返回 -1
	getCol := func(name string) int {
		if positions := headerPositions[name]; len(positions) > 0 {
			return positions[0]
		}
		return -1
	}

	// getColN 返回 name 列的第 n 个位置（0-based），未找到返回 -1
	getColN := func(name string, n int) int {
		if positions := headerPositions[name]; len(positions) > n {
			return positions[n]
		}
		return -1
	}

	// getCell 读取指定列的单元格字符串，越界返回空字符串
	getCell := func(row []string, col int) string {
		if col >= 0 && col < len(row) {
			return strings.TrimSpace(fmt.Sprintf("%v", row[col]))
		}
		return ""
	}

	// 从表头行之后开始解析数据
	startRow := headerRowIndex + 1
	alarmID := 1 // 告警规则 ID 从 1 开始递增
	for i := startRow; i < len(rows); i++ {
		row := rows[i]

		// 跳过注释行（包含"字段解释"关键字的行）
		if isCommentRow(row) {
			continue
		}

		alarm := alarmdomain.DeviceAlarm{
			ID:      alarmID, // 分配唯一 ID
			Enabled: true,    // 默认启用告警
		}

		// 启用状态（兼容 "Y"、"1"、"1-是"、"N"、"0"、"0-否" 等格式）
		if idx := getCol("是否启用"); idx >= 0 {
			val := getCell(row, idx)
			// 跳过未启用的告警（N或0开头）
			if val != "" && (strings.HasPrefix(strings.ToUpper(val), "N") || strings.HasPrefix(val, "0")) {
				continue
			}
		}

		// 关联测点 ID（兼容带空格和不带空格两种写法）
		pointIDCol := getCol("关联测点 ID")
		if pointIDCol < 0 {
			pointIDCol = getCol("关联测点ID")
		}
		if pointIDCol >= 0 {
			alarm.PointID, _ = strconv.Atoi(getCell(row, pointIDCol))
		}

		// 告警名称（优先中文名称，其次英文名称）
		if idx := getCol("中文名称"); idx >= 0 {
			alarm.Name = getCell(row, idx)
		}
		if alarm.Name == "" {
			if idx := getCol("英文名称"); idx >= 0 {
				alarm.Name = getCell(row, idx)
			}
		}

		// 级别
		if idx := getCol("级别"); idx >= 0 {
			val := getCell(row, idx)
			if parts := strings.SplitN(val, "-", 2); len(parts) == 2 {
				alarm.Level = strings.TrimSpace(parts[1])
			} else {
				alarm.Level = val
			}
		}

		// --- 触发条件（第一组：底限/高限/延迟） ---

		// 触发低限（支持多种列名）
		startLowCol := getCol("低于此值开始告警")
		if startLowCol < 0 {
			startLowCol = getColN("底限", 0)
		}
		if startLowCol >= 0 {
			if v, err := strconv.ParseFloat(getCell(row, startLowCol), 64); err == nil {
				alarm.StartLowLimit = &v
			}
		}

		// 触发高限（支持多种列名）
		startHighCol := getCol("高于此值开始告警")
		if startHighCol < 0 {
			startHighCol = getColN("高限", 0)
		}
		if startHighCol >= 0 {
			if v, err := strconv.ParseFloat(getCell(row, startHighCol), 64); err == nil {
				alarm.StartHighLimit = &v
			}
		}

		// 触发延迟秒数（支持多种列名）
		startDelayCol := getCol("告警开始延时（秒）")
		if startDelayCol < 0 {
			startDelayCol = getCol("开始延迟秒数")
		}
		if startDelayCol < 0 {
			startDelayCol = getColN("延迟秒数", 0)
		}
		if startDelayCol >= 0 {
			alarm.StartDelay, _ = strconv.Atoi(getCell(row, startDelayCol))
		}

		// --- 恢复条件（第二组：底限/高限/延迟） ---

		// 恢复低限（支持多种列名）
		endLowCol := getCol("低于此值结束告警")
		if endLowCol < 0 {
			endLowCol = getColN("底限", 1)
		}
		if endLowCol >= 0 {
			if v, err := strconv.ParseFloat(getCell(row, endLowCol), 64); err == nil {
				alarm.EndLowLimit = &v
			}
		}

		// 恢复高限（支持多种列名）
		endHighCol := getCol("高于此值结束告警")
		if endHighCol < 0 {
			endHighCol = getColN("高限", 1)
		}
		if endHighCol >= 0 {
			if v, err := strconv.ParseFloat(getCell(row, endHighCol), 64); err == nil {
				alarm.EndHighLimit = &v
			}
		}

		// 恢复延迟秒数（支持多种列名）
		endDelayCol := getCol("结束延迟秒数")
		if endDelayCol < 0 {
			endDelayCol = getColN("延迟秒数", 1)
		}
		if endDelayCol >= 0 {
			alarm.EndDelay, _ = strconv.Atoi(getCell(row, endDelayCol))
		}

		// 描述
		if idx := getCol("备注"); idx >= 0 {
			alarm.Description = getCell(row, idx)
		}

		// 测点值掩码
		if idx := getCol("测点值掩码"); idx >= 0 {
			alarm.PointMask = getCell(row, idx)
		}

		if alarm.PointID > 0 && alarm.Level != "" {
			alarms = append(alarms, alarm)
			alarmID++ // 递增 ID
		}
	}

	return alarms
}

type pointRowReader struct {
	headers map[string]int
	row     []string
}

func newPointRowReader(headers map[string]int, row []string) pointRowReader {
	return pointRowReader{headers: headers, row: row}
}

func (r pointRowReader) getString(key string) string {
	if idx, ok := r.headers[key]; ok && idx < len(r.row) {
		return strings.TrimSpace(fmt.Sprintf("%v", r.row[idx]))
	}
	return ""
}

func (r pointRowReader) getInt(key string) int {
	if v := r.getString(key); v != "" {
		// 模板字段说明允许寄存器地址/功能码使用十进制或 0x 开头的十六进制
		if val, err := strconv.ParseInt(v, 0, 64); err == nil {
			return int(val)
		}
	}
	return 0
}

func (r pointRowReader) getFloat(key string) float64 {
	if v := r.getString(key); v != "" {
		val, _ := strconv.ParseFloat(v, 64)
		return val
	}
	return 0
}

func (r pointRowReader) isEmptyRow() bool {
	for _, cell := range r.row {
		if strings.TrimSpace(cell) != "" {
			return false
		}
	}
	return true
}

// ---------- 老动环模板兼容 ----------
//
// 历史动环（C 版）的模板 Excel 用的是“编号-中文”枚举，粒度也比现在粗：
//
//	类型         ：0-测点 / 1-指标
//	原始数据类型  ：0-整数 / 1-浮点数   （不写位宽，靠“寄存器数量”列推断）
//
// 这里做一次映射，避免整份老模板直接导入失败；映射是显式的，
// 位宽按占用寄存器个数推断（1 个=16 位、2 个=32 位、4 个=64 位），符号按“有符号”处理。

var legacyPointTypeAliases = map[string]string{
	"0-测点": templatebiz.PointTypePOINT,
	"1-指标": templatebiz.PointTypeMETRIC,
}

var legacyRawTypeKinds = map[string]string{
	"0-整数":  "int",
	"1-浮点数": "float",
	"2-布尔":  "bool",
	"3-字符串": "string",
}

// legacyPointType 把老格式的“类型”归一成标准值；不是老写法时原样返回。
func legacyPointType(value string) string {
	if mapped, ok := legacyPointTypeAliases[strings.TrimSpace(value)]; ok {
		return mapped
	}
	return value
}

// legacyRawType 把老格式的“原始数据类型”映射成标准原始类型。
// 返回 (归一后的值, 是否命中老写法)；未命中时原样返回，交给后续白名单校验。
func legacyRawType(value string, registerCount int) (string, bool) {
	kind, ok := legacyRawTypeKinds[strings.TrimSpace(value)]
	if !ok {
		return value, false
	}
	if registerCount <= 0 {
		registerCount = 1
	}
	switch kind {
	case "int":
		switch {
		case registerCount >= 4:
			return valueobj.RawTypeINT64, true
		case registerCount >= 2:
			return valueobj.RawTypeINT32, true
		default:
			return valueobj.RawTypeINT16, true
		}
	case "float":
		if registerCount >= 2 {
			return valueobj.RawTypeFLOAT64, true
		}
		return valueobj.RawTypeFLOAT32, true
	case "bool":
		return valueobj.RawTypeBOOL, true
	case "string":
		return valueobj.RawTypeSTRING, true
	}
	return value, false
}

func parseCommonPointFields(reader pointRowReader) (templatebiz.DevicePoint, error) {
	// legacyRawMatched 记录“原始数据类型”是否命中老格式写法；
	// 老模板的指标行不填最终数据类型，需要用原始类型推导一个（整数→INT64 / 浮点→FLOAT64）。
	var legacyRawMatched bool
	point := templatebiz.DevicePoint{
		Type:        templatebiz.PointTypePOINT,
		Coefficient: 1.0,
		Precision:   2,
		Enabled:     1,
	}

	point.PointID = reader.getInt("ID")
	point.Type = templatebiz.NormalizePointType(legacyPointType(reader.getString("类型")))
	point.Name = reader.getString("中文名称")
	point.NameEN = reader.getString("英文名称")
	if point.Name == "" {
		point.Name = point.NameEN
	}
	point.Access = reader.getString("读写属性")
	point.OriginalType, legacyRawMatched = legacyRawType(reader.getString("原始数据类型"), reader.getInt("寄存器数量"))
	point.DataType = reader.getString("最终数据类型")
	point.Unit = reader.getString("中文单位")
	point.UnitEN = reader.getString("英文单位")
	if point.Unit == "" {
		point.Unit = point.UnitEN
	}
	if coefficient := reader.getFloat("系数"); coefficient != 0 {
		point.Coefficient = coefficient
	}
	point.Offset = reader.getFloat("偏移量")
	point.CorrectionValue = reader.getFloat("修正值")
	if precision := reader.getInt("浮点精度"); precision != 0 {
		point.Precision = precision
	}
	if enabled := reader.getInt("是否启用"); enabled != 0 {
		point.Enabled = enabled
	}
	point.Expression = reader.getString("自定义表达式")
	point.ValueMappingCN = reader.getString("数值中文含义")
	point.ValueMappingEN = reader.getString("数值英文含义")
	point.ValueMeaning = point.ValueMappingCN
	if point.ValueMeaning == "" {
		point.ValueMeaning = point.ValueMappingEN
	}
	point.Description = reader.getString("备注")

	if point.PointID <= 0 {
		return templatebiz.DevicePoint{}, fmt.Errorf("ID 必须大于 0")
	}
	if strings.TrimSpace(point.Name) == "" && strings.TrimSpace(point.NameEN) == "" {
		return templatebiz.DevicePoint{}, fmt.Errorf("中文名称和英文名称至少填写一个")
	}
	if point.DataType == "" && legacyRawMatched {
		// 老模板不带“最终数据类型”，用它的原始类型推导（整数→INT64，浮点→FLOAT64）。
		point.DataType = normalizeFinalDataType(point.OriginalType)
	}
	if point.DataType != "" {
		point.DataType = normalizeFinalDataType(point.DataType)
		if point.DataType == "" {
			return templatebiz.DevicePoint{}, fmt.Errorf("最终数据类型只允许 INT16/UINT16/INT32/UINT32/INT64/UINT64/FLOAT32/FLOAT64/BOOL/STRING")
		}
	}

	return point, nil
}

// normalizeFinalDataType 将模板里的最终数据类型归一到系统支持的四种基础类型。
// 官方 Excel 模板字段说明允许填写全部原始类型（INT16 等），
// 系统内部统一按 64 位存储：整数类归 INT64、浮点类归 FLOAT64、BOOL/STRING 保持。
func normalizeFinalDataType(s string) string {
	switch valueobj.NormalizeRawType(s) {
	case valueobj.RawTypeINT16, valueobj.RawTypeUINT16, valueobj.RawTypeINT32, valueobj.RawTypeUINT32,
		valueobj.RawTypeINT64, valueobj.RawTypeUINT64:
		return valueobj.DataTypeINT64
	case valueobj.RawTypeFLOAT32, valueobj.RawTypeFLOAT64:
		return valueobj.DataTypeFLOAT64
	case valueobj.RawTypeBOOL:
		return valueobj.DataTypeBOOL
	case valueobj.RawTypeSTRING:
		return valueobj.DataTypeSTRING
	}
	return ""
}

func parseModbusPointSpec(point *templatebiz.DevicePoint, reader pointRowReader) error {
	if point.Type == templatebiz.PointTypeMETRIC {
		if point.DataType == "" {
			return fmt.Errorf("METRIC 测点必须填写最终数据类型")
		}
		return nil
	}

	rawType := valueobj.NormalizeRawType(point.OriginalType)
	if rawType == "" {
		return fmt.Errorf("原始数据类型不受支持：%s", point.OriginalType)
	}
	point.OriginalType = rawType

	spec := &templatebiz.ModbusPointSpec{
		RegisterAddress: reader.getInt("寄存器地址"),
		RegisterCount:   reader.getInt("寄存器数量"),
		ReadFuncCode:    reader.getInt("读功能码"),
		WriteFuncCode:   reader.getInt("写功能码"),
		EncodingOrder:   strings.ToUpper(strings.TrimSpace(reader.getString("编码顺序"))),
	}
	if spec.EncodingOrder == "" {
		spec.EncodingOrder = modbusspec.TypeDefaultEncodingOrder(rawType)
	}
	if spec.RegisterCount == 0 {
		spec.RegisterCount = modbusspec.TypeRegisterCount(rawType)
	}
	if spec.ReadFuncCode == 0 {
		spec.ReadFuncCode = modbusspec.TypeDefaultFuncCode(rawType)
	}
	point.Modbus = spec

	if point.DataType == "" {
		point.DataType = deriveDataTypeFromRawType(rawType)
	}
	return nil
}

func parseSNMPPointSpec(point *templatebiz.DevicePoint, reader pointRowReader) error {
	if point.Type == templatebiz.PointTypeMETRIC {
		if point.DataType == "" {
			return fmt.Errorf("METRIC 测点必须填写最终数据类型")
		}
		return nil
	}

	originalType := snmpproto.NormalizeOriginalType(point.OriginalType)
	if originalType == "" {
		return fmt.Errorf("原始数据类型不受支持：%s", point.OriginalType)
	}
	point.OriginalType = originalType

	oid := reader.getString("OID")
	if oid == "" {
		return fmt.Errorf("OID 必填")
	}
	point.SNMP = &templatebiz.SNMPPointSpec{OID: oid}

	if point.DataType == "" {
		point.DataType = snmpproto.ToDataType(originalType)
	}
	return nil
}

func validateModbusPoint(point templatebiz.DevicePoint) error {
	if point.Type == templatebiz.PointTypeMETRIC {
		return nil
	}
	if point.Modbus == nil {
		return fmt.Errorf("缺少 Modbus 专用配置")
	}
	if point.Modbus.RegisterAddress < 0 {
		return fmt.Errorf("寄存器地址不能小于 0")
	}
	if point.DataType == "" {
		return fmt.Errorf("无法从原始数据类型推导最终数据类型")
	}
	return nil
}

func validateSNMPPoint(point templatebiz.DevicePoint) error {
	if point.Type == templatebiz.PointTypeMETRIC {
		return nil
	}
	if point.SNMP == nil {
		return fmt.Errorf("缺少 SNMP 专用配置")
	}
	if point.DataType == "" {
		return fmt.Errorf("无法从原始数据类型推导最终数据类型")
	}
	return nil
}

func deriveDataTypeFromRawType(rawType string) string {
	switch rawType {
	case valueobj.RawTypeBOOL:
		return valueobj.DataTypeBOOL
	case valueobj.RawTypeSTRING:
		return valueobj.DataTypeSTRING
	case valueobj.RawTypeFLOAT32, valueobj.RawTypeFLOAT64:
		return valueobj.DataTypeFLOAT64
	case valueobj.RawTypeINT16, valueobj.RawTypeUINT16, valueobj.RawTypeINT32, valueobj.RawTypeUINT32,
		valueobj.RawTypeINT64, valueobj.RawTypeUINT64:
		return valueobj.DataTypeINT64
	default:
		return ""
	}
}
