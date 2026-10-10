package protocol

import "testing"

func TestGetAllProtocolTypesIncludesSNMP(t *testing.T) {
	types := GetAllProtocolTypes()
	found := false
	for _, item := range types {
		if item.Type == ProtocolSNMP {
			found = true
			if item.Category != "snmp" {
				t.Fatalf("expected snmp category, got %s", item.Category)
			}
		}
	}
	if !found {
		t.Fatal("expected SNMP protocol type to be registered")
	}
}

func TestGetProtocolParamsSNMP(t *testing.T) {
	params := GetProtocolParams(ProtocolSNMP)
	if len(params) == 0 {
		t.Fatal("expected SNMP protocol params to be defined")
	}

	foundSNMPVersion := false
	for _, param := range params {
		if param.Name == "snmp_v" {
			foundSNMPVersion = true
			if !param.Required {
				t.Fatal("expected snmp_v to be required")
			}
		}
	}
	if !foundSNMPVersion {
		t.Fatal("expected snmp_v parameter to be defined")
	}

	defaults := ApplyParamDefaults(ProtocolSNMP, map[string]string{})
	if defaults["port"] != "161" {
		t.Fatalf("expected default port 161, got %q", defaults["port"])
	}
	if defaults["community"] != "public" {
		t.Fatalf("expected default community public, got %q", defaults["community"])
	}
	if defaults["snmp_v"] != "2" {
		t.Fatalf("expected default snmp_v 2, got %q", defaults["snmp_v"])
	}
	if defaults["segment_max"] != "200" {
		t.Fatalf("expected default segment_max 200, got %q", defaults["segment_max"])
	}
}

func TestValidateDeviceParamsSNMPv3Conditions(t *testing.T) {
	params := map[string]string{
		"ip_address":     "192.168.1.10",
		"snmp_v":         "3",
		"security_level": "authPriv",
		"timeout":        "500",
		"retry_cnt":      "5",
		"interval":       "5000",
		"port":           "161",
		"segment_max":    "200",
	}

	errs := ValidateDeviceParams(ProtocolSNMP, params)
	if len(errs) == 0 {
		t.Fatal("expected SNMPv3 conditional validation errors")
	}

	required := map[string]bool{}
	for _, err := range errs {
		required[err.Param] = true
	}

	for _, param := range []string{"user_name", "auth_protocol", "auth_password", "priv_protocol", "priv_password"} {
		if !required[param] {
			t.Fatalf("expected validation error for %s", param)
		}
	}
}

func TestGetProtocolParamsModbusRTUContinuousRead(t *testing.T) {
	for _, protocolType := range []ProtocolType{ProtocolRS485RTU, ProtocolRS232RTU} {
		params := GetProtocolParams(protocolType)
		if len(params) == 0 {
			t.Fatalf("expected Modbus params for %s to be defined", protocolType)
		}

		foundMaxCount := false
		foundInterval := false
		for _, param := range params {
			switch param.Name {
			case "max_continuous_count":
				foundMaxCount = true
				if param.Min != 1 || param.Max != 2000 {
					t.Fatalf("expected max_continuous_count range 1..2000, got %.0f..%.0f", param.Min, param.Max)
				}
				if param.Default != "100" {
					t.Fatalf("expected max_continuous_count default 100, got %q", param.Default)
				}
			case "continuous_interval_ms":
				foundInterval = true
				if param.Min != 30 {
					t.Fatalf("expected continuous_interval_ms min 30, got %.0f", param.Min)
				}
				if param.Default != "30" {
					t.Fatalf("expected continuous_interval_ms default 30, got %q", param.Default)
				}
			}
		}

		if !foundMaxCount {
			t.Fatalf("expected max_continuous_count parameter to be defined for %s", protocolType)
		}
		if !foundInterval {
			t.Fatalf("expected continuous_interval_ms parameter to be defined for %s", protocolType)
		}

		defaults := ApplyParamDefaults(protocolType, map[string]string{})
		if defaults["max_continuous_count"] != "100" {
			t.Fatalf("expected max_continuous_count default 100, got %q", defaults["max_continuous_count"])
		}
		if defaults["continuous_interval_ms"] != "30" {
			t.Fatalf("expected continuous_interval_ms default 30, got %q", defaults["continuous_interval_ms"])
		}
	}
}

func TestGetProtocolParamsModbusTCPContinuousRead(t *testing.T) {
	params := GetProtocolParams(ProtocolMODBUSTCP)
	if len(params) == 0 {
		t.Fatal("expected MODBUS-TCP params to be defined")
	}

	foundMaxCount := false
	foundInterval := false
	foundSlaveAddr := false
	for _, param := range params {
		switch param.Name {
		case "max_continuous_count":
			foundMaxCount = true
		case "continuous_interval_ms":
			foundInterval = true
		case "slave_addr":
			foundSlaveAddr = true
		}
	}

	if !foundMaxCount {
		t.Fatal("expected max_continuous_count parameter to be defined for MODBUS-TCP")
	}
	if foundInterval {
		t.Fatal("did not expect continuous_interval_ms for MODBUS-TCP")
	}
	if !foundSlaveAddr {
		t.Fatal("expected slave_addr parameter to be defined for MODBUS-TCP")
	}

	defaults := ApplyParamDefaults(ProtocolMODBUSTCP, map[string]string{})
	if defaults["max_continuous_count"] != "100" {
		t.Fatalf("expected max_continuous_count default 100, got %q", defaults["max_continuous_count"])
	}
	if _, ok := defaults["continuous_interval_ms"]; ok {
		t.Fatal("did not expect continuous_interval_ms default for MODBUS-TCP")
	}
	if defaults["slave_addr"] != "1" {
		t.Fatalf("expected slave_addr default 1, got %q", defaults["slave_addr"])
	}
}

func TestGetProtocolParamsModbusTCPRTUContinuousRead(t *testing.T) {
	for _, protocolType := range []ProtocolType{ProtocolTCPRTU, ProtocolUDPRTU} {
		params := GetProtocolParams(protocolType)
		if len(params) == 0 {
			t.Fatalf("expected Modbus params for %s to be defined", protocolType)
		}

		foundMaxCount := false
		foundInterval := false
		foundBaudrate := false
		for _, param := range params {
			switch param.Name {
			case "max_continuous_count":
				foundMaxCount = true
			case "continuous_interval_ms":
				foundInterval = true
			case "baudrate":
				foundBaudrate = true
			}
		}

		if !foundMaxCount {
			t.Fatalf("expected max_continuous_count parameter to be defined for %s", protocolType)
		}
		if !foundInterval {
			t.Fatalf("expected continuous_interval_ms parameter to be defined for %s", protocolType)
		}
		if !foundBaudrate {
			t.Fatalf("expected baudrate parameter to be defined for %s", protocolType)
		}

		defaults := ApplyParamDefaults(protocolType, map[string]string{})
		if defaults["max_continuous_count"] != "100" {
			t.Fatalf("expected max_continuous_count default 100, got %q", defaults["max_continuous_count"])
		}
		if defaults["continuous_interval_ms"] != "30" {
			t.Fatalf("expected continuous_interval_ms default 30, got %q", defaults["continuous_interval_ms"])
		}
	}
}

func TestValidateDeviceParamsModbusContinuousReadLimits(t *testing.T) {
	params := map[string]string{
		"serial":               "/dev/ttyUSB0",
		"slave_addr":           "1",
		"timeout":              "500",
		"retry_cnt":            "5",
		"interval":             "5000",
		"baudrate":             "9600",
		"data_bit":             "8",
		"stop_bit":             "1",
		"parity":               "N",
		"max_continuous_count": "0",
	}

	errs := ValidateDeviceParams(ProtocolRS485RTU, params)
	if len(errs) == 0 {
		t.Fatal("expected validation errors for invalid modbus continuous read params")
	}

	required := map[string]bool{}
	for _, err := range errs {
		required[err.Param] = true
	}

	if !required["max_continuous_count"] {
		t.Fatal("expected validation error for max_continuous_count")
	}
}

func TestValidateDeviceParamsModbusContinuousIntervalScope(t *testing.T) {
	params := map[string]string{
		"ip_address":             "192.168.1.10",
		"port":                   "502",
		"slave_addr":             "1",
		"max_continuous_count":   "0",
		"continuous_interval_ms": "29",
	}

	errs := ValidateDeviceParams(ProtocolMODBUSTCP, params)
	if len(errs) == 0 {
		t.Fatal("expected validation errors for invalid MODBUS-TCP params")
	}

	foundMaxCount := false
	foundInterval := false
	for _, err := range errs {
		if err.Param == "max_continuous_count" {
			foundMaxCount = true
		}
		if err.Param == "continuous_interval_ms" {
			foundInterval = true
		}
	}
	if !foundMaxCount {
		t.Fatal("expected validation error for max_continuous_count")
	}
	if foundInterval {
		t.Fatal("did not expect continuous_interval_ms validation on MODBUS-TCP")
	}
}

// GetAllProtocolTypes 必须按注册（声明）顺序稳定返回：registry 是 map，直接遍历
// 会让前端「协议类型」下拉的顺序随机化（Go map 迭代序每次 range 重新随机）。
func TestGetAllProtocolTypesStableOrder(t *testing.T) {
	want := []ProtocolType{
		ProtocolRS485RTU, ProtocolRS232RTU, ProtocolMODBUSTCP,
		ProtocolTCPRTU, ProtocolUDPRTU, ProtocolSNMP,
	}
	for i := 0; i < 20; i++ {
		types := GetAllProtocolTypes()
		if len(types) < len(want) {
			t.Fatalf("协议数量 = %d, want >= %d", len(types), len(want))
		}
		for j, w := range want {
			if types[j].Type != w {
				t.Fatalf("第 %d 项 = %s, want %s（顺序必须稳定且按声明序）", j, types[j].Type, w)
			}
		}
	}
}
