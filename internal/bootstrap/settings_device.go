package bootstrap

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/goccy/go-yaml"
)

const (
	defaultDeviceConfigFileName = "device.yaml"
	// deviceConfigSubdir 是版本自带默认配置在程序目录下的子目录。
	deviceConfigSubdir = "config"
	// defaultSystemDeviceConfigPath 是现场配置的固定位置：存在则优先于程序目录下的默认配置。
	// 放在程序目录之外，升级与恢复出厂都天然碰不到它。
	defaultSystemDeviceConfigPath = "/etc/mmdc/device.yaml"
)

// 以下两个可注入点抽成包级变量，供测试指向临时目录。
var (
	systemDeviceConfigPath = defaultSystemDeviceConfigPath
	deviceExecutableDir    = executableDir
)

// DeviceSettings 是 device.yaml 的启动期只读快照。
// 串口协议端点与本地 GPIO 通道都只从这里加载。
type DeviceSettings struct {
	Version   int                                   `yaml:"version"`
	Protocols map[string][]ProtocolEndpointSettings `yaml:"protocols"`
	LocalIO   LocalIOSettings                       `yaml:"local_io"`
}

// ProtocolEndpointSettings 描述某协议下可选的一个物理端点。
type ProtocolEndpointSettings struct {
	Label    string            `yaml:"label"`
	Value    string            `yaml:"value"`
	Defaults map[string]string `yaml:"defaults"`
}

// LocalIOSettings 描述当前板载本地 IO 控制器。
type LocalIOSettings struct {
	DI []LocalIOChannelSettings `yaml:"di"`
	DO []LocalIOChannelSettings `yaml:"do"`
}

// LocalIOChannelSettings 描述一个 DI/DO 通道。
// Value 直接指向 sysfs 等实际 value 设备路径，采集器不感知 GPIO chip/line。
type LocalIOChannelSettings struct {
	Label     string `yaml:"label"`
	Value     string `yaml:"value"`
	ActiveLow bool   `yaml:"active_low"`
	Default   *bool  `yaml:"default"`
}

// SerialPortNames 返回允许用于串口配置的物理端口名。
func (d DeviceSettings) SerialPortNames() []string {
	protocols := sortedProtocolKeys(d.Protocols)
	seen := map[string]struct{}{}
	names := make([]string, 0)
	for _, protocol := range protocols {
		if !isSerialProtocol(protocol) {
			continue
		}
		for _, endpoint := range d.Protocols[protocol] {
			value := strings.TrimSpace(endpoint.Value)
			if value == "" {
				continue
			}
			if _, ok := seen[value]; ok {
				continue
			}
			seen[value] = struct{}{}
			names = append(names, value)
		}
	}
	return names
}

// LocalIOChannel 按 label 查找本地 IO 通道。
// 模板只保存通道名，运行态通过该入口解析真实 value 路径。
func (d DeviceSettings) LocalIOChannel(label string) (LocalIOChannelSettings, bool) {
	normalized := normalizeDeviceLabel(label)
	for _, channel := range d.LocalIO.DI {
		if normalizeDeviceLabel(channel.Label) == normalized {
			return channel, true
		}
	}
	for _, channel := range d.LocalIO.DO {
		if normalizeDeviceLabel(channel.Label) == normalized {
			return channel, true
		}
	}
	return LocalIOChannelSettings{}, false
}

func loadDeviceSettings() (DeviceSettings, string, error) {
	path := findDeviceConfigFile()
	if path == "" {
		return DeviceSettings{}, "", fmt.Errorf("device config file %s not found", defaultDeviceConfigFileName)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return DeviceSettings{}, path, fmt.Errorf("read device config %s: %w", path, err)
	}

	// 三类错误都把实际路径带进 err：启动期 panic 只打印 err 本身，
	// 而 device.yaml 有两个候选位置，不点明读的是哪一个，
	// 现场会以为生效的是另一个文件（改了配置却没变化）。
	var settings DeviceSettings
	if err := yaml.Unmarshal(data, &settings); err != nil {
		return DeviceSettings{}, path, fmt.Errorf("parse device config %s: %w", path, err)
	}
	if err := validateDeviceSettings(settings); err != nil {
		return DeviceSettings{}, path, fmt.Errorf("invalid device config %s: %w", path, err)
	}
	return settings, path, nil
}

// findDeviceConfigFile 只认两个位置，先找到先用：
//
//  1. /etc/mmdc/device.yaml —— 现场配置，升级与恢复出厂都不会碰它
//  2. <程序目录>/config/device.yaml —— 版本自带默认
//
// 两处都没有则返回空串，由调用方报 not found 并终止启动。
// 这里刻意不做环境变量覆盖、也不检索程序目录及其上级的旧位置：
// 候选越少，现场越容易确认实际生效的是哪一份配置。
func findDeviceConfigFile() string {
	if fileExists(systemDeviceConfigPath) {
		return filepath.Clean(systemDeviceConfigPath)
	}

	if exeDir := deviceExecutableDir(); exeDir != "" {
		candidate := filepath.Join(exeDir, deviceConfigSubdir, defaultDeviceConfigFileName)
		if fileExists(candidate) {
			return filepath.Clean(candidate)
		}
	}
	return ""
}

func validateDeviceSettings(settings DeviceSettings) error {
	if settings.Version != 1 {
		return fmt.Errorf("device config version must be 1, got %d", settings.Version)
	}
	if len(settings.Protocols) == 0 && len(settings.LocalIO.DI) == 0 && len(settings.LocalIO.DO) == 0 {
		return fmt.Errorf("device config has no protocols or local_io channels")
	}

	for protocol, endpoints := range settings.Protocols {
		if err := validateProtocolKey(protocol); err != nil {
			return err
		}
		if len(endpoints) == 0 {
			return fmt.Errorf("protocol %q has no endpoints", protocol)
		}
		seenLabels := map[string]struct{}{}
		for i, endpoint := range endpoints {
			label := strings.TrimSpace(endpoint.Label)
			value := strings.TrimSpace(endpoint.Value)
			if label == "" || value == "" {
				return fmt.Errorf("protocol %q endpoint %d must set label and value", protocol, i)
			}
			normalizedLabel := normalizeDeviceLabel(label)
			if _, ok := seenLabels[normalizedLabel]; ok {
				return fmt.Errorf("protocol %q has duplicate endpoint label %q", protocol, label)
			}
			seenLabels[normalizedLabel] = struct{}{}
		}
	}

	if err := validateLocalIOChannels("di", settings.LocalIO.DI); err != nil {
		return err
	}
	if err := validateLocalIOChannels("do", settings.LocalIO.DO); err != nil {
		return err
	}
	seenValues := map[string]struct{}{}
	for _, channel := range append(append([]LocalIOChannelSettings{}, settings.LocalIO.DI...), settings.LocalIO.DO...) {
		value := filepath.Clean(strings.TrimSpace(channel.Value))
		if _, ok := seenValues[value]; ok {
			return fmt.Errorf("local_io has duplicate value path %q", channel.Value)
		}
		seenValues[value] = struct{}{}
	}
	return nil
}

func validateProtocolKey(protocol string) error {
	protocol = strings.TrimSpace(protocol)
	if protocol == "" {
		return fmt.Errorf("protocol key must not be empty")
	}
	if strings.ToLower(protocol) != protocol {
		return fmt.Errorf("protocol key %q must be lowercase", protocol)
	}
	return nil
}

func validateLocalIOChannels(direction string, channels []LocalIOChannelSettings) error {
	seenLabels := map[string]struct{}{}
	for i, channel := range channels {
		label := strings.TrimSpace(channel.Label)
		value := strings.TrimSpace(channel.Value)
		if label == "" || value == "" {
			return fmt.Errorf("local_io.%s channel %d must set label and value", direction, i)
		}
		if !filepath.IsAbs(value) {
			return fmt.Errorf("local_io.%s channel %q value must be an absolute path", direction, label)
		}
		normalizedLabel := normalizeDeviceLabel(label)
		if _, ok := seenLabels[normalizedLabel]; ok {
			return fmt.Errorf("local_io.%s has duplicate label %q", direction, label)
		}
		seenLabels[normalizedLabel] = struct{}{}
		if direction == "di" && channel.Default != nil {
			return fmt.Errorf("local_io.%s channel %q must not set default", direction, label)
		}
	}
	return nil
}

func sortedProtocolKeys(protocols map[string][]ProtocolEndpointSettings) []string {
	keys := make([]string, 0, len(protocols))
	for key := range protocols {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func isSerialProtocol(protocol string) bool {
	switch strings.ToLower(strings.TrimSpace(protocol)) {
	case "rs485_rtu", "rs485-rtu", "rs232_rtu", "rs232-rtu":
		return true
	default:
		return false
	}
}

func normalizeDeviceLabel(label string) string {
	return strings.ToLower(strings.TrimSpace(label))
}
