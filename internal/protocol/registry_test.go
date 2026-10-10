package protocol

import "testing"

func TestRegisterProtocolAndValidator(t *testing.T) {
	customType := ProtocolType("UNIT-TEST-PROTOCOL")

	RegisterProtocol(ProtocolSpec{
		Info: ProtocolInfo{
			Type:     customType,
			NameCN:   "测试协议",
			NameEN:   "Unit Test Protocol",
			Category: "test",
		},
		Params: []ParamDefinition{{Name: "foo", Type: ParamTypeString, Required: false, Default: "bar"}},
		Validator: func(params map[string]string) []ValidationError {
			if params["foo"] == "ok" {
				return nil
			}
			return []ValidationError{{Param: "foo", Message: "foo must be ok"}}
		},
	})

	params := GetProtocolParams(customType)
	if len(params) != 1 || params[0].Name != "foo" {
		t.Fatalf("unexpected params: %+v", params)
	}

	err := ValidateDeviceParams(customType, map[string]string{"foo": "bad"})
	if len(err) == 0 {
		t.Fatal("expected custom validator error")
	}
}
