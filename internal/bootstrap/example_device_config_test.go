package bootstrap

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/goccy/go-yaml"
)

// TestDeployExampleDeviceConfig 校验示例 device.yaml 能通过启动校验。
// 该文件是仓库根目录下的出厂配置 config/device.yaml；文件缺失即视为部署产物缺失，直接失败。
func TestDeployExampleDeviceConfig(t *testing.T) {
	path := filepath.Join("..", "..", "config", "device.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("出厂示例配置缺失（应由 config/device.yaml 提供）: %v", err)
	}

	var settings DeviceSettings
	if err := yaml.Unmarshal(data, &settings); err != nil {
		t.Fatalf("示例配置 YAML 解析失败: %v", err)
	}
	if err := validateDeviceSettings(settings); err != nil {
		t.Fatalf("示例配置未通过 validateDeviceSettings: %v", err)
	}
	if len(settings.Protocols["rs485_rtu"]) == 0 {
		t.Fatalf("示例配置应至少包含一个 rs485_rtu 端点")
	}
}
