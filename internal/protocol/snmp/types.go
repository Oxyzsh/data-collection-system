package snmp

import (
	"strings"

	"data-collection-system/internal/valueobj"
)

// NormalizeOriginalType 归一化 SNMP 原始类型（保留 SNMP 语义名称）
// 例如：OctetString -> OCTETSTRING，TimeTicks -> TIMETICKS
func NormalizeOriginalType(s string) string {
	upper := strings.ToUpper(strings.TrimSpace(s))
	if upper == "" {
		return ""
	}
	normalizedKey := strings.NewReplacer(" ", "", "_", "", "-", "").Replace(upper)
	if canonical, ok := originalTypeCanonical[normalizedKey]; ok {
		return canonical
	}
	return ""
}

// ToDataType 将 SNMP 原始类型映射到系统最终值类型。
func ToDataType(snmpOriginalType string) string {
	if snmpOriginalType == "" {
		return ""
	}
	normalizedKey := strings.NewReplacer(" ", "", "_", "", "-", "").Replace(strings.ToUpper(strings.TrimSpace(snmpOriginalType)))
	if mapped, ok := originalTypeToDataType[normalizedKey]; ok {
		return mapped
	}
	return ""
}

var originalTypeToDataType = map[string]string{
	"INTEGER":          valueobj.DataTypeINT64,
	"INTEGER32":        valueobj.DataTypeINT64,
	"UNSIGNED32":       valueobj.DataTypeINT64,
	"UINTEGER32":       valueobj.DataTypeINT64,
	"GAUGE":            valueobj.DataTypeINT64,
	"GAUGE32":          valueobj.DataTypeINT64,
	"COUNTER":          valueobj.DataTypeINT64,
	"COUNTER32":        valueobj.DataTypeINT64,
	"COUNTER64":        valueobj.DataTypeINT64,
	"TIMETICKS":        valueobj.DataTypeINT64,
	"OCTETSTRING":      valueobj.DataTypeSTRING,
	"DISPLAYSTRING":    valueobj.DataTypeSTRING,
	"OBJECTIDENTIFIER": valueobj.DataTypeSTRING,
	"OID":              valueobj.DataTypeSTRING,
	"IPADDRESS":        valueobj.DataTypeSTRING,
	"OPAQUEFLOAT":      valueobj.DataTypeFLOAT64,
	"OPAQUEDOUBLE":     valueobj.DataTypeFLOAT64,
	"BOOLEAN":          valueobj.DataTypeBOOL,
	"BITS":             valueobj.DataTypeBOOL,
}

var originalTypeCanonical = map[string]string{
	"INTEGER":          "INTEGER",
	"INTEGER32":        "INTEGER32",
	"UNSIGNED32":       "UNSIGNED32",
	"UINTEGER32":       "UINTEGER32",
	"GAUGE":            "GAUGE",
	"GAUGE32":          "GAUGE32",
	"COUNTER":          "COUNTER",
	"COUNTER32":        "COUNTER32",
	"COUNTER64":        "COUNTER64",
	"TIMETICKS":        "TIMETICKS",
	"OCTETSTRING":      "OCTETSTRING",
	"DISPLAYSTRING":    "DISPLAYSTRING",
	"OBJECTIDENTIFIER": "OBJECTIDENTIFIER",
	"OID":              "OID",
	"IPADDRESS":        "IPADDRESS",
	"OPAQUEFLOAT":      "OPAQUEFLOAT",
	"OPAQUEDOUBLE":     "OPAQUEDOUBLE",
	"BOOLEAN":          "BOOLEAN",
	"BITS":             "BITS",
}
