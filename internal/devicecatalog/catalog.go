// Package devicecatalog 提供 device.yaml 生成的设备接入目录：
// 查询可选串口/本地 IO 端点（List），以及把前端提交的 device_ref 解析成固定参数（Resolve）。
// 负责把 device.yaml 中的端点定义转成可选设备列表，并解析前端提交的 device_ref。
package devicecatalog

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"data-collection-system/internal/bootstrap"
)

const (
	DeviceCatalogKindSerial  = "serial"
	DeviceCatalogKindGPIO    = "gpio"
	DeviceCatalogRefVersion  = "1"
	DeviceCatalogLocalIOKey  = "local_io:onboard"
	DeviceCatalogLocalIOKind = "local_io"
)

// DeviceCatalogService 把启动期 device.yaml 快照转换成前端目录和运行时解析入口。
type DeviceCatalogService struct {
	byRef map[string]DeviceCatalogEntry
}

// DeviceCatalogEntry 是后端内部使用的设备目录项。
type DeviceCatalogEntry struct {
	Ref      string                 `json:"ref"`
	Label    string                 `json:"label"`
	Kind     string                 `json:"kind"`
	Selector string                 `json:"selector"`
	Summary  string                 `json:"summary"`
	Params   map[string]string      `json:"params,omitempty"`
	Defaults map[string]string      `json:"defaults,omitempty"`
	Channels []DeviceCatalogChannel `json:"channels,omitempty"`
}

// DeviceCatalogChannel 描述本地 IO 通道，不向前端暴露真实 value 路径。
type DeviceCatalogChannel struct {
	Ref       string `json:"ref"`
	Label     string `json:"label"`
	Direction string `json:"direction"`
	ActiveLow bool   `json:"active_low"`
}

func NewDeviceCatalogService(settings bootstrap.DeviceSettings) *DeviceCatalogService {
	svc := &DeviceCatalogService{byRef: map[string]DeviceCatalogEntry{}}
	svc.load(settings)
	return svc
}

func (s *DeviceCatalogService) load(settings bootstrap.DeviceSettings) {
	protocols := make([]string, 0, len(settings.Protocols))
	for protocol := range settings.Protocols {
		protocols = append(protocols, protocol)
	}
	sort.Strings(protocols)

	for _, protocol := range protocols {
		for _, endpoint := range settings.Protocols[protocol] {
			selector := normalizeCatalogSelector(protocol)
			legacyProtocolType, ok := catalogSelectorToProtocolType(selector)
			if !ok {
				continue
			}
			params := make(map[string]string, len(endpoint.Defaults)+3)
			for key, value := range endpoint.Defaults {
				params[key] = value
			}
			params["serial"] = endpoint.Value
			params["protocol_type"] = legacyProtocolType
			params["device_ref"] = serialDeviceRef(selector, endpoint.Value)
			params["ref_version"] = DeviceCatalogRefVersion

			defaults := make(map[string]string, len(endpoint.Defaults))
			for key, value := range endpoint.Defaults {
				defaults[key] = value
			}
			entry := DeviceCatalogEntry{
				Ref:      params["device_ref"],
				Label:    endpoint.Label,
				Kind:     DeviceCatalogKindSerial,
				Selector: selector,
				Summary:  endpoint.Value,
				Params:   params,
				Defaults: defaults,
			}
			s.byRef[entry.Ref] = entry
		}
	}

	if len(settings.LocalIO.DI) > 0 || len(settings.LocalIO.DO) > 0 {
		entry := DeviceCatalogEntry{
			Ref:      DeviceCatalogLocalIOKey,
			Label:    "本地开关量控制器",
			Kind:     DeviceCatalogKindGPIO,
			Selector: "local_io",
			Summary:  fmt.Sprintf("%dDI / %dDO", len(settings.LocalIO.DI), len(settings.LocalIO.DO)),
			Params: map[string]string{
				"device_ref":     DeviceCatalogLocalIOKey,
				"ref_version":    DeviceCatalogRefVersion,
				"collect_method": "gpio",
				"io_summary":     fmt.Sprintf("%dDI/%dDO", len(settings.LocalIO.DI), len(settings.LocalIO.DO)),
			},
		}
		for _, channel := range settings.LocalIO.DI {
			entry.Channels = append(entry.Channels, DeviceCatalogChannel{
				Ref:       normalizeCatalogSelector(channel.Label),
				Label:     channel.Label,
				Direction: "input",
				ActiveLow: channel.ActiveLow,
			})
		}
		for _, channel := range settings.LocalIO.DO {
			entry.Channels = append(entry.Channels, DeviceCatalogChannel{
				Ref:       normalizeCatalogSelector(channel.Label),
				Label:     channel.Label,
				Direction: "output",
				ActiveLow: channel.ActiveLow,
			})
		}
		s.byRef[entry.Ref] = entry
	}
}

// List 返回指定 selector 下可选的设备目录项。
// List 返回指定 selector 下可选的设备目录项，按标签自然序排列。
func (s *DeviceCatalogService) List(selector string) []DeviceCatalogEntry {
	selector = normalizeCatalogSelector(selector)
	out := make([]DeviceCatalogEntry, 0, len(s.byRef))
	for _, entry := range s.byRef {
		if selector == "" || entry.Selector == selector {
			out = append(out, entry)
		}
	}
	// 按标签自然序输出（COM2 在 COM10 之前）：ref 是内容 hash，按它排序会让前端
	// 「设备添加」的下拉顺序随机化、默认选中的首项漂移到任意端口。label 相同时
	// 退到 ref 兜底，保证输出稳定。
	sort.Slice(out, func(i, j int) bool {
		if out[i].Label != out[j].Label {
			return lessNaturalPortLabel(out[i].Label, out[j].Label)
		}
		return out[i].Ref < out[j].Ref
	})
	return out
}

// lessNaturalPortLabel 按自然序比较端口标签：数字段按数值比较
// （COM2 < COM10，而字典序会得到 COM10 < COM2），其余段按字节序。
func lessNaturalPortLabel(a, b string) bool {
	ia, ib := 0, 0
	for ia < len(a) && ib < len(b) {
		ca, cb := a[ia], b[ib]
		da, db := isASCIIDigit(ca), isASCIIDigit(cb)
		switch {
		case da && db:
			na, nextA := readDigits(a, ia)
			nb, nextB := readDigits(b, ib)
			if na != nb {
				return na < nb
			}
			ia, ib = nextA, nextB
		case ca != cb:
			return ca < cb
		default:
			ia++
			ib++
		}
	}
	return len(a)-ia < len(b)-ib // 前缀更短者在前
}

func isASCIIDigit(c byte) bool { return c >= '0' && c <= '9' }

// readDigits 读取 s[i:] 起始的连续数字段，返回其数值与结束下标。
func readDigits(s string, i int) (int, int) {
	n := 0
	for i < len(s) && isASCIIDigit(s[i]) {
		n = n*10 + int(s[i]-'0')
		i++
	}
	return n, i
}

// Resolve 返回 ref 对应的固定端点参数。
func (s *DeviceCatalogService) Resolve(ref string) (DeviceCatalogEntry, error) {
	entry, ok := s.byRef[strings.TrimSpace(ref)]
	if !ok {
		return DeviceCatalogEntry{}, fmt.Errorf("device ref %q not found", ref)
	}
	return entry, nil
}

// NormalizeProtocolTypeForCatalog 把历史协议编码和新的 selector 统一到目录 selector。
func NormalizeProtocolTypeForCatalog(value string) string {
	return normalizeCatalogSelector(value)
}

func normalizeCatalogSelector(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "rs485_rtu", "rs485-rtu":
		return "rs485_rtu"
	case "rs232_rtu", "rs232-rtu":
		return "rs232_rtu"
	case "local_io", "local-io":
		return "local_io"
	default:
		return strings.ToLower(strings.TrimSpace(value))
	}
}

func catalogSelectorToProtocolType(selector string) (string, bool) {
	switch normalizeCatalogSelector(selector) {
	case "rs485_rtu":
		return "RS485-RTU", true
	case "rs232_rtu":
		return "RS232-RTU", true
	default:
		return "", false
	}
}

func serialDeviceRef(selector, value string) string {
	sum := sha256.Sum256([]byte(DeviceCatalogRefVersion + "|serial|" + normalizeCatalogSelector(selector) + "|" + strings.TrimSpace(value)))
	return "serial:" + hex.EncodeToString(sum[:12])
}
