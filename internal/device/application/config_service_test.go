package application

import (
	"errors"
	"testing"

	devicedomain "data-collection-system/internal/device/domain"
	"data-collection-system/internal/protocol"
)

type fakeCatalogResolver struct {
	entry ResolvedDeviceEntry
	err   error
}

func (f fakeCatalogResolver) ResolveDeviceRef(ref string) (ResolvedDeviceEntry, error) {
	if f.err != nil {
		return ResolvedDeviceEntry{}, f.err
	}
	return f.entry, nil
}

func TestPrepareSerialParamsRequiresDeviceRef(t *testing.T) {
	_, err := prepareDeviceParamsForTemplate(devicedomain.TemplateMeta{Type: "modbus", CollectMethod: "modbus"}, protocol.ProtocolRS485RTU, "", nil, fakeCatalogResolver{}, true)
	if !errors.Is(err, ErrParamValidation) {
		t.Fatalf("err = %v, want ErrParamValidation", err)
	}
}

func TestPrepareSerialParamsMergesCatalog(t *testing.T) {
	resolver := fakeCatalogResolver{entry: ResolvedDeviceEntry{
		Ref:      "serial:test",
		Selector: "rs485_rtu",
		Kind:     "serial",
		Params: map[string]string{
			"device_ref":    "serial:test",
			"serial":        "/dev/ttyCH9344USB9",
			"protocol_type": "RS485-RTU",
			"baudrate":      "9600",
		},
		Defaults: map[string]string{"baudrate": "9600"},
	}}
	params, err := prepareDeviceParamsForTemplate(devicedomain.TemplateMeta{Type: "modbus", CollectMethod: "modbus"}, protocol.ProtocolRS485RTU, "serial:test", map[string]interface{}{"slave_addr": 1}, resolver, true)
	if err != nil {
		t.Fatal(err)
	}
	if params["serial"] != "/dev/ttyCH9344USB9" || params["slave_addr"] != "1" || params["baudrate"] != "9600" {
		t.Fatalf("params = %#v", params)
	}
}

func TestPrepareSerialRejectsFixedOverride(t *testing.T) {
	resolver := fakeCatalogResolver{entry: ResolvedDeviceEntry{
		Ref:      "serial:test",
		Selector: "rs485_rtu",
		Kind:     "serial",
		Params:   map[string]string{"serial": "/dev/ttyCH9344USB9"},
	}}
	_, err := prepareDeviceParamsForTemplate(devicedomain.TemplateMeta{Type: "modbus", CollectMethod: "modbus"}, protocol.ProtocolRS485RTU, "serial:test", map[string]interface{}{"serial": "/dev/ttyS0"}, resolver, true)
	if !errors.Is(err, ErrParamValidation) {
		t.Fatalf("err = %v, want ErrParamValidation", err)
	}
}

func TestPrepareLocalIOParams(t *testing.T) {
	resolver := fakeCatalogResolver{entry: ResolvedDeviceEntry{
		Ref:      "local_io:onboard",
		Selector: "local_io",
		Kind:     "gpio",
		Channels: map[string]string{"di1": "input", "do1": "output"},
		Params: map[string]string{
			"device_ref":     "local_io:onboard",
			"ref_version":    "1",
			"protocol_type":  "local_io",
			"collect_method": "gpio",
			"io_summary":     "6DI/2DO",
		},
	}}
	params, err := prepareDeviceParamsForTemplate(devicedomain.TemplateMeta{Type: "local_io", CollectMethod: "gpio", LocalIOChannels: []devicedomain.TemplateIOPointMeta{{Channel: "AUTO", Access: "rw"}}}, protocol.ProtocolType("local_io"), "local_io:onboard", map[string]interface{}{"io_direction": "di", "io_channel": "DI1", "io_alias_0_zh": "断开", "io_alias_1_zh": "闭合"}, resolver, true)
	if err != nil {
		t.Fatal(err)
	}
	if params["collect_method"] != "gpio" || params["device_ref"] != "local_io:onboard" {
		t.Fatalf("params = %#v", params)
	}
	if params["io_value_mapping_zh"] != "0:断开,1:闭合" {
		t.Fatalf("mapping = %#v", params)
	}
}

func TestPrepareLegacySerialParamsWithoutRef(t *testing.T) {
	params, err := prepareDeviceParamsForTemplate(devicedomain.TemplateMeta{Type: "modbus", CollectMethod: "modbus"}, protocol.ProtocolRS485RTU, "", map[string]interface{}{"serial": "/dev/ttyS0"}, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if params["serial"] != "/dev/ttyS0" {
		t.Fatalf("params = %#v", params)
	}
}

func TestPrepareLocalIORejectsMissingChannel(t *testing.T) {
	resolver := fakeCatalogResolver{entry: ResolvedDeviceEntry{
		Ref:      "local_io:onboard",
		Selector: "local_io",
		Kind:     "gpio",
		Channels: map[string]string{"di1": "input"},
		Params:   map[string]string{"collect_method": "gpio"},
	}}
	_, err := prepareDeviceParamsForTemplate(devicedomain.TemplateMeta{
		Type: "local_io", CollectMethod: "gpio",
		LocalIOChannels: []devicedomain.TemplateIOPointMeta{{Channel: "DO1", Access: "rw"}},
	}, protocol.ProtocolType("local_io"), "local_io:onboard", nil, resolver, true)
	if !errors.Is(err, ErrParamValidation) {
		t.Fatalf("err = %v, want ErrParamValidation", err)
	}
}

func TestPrepareLocalDITemplateSetsDirection(t *testing.T) {
	resolver := fakeCatalogResolver{entry: ResolvedDeviceEntry{
		Ref:      "local_io:onboard",
		Selector: "local_io",
		Kind:     "gpio",
		Channels: map[string]string{"di1": "input"},
		Params: map[string]string{
			"device_ref":     "local_io:onboard",
			"collect_method": "gpio",
		},
	}}
	params, err := prepareDeviceParamsForTemplate(devicedomain.TemplateMeta{
		Type: "local_di", CollectMethod: "gpio",
		LocalIOChannels: []devicedomain.TemplateIOPointMeta{{Channel: "AUTO", Access: "r"}},
	}, protocol.ProtocolType("local_di"), "local_io:onboard", map[string]interface{}{"io_channel": "DI1"}, resolver, true)
	if err != nil {
		t.Fatal(err)
	}
	if params["io_direction"] != "di" {
		t.Fatalf("params = %#v", params)
	}
}

func TestPrepareLocalDIEmptyAliases(t *testing.T) {
	resolver := fakeCatalogResolver{entry: ResolvedDeviceEntry{
		Ref:      "local_io:onboard",
		Selector: "local_io",
		Kind:     "gpio",
		Channels: map[string]string{"di1": "input"},
		Params:   map[string]string{"device_ref": "local_io:onboard", "collect_method": "gpio"},
	}}
	params, err := prepareDeviceParamsForTemplate(devicedomain.TemplateMeta{
		Type: "local_di", CollectMethod: "gpio",
		LocalIOChannels: []devicedomain.TemplateIOPointMeta{{Channel: "AUTO", Access: "r"}},
	}, protocol.ProtocolType("local_di"), "local_io:onboard", map[string]interface{}{"io_channel": "DI1"}, resolver, true)
	if err != nil {
		t.Fatal(err)
	}
	if params["io_alias_0_zh"] != "" || params["io_alias_1_en"] != "" || params["io_value_mapping_zh"] != "" {
		t.Fatalf("params = %#v", params)
	}
}

func TestPrepareSerialWithoutCatalogDefaultsKeepsProtocolDefaults(t *testing.T) {
	resolver := fakeCatalogResolver{entry: ResolvedDeviceEntry{
		Ref:      "serial:test",
		Selector: "rs485_rtu",
		Kind:     "serial",
		Params:   map[string]string{"device_ref": "serial:test", "serial": "/dev/ttyCH9344USB9", "protocol_type": "RS485-RTU"},
	}}
	params, err := prepareDeviceParamsForTemplate(devicedomain.TemplateMeta{Type: "modbus", CollectMethod: "modbus"}, protocol.ProtocolRS485RTU, "serial:test", nil, resolver, true)
	if err != nil {
		t.Fatal(err)
	}
	if params["baudrate"] != "9600" || params["data_bit"] != "8" || params["stop_bit"] != "1" || params["parity"] != "N" {
		t.Fatalf("params = %#v", params)
	}
}
