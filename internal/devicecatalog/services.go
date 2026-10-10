package devicecatalog

import (
	"sync"

	"data-collection-system/internal/bootstrap"
)

var (
	serviceOnce sync.Once
	service     *DeviceCatalogService
)

// Get 返回进程级设备目录单例（启动期 device.yaml 快照）。
func Get() *DeviceCatalogService {
	serviceOnce.Do(func() {
		service = NewDeviceCatalogService(bootstrap.LoadSettings().Device)
	})
	return service
}
