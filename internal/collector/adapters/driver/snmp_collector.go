package collector

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	snmpproto "data-collection-system/internal/protocol/snmp"
	"data-collection-system/internal/shared/logger"
	templatebiz "data-collection-system/internal/template/domain"
	"data-collection-system/internal/valueobj"

	"github.com/gosnmp/gosnmp"
)

type SNMPCollector struct {
	client     *gosnmp.GoSNMP
	mu         sync.Mutex
	connected  bool
	version    gosnmp.SnmpVersion
	segmentMax int
	timeout    time.Duration
	retries    int
	params     map[string]string // 设备连接参数，供 Write 方法使用
}

// NewSNMPCollector 创建 SNMP 采集器。
// 当前对象主要承载一些默认参数和后续扩展点；每轮采集时再按设备参数构造具体 client。
func NewSNMPCollector() *SNMPCollector {
	return &SNMPCollector{
		segmentMax: 200,
		timeout:    5 * time.Second,
		retries:    0,
	}
}

func init() {
	RegisterCollector("snmp", func(params map[string]string) Collector {
		c := NewSNMPCollector()
		c.params = params
		return c
	})
}

// Collect 执行一轮 SNMP 采集。
// 流程大致是：
// 1. 从设备参数解析连接参数；
// 2. 按版本建立 client；
// 3. 过滤出启用且有 OID 的点位；
// 4. 分批 Get，再把 PDU 转为统一值对象。
func (c *SNMPCollector) Collect(ctx context.Context, deviceID int, points []templatebiz.DevicePoint, params map[string]string) (map[int]valueobj.CollectedValue, error) {
	result := make(map[int]valueobj.CollectedValue)

	ipAddress := params["ip_address"]
	if ipAddress == "" {
		return nil, fmt.Errorf("SNMP采集器: 缺少ip_address参数")
	}

	port := params["port"]
	if port == "" {
		port = "161"
	}

	community := params["community"]
	if community == "" {
		community = "public"
	}

	snmpVersion := params["snmp_v"]
	if snmpVersion == "" {
		snmpVersion = "2"
	}

	segmentMax := 200
	if sm := params["segment_max"]; sm != "" {
		if v, err := strconv.Atoi(sm); err == nil && v > 0 {
			segmentMax = v
		}
	}

	timeout := 5 * time.Second
	if t := params["timeout"]; t != "" {
		if ms, err := strconv.Atoi(t); err == nil && ms > 0 {
			timeout = time.Duration(ms) * time.Millisecond
		}
	}

	retries := 0
	if r := params["retry_cnt"]; r != "" {
		if v, err := strconv.Atoi(r); err == nil && v >= 0 {
			retries = v
		}
	}

	portInt, _ := strconv.Atoi(port)

	var version gosnmp.SnmpVersion
	switch snmpVersion {
	case "1":
		version = gosnmp.Version1
	case "2", "2c":
		version = gosnmp.Version2c
	case "3":
		return c.collectV3(ctx, deviceID, points, params, ipAddress, portInt, segmentMax, timeout, retries)
	default:
		version = gosnmp.Version2c
	}

	client := &gosnmp.GoSNMP{
		Target:    ipAddress,
		Port:      uint16(portInt),
		Version:   version,
		Community: community,
		Timeout:   timeout,
		Retries:   retries,
		MaxOids:   gosnmp.MaxOids,
	}

	if err := client.Connect(); err != nil {
		return nil, fmt.Errorf("SNMP连接失败: %w", err)
	}
	defer client.Conn.Close()

	logger.Debugf("[SNMPCollector] 设备ID: %d, 已连接到 %s:%s, 版本: %s", deviceID, ipAddress, port, snmpVersion)

	pointOIDs := make([]templatebiz.DevicePoint, 0)
	for _, point := range points {
		if point.Type == templatebiz.PointTypePOINT && point.SNMP != nil && point.SNMP.OID != "" && point.Enabled == 1 {
			pointOIDs = append(pointOIDs, point)
		}
	}

	for i := 0; i < len(pointOIDs); i += segmentMax {
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		default:
		}
		end := i + segmentMax
		if end > len(pointOIDs) {
			end = len(pointOIDs)
		}
		batch := pointOIDs[i:end]

		oids := make([]string, len(batch))
		for j, point := range batch {
			oids[j] = point.SNMP.OID
		}

		response, err := client.Get(oids)
		if err != nil {
			logger.Warnf("[SNMPCollector] 设备ID: %d, 批量获取失败: %v", deviceID, err)
			continue
		}

		for j, pdu := range response.Variables {
			if j >= len(batch) {
				break
			}
			point := batch[j]

			value, err := c.parsePDUValue(pdu, point)
			if err != nil {
				logger.Warnf("[SNMPCollector] 设备ID: %d, 测点ID: %d, OID: %s, 解析失败: %v",
					deviceID, point.PointID, point.SNMP.OID, err)
				continue
			}

			result[point.PointID] = value
		}
	}

	return result, nil
}

// buildV3Client 构造并配置 SNMPv3 的 gosnmp.GoSNMP 实例。
func (c *SNMPCollector) buildV3Client(ipAddress string, port int, timeout time.Duration, retries int, params map[string]string) (*gosnmp.GoSNMP, error) {
	userName := params["user_name"]
	authProtocol := params["auth_protocol"]
	authPassword := params["auth_password"]
	privProtocol := params["priv_protocol"]
	privPassword := params["priv_password"]
	securityLevel := params["security_level"]

	if userName == "" {
		return nil, fmt.Errorf("SNMPv3需要user_name参数")
	}

	var authProto gosnmp.SnmpV3AuthProtocol
	switch strings.ToUpper(authProtocol) {
	case "MD5":
		authProto = gosnmp.MD5
	case "SHA":
		authProto = gosnmp.SHA
	default:
		authProto = gosnmp.NoAuth
	}

	var privProto gosnmp.SnmpV3PrivProtocol
	switch strings.ToUpper(privProtocol) {
	case "DES":
		privProto = gosnmp.DES
	case "AES":
		privProto = gosnmp.AES
	default:
		privProto = gosnmp.NoPriv
	}

	var secLevel gosnmp.SnmpV3MsgFlags
	switch strings.ToLower(securityLevel) {
	case "noauthnopriv":
		secLevel = gosnmp.NoAuthNoPriv
	case "authnopriv":
		secLevel = gosnmp.AuthNoPriv
	case "authpriv":
		secLevel = gosnmp.AuthPriv
	default:
		if authProto != gosnmp.NoAuth && privProto != gosnmp.NoPriv {
			secLevel = gosnmp.AuthPriv
		} else if authProto != gosnmp.NoAuth {
			secLevel = gosnmp.AuthNoPriv
		} else {
			secLevel = gosnmp.NoAuthNoPriv
		}
	}

	usmSecurityParameters := &gosnmp.UsmSecurityParameters{
		UserName:                 userName,
		AuthenticationProtocol:   authProto,
		AuthenticationPassphrase: authPassword,
		PrivacyProtocol:          privProto,
		PrivacyPassphrase:        privPassword,
	}

	client := &gosnmp.GoSNMP{
		Target:             ipAddress,
		Port:               uint16(port),
		Version:            gosnmp.Version3,
		Timeout:            timeout,
		Retries:            retries,
		MaxOids:            gosnmp.MaxOids,
		SecurityModel:      gosnmp.UserSecurityModel,
		MsgFlags:           secLevel,
		SecurityParameters: usmSecurityParameters,
	}
	return client, nil
}

// collectV3 把 SNMPv3 的鉴权/加密参数处理单独拆出来，
// 避免普通 v1/v2c 采集路径被大量安全参数逻辑淹没。
func (c *SNMPCollector) collectV3(ctx context.Context, deviceID int, points []templatebiz.DevicePoint, params map[string]string, ipAddress string, port, segmentMax int, timeout time.Duration, retries int) (map[int]valueobj.CollectedValue, error) {
	result := make(map[int]valueobj.CollectedValue)

	client, err := c.buildV3Client(ipAddress, port, timeout, retries, params)
	if err != nil {
		return nil, err
	}

	if err := client.Connect(); err != nil {
		return nil, fmt.Errorf("SNMPv3连接失败: %w", err)
	}
	defer client.Conn.Close()

	logger.Debugf("[SNMPCollector] 设备ID: %d, 已连接到 %s:%d (SNMPv3)", deviceID, ipAddress, port)

	pointOIDs := make([]templatebiz.DevicePoint, 0)
	for _, point := range points {
		if point.Type == templatebiz.PointTypePOINT && point.SNMP != nil && point.SNMP.OID != "" && point.Enabled == 1 {
			pointOIDs = append(pointOIDs, point)
		}
	}

	for i := 0; i < len(pointOIDs); i += segmentMax {
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		default:
		}
		end := i + segmentMax
		if end > len(pointOIDs) {
			end = len(pointOIDs)
		}
		batch := pointOIDs[i:end]

		oids := make([]string, len(batch))
		for j, point := range batch {
			oids[j] = point.SNMP.OID
		}

		response, err := client.Get(oids)
		if err != nil {
			logger.Warnf("[SNMPCollector] 设备ID: %d, 批量获取失败: %v", deviceID, err)
			continue
		}

		for j, pdu := range response.Variables {
			if j >= len(batch) {
				break
			}
			point := batch[j]

			value, err := c.parsePDUValue(pdu, point)
			if err != nil {
				logger.Warnf("[SNMPCollector] 设备ID: %d, 测点ID: %d, OID: %s, 解析失败: %v",
					deviceID, point.PointID, point.SNMP.OID, err)
				continue
			}

			result[point.PointID] = value
		}
	}

	return result, nil
}

// parsePDUValue 把 gosnmp 返回的 PDU 转成项目统一的 CollectedValue。
// 这一步的关键是区分：
// - SNMP 原始类型；
// - 模板里声明的目标数据类型。
func (c *SNMPCollector) parsePDUValue(pdu gosnmp.SnmpPDU, point templatebiz.DevicePoint) (valueobj.CollectedValue, error) {
	originalType, targetDataType := resolveSNMPPointTypes(point, pdu.Type)

	switch pdu.Type {
	case gosnmp.NoSuchObject, gosnmp.NoSuchInstance:
		return valueobj.CollectedValue{}, fmt.Errorf("OID不存在: %s", pdu.Name)
	case gosnmp.Null:
		return defaultCollectedValueByTargetType(targetDataType), nil
	}

	rawValue, err := parseRawByOriginalType(pdu, originalType)
	if err != nil {
		return valueobj.CollectedValue{}, fmt.Errorf("按原始类型解析失败: OriginalType=%s, PDU=%v, err=%w", originalType, pdu.Type, err)
	}

	return convertRawToTargetValue(rawValue, targetDataType)
}

func defaultCollectedValueByTargetType(targetDataType string) valueobj.CollectedValue {
	switch targetDataType {
	case valueobj.DataTypeSTRING:
		return valueobj.NewTextValue("")
	case valueobj.DataTypeBOOL:
		return valueobj.NewBoolValue(false)
	default:
		return valueobj.NewNumberValue(0)
	}
}

func parseRawByOriginalType(pdu gosnmp.SnmpPDU, originalType string) (interface{}, error) {
	switch originalType {
	case "INTEGER", "INTEGER32":
		if pdu.Type != gosnmp.Integer {
			return nil, fmt.Errorf("PDU类型不匹配，期望 Integer，实际 %v", pdu.Type)
		}
		return gosnmp.ToBigInt(pdu.Value).Int64(), nil
	case "UNSIGNED32", "UINTEGER32", "GAUGE", "GAUGE32", "COUNTER", "COUNTER32", "TIMETICKS":
		if pdu.Type != gosnmp.Counter32 && pdu.Type != gosnmp.Gauge32 &&
			pdu.Type != gosnmp.TimeTicks && pdu.Type != gosnmp.Uinteger32 {
			return nil, fmt.Errorf("PDU类型不匹配，期望无符号32位计数类型，实际 %v", pdu.Type)
		}
		return gosnmp.ToBigInt(pdu.Value).Uint64(), nil
	case "COUNTER64":
		if pdu.Type != gosnmp.Counter64 {
			return nil, fmt.Errorf("PDU类型不匹配，期望 Counter64，实际 %v", pdu.Type)
		}
		return gosnmp.ToBigInt(pdu.Value).Uint64(), nil
	case "OCTETSTRING", "DISPLAYSTRING":
		if pdu.Type != gosnmp.OctetString {
			return nil, fmt.Errorf("PDU类型不匹配，期望 OctetString，实际 %v", pdu.Type)
		}
		if b, ok := pdu.Value.([]byte); ok {
			return string(b), nil
		}
		return fmt.Sprint(pdu.Value), nil
	case "OBJECTIDENTIFIER", "OID":
		if pdu.Type != gosnmp.ObjectIdentifier {
			return nil, fmt.Errorf("PDU类型不匹配，期望 ObjectIdentifier，实际 %v", pdu.Type)
		}
		return fmt.Sprint(pdu.Value), nil
	case "IPADDRESS":
		if pdu.Type != gosnmp.IPAddress {
			return nil, fmt.Errorf("PDU类型不匹配，期望 IPAddress，实际 %v", pdu.Type)
		}
		return fmt.Sprint(pdu.Value), nil
	case "OPAQUEFLOAT":
		if pdu.Type != gosnmp.OpaqueFloat {
			return nil, fmt.Errorf("PDU类型不匹配，期望 OpaqueFloat，实际 %v", pdu.Type)
		}
		return float64(pdu.Value.(float32)), nil
	case "OPAQUEDOUBLE":
		if pdu.Type != gosnmp.OpaqueDouble {
			return nil, fmt.Errorf("PDU类型不匹配，期望 OpaqueDouble，实际 %v", pdu.Type)
		}
		return pdu.Value.(float64), nil
	case "BOOLEAN", "BITS":
		if pdu.Type != gosnmp.Boolean {
			return nil, fmt.Errorf("PDU类型不匹配，期望 Boolean，实际 %v", pdu.Type)
		}
		return pdu.Value.(bool), nil
	default:
		// 兜底按 PDU 推断（仅用于空配置或异常配置）
		switch pdu.Type {
		case gosnmp.Integer:
			return gosnmp.ToBigInt(pdu.Value).Int64(), nil
		case gosnmp.Counter32, gosnmp.Gauge32, gosnmp.TimeTicks, gosnmp.Uinteger32:
			return gosnmp.ToBigInt(pdu.Value).Uint64(), nil
		case gosnmp.Counter64:
			return gosnmp.ToBigInt(pdu.Value).Uint64(), nil
		case gosnmp.OctetString:
			if b, ok := pdu.Value.([]byte); ok {
				return string(b), nil
			}
			return fmt.Sprint(pdu.Value), nil
		case gosnmp.ObjectIdentifier, gosnmp.IPAddress:
			return fmt.Sprint(pdu.Value), nil
		case gosnmp.OpaqueFloat:
			return float64(pdu.Value.(float32)), nil
		case gosnmp.OpaqueDouble:
			return pdu.Value.(float64), nil
		case gosnmp.Boolean:
			return pdu.Value.(bool), nil
		default:
			return pdu.Value, nil
		}
	}
}

func resolveSNMPPointTypes(point templatebiz.DevicePoint, pduType gosnmp.Asn1BER) (string, string) {
	originalType := snmpproto.NormalizeOriginalType(point.OriginalType)
	if originalType == "" {
		originalType = inferOriginalTypeByPDU(pduType)
	}

	targetDataType := valueobj.NormalizeDataType(point.DataType)
	if targetDataType == "" {
		targetDataType = snmpproto.ToDataType(originalType)
	}
	if targetDataType == "" {
		targetDataType = snmpproto.ToDataType(inferOriginalTypeByPDU(pduType))
	}
	return originalType, targetDataType
}

func inferOriginalTypeByPDU(pduType gosnmp.Asn1BER) string {
	switch pduType {
	case gosnmp.Integer:
		return "INTEGER32"
	case gosnmp.Counter32:
		return "COUNTER32"
	case gosnmp.Gauge32:
		return "GAUGE32"
	case gosnmp.TimeTicks:
		return "TIMETICKS"
	case gosnmp.Uinteger32:
		return "UINTEGER32"
	case gosnmp.Counter64:
		return "COUNTER64"
	case gosnmp.OctetString:
		return "OCTETSTRING"
	case gosnmp.ObjectIdentifier:
		return "OBJECTIDENTIFIER"
	case gosnmp.IPAddress:
		return "IPADDRESS"
	case gosnmp.OpaqueFloat:
		return "OPAQUEFLOAT"
	case gosnmp.OpaqueDouble:
		return "OPAQUEDOUBLE"
	case gosnmp.Boolean:
		return "BOOLEAN"
	default:
		return ""
	}
}

func convertRawToTargetValue(raw interface{}, targetDataType string) (valueobj.CollectedValue, error) {
	switch targetDataType {
	case valueobj.DataTypeBOOL:
		v, err := parseBooleanValue(raw)
		if err != nil {
			return valueobj.CollectedValue{}, err
		}
		return valueobj.NewBoolValue(v), nil
	case valueobj.DataTypeSTRING:
		if s, ok := raw.(string); ok {
			return valueobj.NewTextValue(s), nil
		}
		if b, ok := raw.([]byte); ok {
			return valueobj.NewTextValue(string(b)), nil
		}
		return valueobj.NewTextValue(fmt.Sprint(raw)), nil
	case valueobj.DataTypeINT64:
		v, err := convertToIntRange(raw, math.MinInt64, math.MaxInt64)
		if err != nil {
			return valueobj.CollectedValue{}, err
		}
		return valueobj.NewNumberValue(v), nil
	case valueobj.DataTypeFLOAT64:
		if v, ok := toFloat64(raw); ok {
			return valueobj.NewNumberValue(v), nil
		}
		if s, ok := raw.(string); ok {
			fv, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
			if err != nil {
				return valueobj.CollectedValue{}, fmt.Errorf("无法解析字符串为FLOAT64: %s", s)
			}
			return valueobj.NewNumberValue(fv), nil
		}
		return valueobj.CollectedValue{}, fmt.Errorf("不支持的FLOAT64类型: %T", raw)
	default:
		if v, ok := toFloat64(raw); ok {
			return valueobj.NewNumberValue(v), nil
		}
		return valueobj.CollectedValue{}, fmt.Errorf("未知最终值类型: %s", targetDataType)
	}
}

func convertToIntRange(raw interface{}, min, max float64) (float64, error) {
	fv, err := parseNumeric(raw)
	if err != nil {
		return 0, err
	}
	if fv < min || fv > max {
		return 0, fmt.Errorf("数值超出范围 [%.0f, %.0f]: %v", min, max, fv)
	}
	return math.Trunc(fv), nil
}

func convertToUintRange(raw interface{}, max float64) (float64, error) {
	fv, err := parseNumeric(raw)
	if err != nil {
		return 0, err
	}
	if fv < 0 || fv > max {
		return 0, fmt.Errorf("无符号数值超出范围 [0, %.0f]: %v", max, fv)
	}
	return math.Trunc(fv), nil
}

func parseNumeric(raw interface{}) (float64, error) {
	if v, ok := toFloat64(raw); ok {
		return v, nil
	}
	if s, ok := raw.(string); ok {
		fv, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
		if err != nil {
			return 0, fmt.Errorf("无法解析字符串为数值: %s", s)
		}
		return fv, nil
	}
	return 0, fmt.Errorf("不支持的数值类型: %T", raw)
}

func parseBooleanValue(raw interface{}) (bool, error) {
	switch v := raw.(type) {
	case bool:
		return v, nil
	case string:
		s := strings.ToLower(strings.TrimSpace(v))
		switch s {
		case "1", "true", "yes", "on":
			return true, nil
		case "0", "false", "no", "off":
			return false, nil
		default:
			return false, fmt.Errorf("无法解析布尔字符串: %s", v)
		}
	default:
		if fv, ok := toFloat64(raw); ok {
			return fv != 0, nil
		}
		return false, fmt.Errorf("无法解析布尔值类型: %T", raw)
	}
}

func toFloat64(raw interface{}) (float64, bool) {
	switch v := raw.(type) {
	case int:
		return float64(v), true
	case int32:
		return float64(v), true
	case int64:
		return float64(v), true
	case uint:
		return float64(v), true
	case uint32:
		return float64(v), true
	case uint64:
		return float64(v), true
	case float32:
		return float64(v), true
	case float64:
		return v, true
	case []byte:
		fv, err := strconv.ParseFloat(strings.TrimSpace(string(v)), 64)
		if err == nil {
			return fv, true
		}
		return 0, false
	default:
		return 0, false
	}
}

func (c *SNMPCollector) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.client != nil && c.connected {
		c.client.Conn.Close()
		c.connected = false
		logger.Infof("[SNMPCollector] 已停止采集")
	}
}

// Write 将原始类型值通过 SNMP Set 写入设备。
// 按测点模板中的 OriginalType 确定 PDU 类型，构建 gosnmp.SnmpPDU 并调用 Set。
// 每次 Write 创建独立连接，写入后立即关闭，不依赖预建立的 c.client。
func (c *SNMPCollector) Write(ctx context.Context, deviceID int, point templatebiz.DevicePoint, rawValue interface{}) error {
	spec := point.SNMP
	if spec == nil {
		return fmt.Errorf("测点 %d 无 SNMP 配置", point.PointID)
	}

	pduType, err := c.originalTypeToPDUType(point.OriginalType)
	if err != nil {
		return err
	}

	// 按 PDU 类型将 rawValue 转换为 gosnmp 需要的 Go 类型。
	// reverseToRaw 返回到 rawValue 可能只是 int64，而 gosnmp 对不同类型的 Set 要求
	// 不同的 Go 类型（如 Counter32 要 uint32、Counter64 要 uint64、OpaqueFloat 要 float32 等）。
	switch pduType {
	case gosnmp.Integer, gosnmp.Boolean:
		switch v := rawValue.(type) {
		case int64:
			rawValue = int(v)
		case float64:
			rawValue = int(v)
		case bool:
			if v {
				rawValue = 1
			} else {
				rawValue = 0
			}
		}
	case gosnmp.Counter32, gosnmp.Gauge32, gosnmp.TimeTicks, gosnmp.Uinteger32:
		if v, ok := rawValue.(int64); ok {
			rawValue = uint32(v)
		}
	case gosnmp.Counter64:
		if v, ok := rawValue.(int64); ok {
			rawValue = uint64(v)
		}
	case gosnmp.OpaqueFloat:
		if v, ok := rawValue.(float64); ok {
			rawValue = float32(v)
		} else if v, ok := rawValue.(int64); ok {
			rawValue = float32(v)
		}
	case gosnmp.OpaqueDouble:
		if v, ok := rawValue.(float64); ok {
			rawValue = v
		}
	case gosnmp.OctetString:
		if v, ok := rawValue.(int64); ok {
			rawValue = fmt.Sprintf("%d", v)
		}
	}

	// 从设备参数解析连接配置
	ipAddress := c.params["ip_address"]
	if ipAddress == "" {
		return fmt.Errorf("SNMP写入: 缺少ip_address参数")
	}
	port := c.params["port"]
	if port == "" {
		port = "161"
	}
	community := c.params["community"]
	if community == "" {
		community = "public"
	}
	snmpVersion := c.params["snmp_v"]
	if snmpVersion == "" {
		snmpVersion = "2"
	}
	timeout := 5 * time.Second
	if t := c.params["timeout"]; t != "" {
		if ms, err := strconv.Atoi(t); err == nil && ms > 0 {
			timeout = time.Duration(ms) * time.Millisecond
		}
	}
	retries := 0
	if r := c.params["retry_cnt"]; r != "" {
		if v, err := strconv.Atoi(r); err == nil && v >= 0 {
			retries = v
		}
	}

	portInt, _ := strconv.Atoi(port)
	var version gosnmp.SnmpVersion
	switch snmpVersion {
	case "1":
		version = gosnmp.Version1
	case "2", "2c":
		version = gosnmp.Version2c
	case "3":
		return c.writeV3(ctx, ipAddress, portInt, timeout, retries, spec, pduType, rawValue)
	default:
		version = gosnmp.Version2c
	}

	client := &gosnmp.GoSNMP{
		Target:    ipAddress,
		Port:      uint16(portInt),
		Version:   version,
		Community: community,
		Timeout:   timeout,
		Retries:   retries,
	}

	if err := client.Connect(); err != nil {
		return fmt.Errorf("SNMP连接失败: %w", err)
	}
	defer client.Conn.Close()

	pdu := gosnmp.SnmpPDU{
		Name:  spec.OID,
		Type:  pduType,
		Value: rawValue,
	}

	_, err = client.Set([]gosnmp.SnmpPDU{pdu})
	return err
}

// writeV3 处理 SNMPv3 Set 请求，按安全参数建立连接后写入。
func (c *SNMPCollector) writeV3(ctx context.Context, ipAddress string, port int, timeout time.Duration, retries int, spec *templatebiz.SNMPPointSpec, pduType gosnmp.Asn1BER, rawValue interface{}) error {
	client, err := c.buildV3Client(ipAddress, port, timeout, retries, c.params)
	if err != nil {
		return err
	}

	if err := client.Connect(); err != nil {
		return fmt.Errorf("SNMPv3连接失败: %w", err)
	}
	defer client.Conn.Close()

	pdu := gosnmp.SnmpPDU{
		Name:  spec.OID,
		Type:  pduType,
		Value: rawValue,
	}

	_, err = client.Set([]gosnmp.SnmpPDU{pdu})
	return err
}

// originalTypeToPDUType 将模板定义的 SNMP 原始类型映射为 gosnmp PDU 类型常量。
func (c *SNMPCollector) originalTypeToPDUType(originalType string) (gosnmp.Asn1BER, error) {
	normalized := snmpproto.NormalizeOriginalType(originalType)
	switch normalized {
	case "INTEGER", "INTEGER32":
		return gosnmp.Integer, nil
	case "UNSIGNED32", "UINTEGER32":
		return gosnmp.Uinteger32, nil
	case "GAUGE", "GAUGE32":
		return gosnmp.Gauge32, nil
	case "COUNTER", "COUNTER32":
		return gosnmp.Counter32, nil
	case "COUNTER64":
		return gosnmp.Counter64, nil
	case "TIMETICKS":
		return gosnmp.TimeTicks, nil
	case "OCTETSTRING", "DISPLAYSTRING":
		return gosnmp.OctetString, nil
	case "OBJECTIDENTIFIER", "OID":
		return gosnmp.ObjectIdentifier, nil
	case "IPADDRESS":
		return gosnmp.IPAddress, nil
	case "OPAQUEFLOAT":
		return gosnmp.OpaqueFloat, nil
	case "OPAQUEDOUBLE":
		return gosnmp.OpaqueDouble, nil
	case "BOOLEAN", "BITS":
		return gosnmp.Boolean, nil
	default:
		// 未知类型兜底到 OctetString，仍尝试写入但通知调用方。
		return gosnmp.OctetString, fmt.Errorf("未知 SNMP 原始类型: %s，使用 OctetString 兜底", originalType)
	}
}

func (c *SNMPCollector) Walk(deviceID int, rootOID string, params map[string]string) (map[string]interface{}, error) {
	result := make(map[string]interface{})

	ipAddress := params["ip_address"]
	if ipAddress == "" {
		return nil, fmt.Errorf("SNMP采集器: 缺少ip_address参数")
	}

	port := params["port"]
	if port == "" {
		port = "161"
	}

	community := params["community"]
	if community == "" {
		community = "public"
	}

	portInt, _ := strconv.Atoi(port)

	client := &gosnmp.GoSNMP{
		Target:    ipAddress,
		Port:      uint16(portInt),
		Version:   gosnmp.Version2c,
		Community: community,
		Timeout:   5 * time.Second,
		Retries:   0,
		MaxOids:   gosnmp.MaxOids,
	}

	if err := client.Connect(); err != nil {
		return nil, fmt.Errorf("SNMP连接失败: %w", err)
	}
	defer client.Conn.Close()

	err := client.Walk(rootOID, func(pdu gosnmp.SnmpPDU) error {
		value, err := c.parsePDUValueSimple(pdu)
		if err != nil {
			logger.Warnf("[SNMPCollector] Walk解析失败: %v", err)
			return nil
		}
		result[pdu.Name] = value
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("SNMP Walk失败: %w", err)
	}

	return result, nil
}

func (c *SNMPCollector) parsePDUValueSimple(pdu gosnmp.SnmpPDU) (interface{}, error) {
	switch pdu.Type {
	case gosnmp.Integer:
		return gosnmp.ToBigInt(pdu.Value).Int64(), nil
	case gosnmp.OctetString:
		return string(pdu.Value.([]byte)), nil
	case gosnmp.ObjectIdentifier:
		return pdu.Value.(string), nil
	case gosnmp.IPAddress:
		return pdu.Value.(string), nil
	case gosnmp.Counter32, gosnmp.Gauge32, gosnmp.TimeTicks, gosnmp.Counter64, gosnmp.Uinteger32:
		return gosnmp.ToBigInt(pdu.Value).Int64(), nil
	case gosnmp.OpaqueFloat:
		return pdu.Value.(float32), nil
	case gosnmp.OpaqueDouble:
		return pdu.Value.(float64), nil
	case gosnmp.Boolean:
		return pdu.Value.(bool), nil
	case gosnmp.NoSuchObject, gosnmp.NoSuchInstance:
		return nil, fmt.Errorf("OID不存在")
	case gosnmp.Null:
		return nil, nil
	default:
		return pdu.Value, nil
	}
}
