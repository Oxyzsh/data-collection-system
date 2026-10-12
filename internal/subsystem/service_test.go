package subsystem

import "testing"

func TestParsePointStr(t *testing.T) {
	ref, err := ParsePointStr("12.34")
	if err != nil || ref.DeviceID != 12 || ref.PointID != 34 {
		t.Fatalf("ParsePointStr(12.34) = %+v, %v", ref, err)
	}
	if got := PointStr(ref.DeviceID, ref.PointID); got != "12.34" {
		t.Fatalf("PointStr() = %q", got)
	}
	for _, bad := range []string{"", "1", "1.", ".2", "a.b", "1.2.3", "-1.2"} {
		if _, err := ParsePointStr(bad); err == nil {
			t.Fatalf("ParsePointStr(%q) 应报错", bad)
		}
	}
}

func TestProtocolMapping(t *testing.T) {
	cases := []struct {
		dataType string
		want     int
	}{
		{"INT64", DataTypeInt},
		{"BOOL", DataTypeInt},
		{"FLOAT64", DataTypeFloat},
		{"STRING", DataTypeString},
	}
	for _, tc := range cases {
		if got := ProtocolDataType(tc.dataType); got != tc.want {
			t.Fatalf("ProtocolDataType(%s) = %d, want %d", tc.dataType, got, tc.want)
		}
	}
	if ProtocolRWType("r") != RWReadOnly || ProtocolRWType("rw") != RWReadWrite || ProtocolRWType("w") != RWReadWrite {
		t.Fatal("读写类型映射不符合指南 §5.3")
	}
}
