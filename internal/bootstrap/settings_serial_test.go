package bootstrap

import (
	"os"
	"testing"
)

func TestDeviceConfigSerialPortNames(t *testing.T) {
	settings := DeviceSettings{
		Version: 1,
		Protocols: map[string][]ProtocolEndpointSettings{
			"rs485_rtu": {{Label: "COM1", Value: "/dev/ttyS1"}},
			"rs232_rtu": {{Label: "COM2", Value: "/dev/ttyUSB0"}},
		},
	}
	names := settings.SerialPortNames()
	if len(names) != 2 {
		t.Fatalf("应返回 2 个串口，got %d", len(names))
	}
	got := map[string]bool{}
	for _, name := range names {
		got[name] = true
	}
	if !got["/dev/ttyS1"] || !got["/dev/ttyUSB0"] {
		t.Fatalf("串口列表不正确: %v", names)
	}
}

func TestLoadDeviceSettings(t *testing.T) {
	// 本机若恰好存在 /etc/mmdc/device.yaml，会盖过临时文件；
	// 这里复用包内统一的可注入点屏蔽环境差异，把现场配置指向临时路径。
	setupDeviceSearch(t)
	path := systemDeviceConfigPath
	data := []byte(`version: 1
protocols:
  rs485_rtu:
    - label: COM1
      value: /dev/ttyS1
      defaults:
        baudrate: "9600"
local_io:
  di:
    - label: DI1
      value: /sys/class/gpio/gpio495/value
      active_low: true
  do:
    - label: DO1
      value: /sys/class/gpio/gpio503/value
      default: false
`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	settings, loadedPath, err := loadDeviceSettings()
	if err != nil {
		t.Fatalf("loadDeviceSettings() error = %v", err)
	}
	if loadedPath != path {
		t.Fatalf("loaded path = %q, want %q", loadedPath, path)
	}
	channel, ok := settings.LocalIOChannel("di1")
	if !ok || channel.Value != "/sys/class/gpio/gpio495/value" {
		t.Fatalf("LocalIOChannel() = %#v, %v", channel, ok)
	}
}
