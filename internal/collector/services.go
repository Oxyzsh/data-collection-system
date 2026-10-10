package collector

import (
	"sync"

	runtimeadapter "data-collection-system/internal/collector/adapters/runtime"
	collectorapp "data-collection-system/internal/collector/application"
)

var (
	serviceOnce sync.Once
	service     *collectorapp.Service
)

// GetService 暴露 collector 模块的统一入口。
// application.Service 本身只依赖 RuntimeManager / RuntimeMaintenance 接口，
// 这里再注入 runtime adapter，形成“上层依赖接口、下层提供实现”的装配关系。
//
// 这也是 Go 项目里很常见的写法：
// - application 包定义抽象；
// - adapter/infrastructure 包实现抽象；
// - 模块根目录负责组装对象图。
func GetService() *collectorapp.Service {
	serviceOnce.Do(func() {
		service = collectorapp.NewService(runtimeadapter.ManagerAdapter{}, runtimeadapter.MaintenanceAdapter{})
	})
	return service
}

// RebuildEnabledDevices 按当前配置重建全部已启用设备的采集线程。
// 供配置导入/还原点回滚等「运行中整体替换 config.db」的场景调用
// （冷启动的恢复采集由 main 调用 GetService().RestoreEnabledDevices()）。
func RebuildEnabledDevices() {
	GetService().RebuildEnabledDevices()
}
