package collector

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

	"data-collection-system/internal/bootstrap"
	templatebiz "data-collection-system/internal/template/domain"
	"data-collection-system/internal/valueobj"
)

// gpioValueBackend 抽象 value 设备路径的读写，便于测试时替换为 fake backend。
type gpioValueBackend interface {
	Read(path string) (bool, error)
	Write(path string, value bool) error
}

type sysfsGPIOBackend struct{}

func (sysfsGPIOBackend) Read(path string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	value := strings.TrimSpace(string(data))
	switch value {
	case "0":
		return false, nil
	case "1":
		return true, nil
	default:
		return false, fmt.Errorf("GPIO value %q 无效", value)
	}
}

func (sysfsGPIOBackend) Write(path string, value bool) error {
	raw := "0"
	if value {
		raw = "1"
	}
	return os.WriteFile(path, []byte(raw), 0)
}

// GPIOCollector 直接访问 device.yaml 配置的 value 路径。
// 同一实例的写操作串行化，避免多个写请求交叉修改同一组 GPIO。
type GPIOCollector struct {
	mu      sync.Mutex
	backend gpioValueBackend
	lookup  func(label string) (bootstrap.LocalIOChannelSettings, bool)
}

func init() {
	RegisterCollector("gpio", func(params map[string]string) Collector {
		return NewGPIOCollector(sysfsGPIOBackend{})
	})
}

func NewGPIOCollector(backend gpioValueBackend) *GPIOCollector {
	return newGPIOCollector(backend, func(label string) (bootstrap.LocalIOChannelSettings, bool) {
		return bootstrap.LoadSettings().Device.LocalIOChannel(label)
	})
}

func newGPIOCollector(backend gpioValueBackend, lookup func(label string) (bootstrap.LocalIOChannelSettings, bool)) *GPIOCollector {
	if backend == nil {
		backend = sysfsGPIOBackend{}
	}
	return &GPIOCollector{backend: backend, lookup: lookup}
}

func (c *GPIOCollector) Collect(ctx context.Context, deviceID int, points []templatebiz.DevicePoint, params map[string]string) (map[int]valueobj.CollectedValue, error) {
	result := make(map[int]valueobj.CollectedValue)
	for _, point := range points {
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		default:
		}

		if point.LocalIO == nil {
			continue
		}
		channelLabel := resolveDIDOChannel(point.LocalIO.Channel, params)
		channel, ok := c.lookup(channelLabel)
		if !ok {
			return result, fmt.Errorf("设备 %d 的通道 %s 未在 device.yaml 中配置", deviceID, channelLabel)
		}
		raw, err := c.backend.Read(channel.Value)
		if err != nil {
			return result, fmt.Errorf("读取通道 %s (%s) 失败: %w", channel.Label, channel.Value, err)
		}
		result[point.PointID] = valueobj.NewBoolValue(applyActiveLow(raw, channel.ActiveLow))
	}
	return result, nil
}

func (c *GPIOCollector) Write(ctx context.Context, deviceID int, point templatebiz.DevicePoint, rawValue interface{}) error {
	if point.LocalIO == nil {
		return fmt.Errorf("测点 %d 缺少 local_io 通道配置", point.PointID)
	}
	logical, ok := rawValue.(bool)
	if !ok {
		return fmt.Errorf("GPIO 采集器仅支持 BOOL 写入，got %T", rawValue)
	}
	channelLabel := strings.ToUpper(strings.TrimSpace(point.LocalIO.Channel))
	channel, ok := c.lookup(channelLabel)
	if !ok {
		return fmt.Errorf("设备 %d 的通道 %s 未在 device.yaml 中配置", deviceID, channelLabel)
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.backend.Write(channel.Value, applyActiveLow(logical, channel.ActiveLow)); err != nil {
		return fmt.Errorf("写入通道 %s (%s) 失败: %w", channel.Label, channel.Value, err)
	}
	return nil
}

func (c *GPIOCollector) Stop() {}

func resolveDIDOChannel(templateChannel string, params map[string]string) string {
	channel := strings.TrimSpace(templateChannel)
	if strings.EqualFold(channel, "AUTO") || channel == "" {
		return strings.ToUpper(strings.TrimSpace(params["io_channel"]))
	}
	return channel
}

func applyActiveLow(value bool, activeLow bool) bool {
	if activeLow {
		return !value
	}
	return value
}
