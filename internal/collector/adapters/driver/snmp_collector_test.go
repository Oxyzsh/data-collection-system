package collector

import (
	"testing"

	templatebiz "data-collection-system/internal/template/domain"
	"data-collection-system/internal/valueobj"

	"github.com/gosnmp/gosnmp"
)

func TestParsePDUValue_OctetStringAsString_NoError(t *testing.T) {
	c := &SNMPCollector{}
	point := templatebiz.DevicePoint{
		PointID:      1,
		OriginalType: "OctetString",
		DataType:     valueobj.DataTypeSTRING,
	}
	pdu := gosnmp.SnmpPDU{
		Name:  ".1.3.6.1.2.1.1.5.0",
		Type:  gosnmp.OctetString,
		Value: []byte("li"),
	}

	got, err := c.parsePDUValue(pdu, point)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got.Kind != valueobj.KindText || got.Text != "li" {
		t.Fatalf("expected text value 'li', got kind=%s text=%q", got.Kind, got.Text)
	}
}

func TestParsePDUValue_OctetStringNumericString(t *testing.T) {
	c := &SNMPCollector{}
	point := templatebiz.DevicePoint{
		PointID:      2,
		OriginalType: "OctetString",
		DataType:     valueobj.DataTypeFLOAT64,
	}
	pdu := gosnmp.SnmpPDU{
		Name:  ".1.3.6.1.4.1.test",
		Type:  gosnmp.OctetString,
		Value: []byte("97.5"),
	}

	got, err := c.parsePDUValue(pdu, point)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got.Kind != valueobj.KindNumber || got.Number != 97.5 {
		t.Fatalf("expected numeric 97.5, got kind=%s number=%v", got.Kind, got.Number)
	}
}

func TestParsePDUValue_Counter64ToInt(t *testing.T) {
	c := &SNMPCollector{}
	point := templatebiz.DevicePoint{
		PointID:      3,
		OriginalType: "Counter64",
		DataType:     valueobj.DataTypeINT64,
	}
	pdu := gosnmp.SnmpPDU{
		Name:  ".1.3.6.1.4.1.test2",
		Type:  gosnmp.Counter64,
		Value: uint64(123456789),
	}

	got, err := c.parsePDUValue(pdu, point)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got.Kind != valueobj.KindNumber || got.Number != 123456789 {
		t.Fatalf("expected numeric 123456789, got kind=%s number=%v", got.Kind, got.Number)
	}
}

func TestParsePDUValue_BooleanToBoolDataType(t *testing.T) {
	c := &SNMPCollector{}
	point := templatebiz.DevicePoint{
		PointID:      4,
		OriginalType: "Boolean",
		DataType:     valueobj.DataTypeBOOL,
	}
	pdu := gosnmp.SnmpPDU{
		Name:  ".1.3.6.1.4.1.test3",
		Type:  gosnmp.Boolean,
		Value: true,
	}

	got, err := c.parsePDUValue(pdu, point)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got.Kind != valueobj.KindBool || !got.Bool {
		t.Fatalf("expected bool true, got kind=%s bool=%v", got.Kind, got.Bool)
	}
}

func TestParsePDUValue_UsesTargetDataTypePreferDataType(t *testing.T) {
	c := &SNMPCollector{}
	point := templatebiz.DevicePoint{
		PointID:      5,
		OriginalType: "OctetString",
		DataType:     valueobj.DataTypeBOOL,
	}
	pdu := gosnmp.SnmpPDU{
		Name:  ".1.3.6.1.4.1.target-type",
		Type:  gosnmp.OctetString,
		Value: []byte("true"),
	}

	got, err := c.parsePDUValue(pdu, point)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got.Kind != valueobj.KindBool || !got.Bool {
		t.Fatalf("expected bool true when DataType=BOOL, got kind=%s bool=%v", got.Kind, got.Bool)
	}
}
