package snmp

import (
	"testing"

	"data-collection-system/internal/valueobj"
)

func TestNormalizeOriginalType(t *testing.T) {
	cases := map[string]string{
		"OctetString": "OCTETSTRING",
		"TimeTicks":   "TIMETICKS",
		"Counter64":   "COUNTER64",
		"unknown":     "",
	}

	for input, expected := range cases {
		if got := NormalizeOriginalType(input); got != expected {
			t.Fatalf("NormalizeOriginalType(%q): expected %q, got %q", input, expected, got)
		}
	}
}

func TestToDataType(t *testing.T) {
	cases := map[string]string{
		"INTEGER32":   valueobj.DataTypeINT64,
		"Unsigned32":  valueobj.DataTypeINT64,
		"OctetString": valueobj.DataTypeSTRING,
		"Boolean":     valueobj.DataTypeBOOL,
		"unknown":     "",
	}

	for input, expected := range cases {
		if got := ToDataType(input); got != expected {
			t.Fatalf("ToDataType(%q): expected %q, got %q", input, expected, got)
		}
	}
}
