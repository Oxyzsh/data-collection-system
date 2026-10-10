package device

import (
	"strings"
	"sync"
	"time"

	collectormodule "data-collection-system/internal/collector"
	"data-collection-system/internal/database"
	deviceapp "data-collection-system/internal/device/application"
	devicecatalog "data-collection-system/internal/devicecatalog"
	deviceinfra "data-collection-system/internal/device/infrastructure"
)

// deviceCatalogResolver 把 devicecatalog 的目录能力适配成 device application 本地接口。
// application 不感知 device.yaml 目录接口的具体数据结构。
type deviceCatalogResolver struct{}

func (deviceCatalogResolver) ResolveDeviceRef(ref string) (deviceapp.ResolvedDeviceEntry, error) {
	entry, err := devicecatalog.Get().Resolve(ref)
	if err != nil {
		return deviceapp.ResolvedDeviceEntry{}, err
	}
	channels := make(map[string]string, len(entry.Channels))
	for _, channel := range entry.Channels {
		channels[strings.ToLower(strings.TrimSpace(channel.Label))] = channel.Direction
	}
	return deviceapp.ResolvedDeviceEntry{
		Ref:      entry.Ref,
		Selector: entry.Selector,
		Kind:     entry.Kind,
		Params:   entry.Params,
		Defaults: entry.Defaults,
		Channels: channels,
	}, nil
}

type collectorReloader struct{}

// collectorReloader / collectorController 都是很薄的一层适配器。
// 它们把 device 模块需要的能力声明成小接口，再转调到 collector/alarm 模块。
// 这样 application 层只依赖“能力”，不依赖具体实现，便于测试，也能降低模块耦合。
//
// 对 C 语言背景可以这样理解：
// - 这里的 interface 更像“一组函数签名约定”，而不是带数据成员的基类。
// - 只要某个类型实现了这些方法，Go 就认为它“满足接口”，不需要显式写 implements。
func (collectorReloader) ReloadIfRunning(deviceID int, status string, templateID int) error {
	if status != "running" {
		return nil
	}
	return collectormodule.GetService().Restart(deviceID, templateID)
}

type collectorController struct{}

type alarmCleaner struct{}

func (collectorController) Start(deviceID, templateID int) error {
	return collectormodule.GetService().Start(deviceID, templateID)
}

func (collectorController) StopWithTimeout(deviceID int, timeout time.Duration) error {
	return collectormodule.GetService().StopWithTimeout(deviceID, timeout)
}

func (collectorController) Stop(deviceID int) {
	collectormodule.GetService().Stop(deviceID)
}

func (collectorController) IsRunning(deviceID int) bool {
	return collectormodule.GetService().IsRunning(deviceID)
}

// RecordOperation 保留原操作日志入口签名；裁剪后的采集进程不落操作日志。
func RecordOperation(userID int64, username, ip, messageKey string, params map[string]any) {}

var (
	configServiceOnce    sync.Once
	configService        *deviceapp.ConfigService
	lifecycleServiceOnce sync.Once
	lifecycleService     *deviceapp.LifecycleService
	queryServiceOnce     sync.Once
	queryService         *deviceapp.QueryService
	selectorServiceOnce  sync.Once
	selectorService      *deviceapp.SelectorService
	selectorRTOnce       sync.Once
	selectorRTService    *deviceapp.SelectorRealtimeService
	pointCfgServiceOnce  sync.Once
	pointCfgService      *deviceapp.PointConfigService
)

// GetXxxService 这一组函数是模块级“单例入口”。
// sync.Once.Do 会保证传入的初始化函数只执行一次，即使多个 goroutine 同时调用也安全。
//
// 对比 C 里常见的“静态全局变量 + 手写判空”，这里的好处是：
// 1. 并发安全已经由标准库保证；
// 2. 初始化逻辑集中在一个闭包里，读起来更接近“懒加载工厂”。
func GetConfigService() *deviceapp.ConfigService {
	configServiceOnce.Do(func() {
		configService = deviceapp.NewConfigService(
			deviceinfra.NewSQLConfigRepository(database.ConfigDB),
			collectorReloader{},
			deviceCatalogResolver{},
		)
	})
	return configService
}

func GetLifecycleService() *deviceapp.LifecycleService {
	lifecycleServiceOnce.Do(func() {
		lifecycleService = deviceapp.NewLifecycleService(
			deviceinfra.NewSQLLifecycleRepository(database.ConfigDB),
			collectorController{},
			10*time.Second,
		)
	})
	return lifecycleService
}

func GetQueryService() *deviceapp.QueryService {
	queryServiceOnce.Do(func() {
		queryService = deviceapp.NewQueryService(
			deviceinfra.NewSQLQueryRepository(database.ConfigDB, database.HistoryDB),
		)
	})
	return queryService
}

func GetSelectorService() *deviceapp.SelectorService {
	selectorServiceOnce.Do(func() {
		selectorService = deviceapp.NewSelectorService(
			deviceinfra.NewSelectorProvider(database.ConfigDB, database.RuntimeDB),
		)
	})
	return selectorService
}

func GetSelectorRealtimeService() *deviceapp.SelectorRealtimeService {
	selectorRTOnce.Do(func() {
		selectorRTService = deviceapp.NewSelectorRealtimeService(GetSelectorService())
	})
	return selectorRTService
}

func GetPointConfigService() *deviceapp.PointConfigService {
	pointCfgServiceOnce.Do(func() {
		pointCfgService = deviceapp.NewPointConfigService(
			deviceinfra.NewDefaultSQLPointConfigRepository(),
		)
	})
	return pointCfgService
}

func UpdatePointEnabled(deviceID, pointID int, enabled bool) {
	collectormodule.GetService().UpdatePointEnabled(deviceID, pointID, enabled)
}
