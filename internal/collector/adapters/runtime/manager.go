package runtime

import (
	"sort"
	"time"

	collectorapp "data-collection-system/internal/collector/application"
	collectorinfra "data-collection-system/internal/collector/infrastructure"
	collectorruntime "data-collection-system/internal/collector/runtime"
	"data-collection-system/internal/shared/logger"
	"data-collection-system/internal/valueobj"
)

type ManagerAdapter struct{}

// ManagerAdapter 把 collector/application 需要的 RuntimeManager 接口，
// 转接到真正的 collectorruntime.CollectorManager 单例上。
// 这是典型的 adapter：上层只看接口，下层对接具体运行时实现。
func (ManagerAdapter) Start(deviceID, templateID int) error {
	return collectorruntime.GetCollectorManager().StartCollector(deviceID, templateID)
}

func (ManagerAdapter) StopWithTimeout(deviceID int, timeout time.Duration) error {
	return collectorruntime.GetCollectorManager().StopCollectorWithTimeout(deviceID, timeout)
}

func (ManagerAdapter) Stop(deviceID int) {
	collectorruntime.GetCollectorManager().StopCollector(deviceID)
}

func (ManagerAdapter) Restart(deviceID, templateID int) error {
	return collectorruntime.GetCollectorManager().RestartCollector(deviceID, templateID)
}

func (ManagerAdapter) IsRunning(deviceID int) bool {
	return collectorruntime.GetCollectorManager().IsCollectorRunning(deviceID)
}

func (ManagerAdapter) UpdatePointEnabled(deviceID, pointID int, enabled bool) {
	collectorruntime.GetCollectorManager().UpdatePointEnabled(deviceID, pointID, enabled)
}

func (ManagerAdapter) GetPointValue(deviceID, pointID int) (valueobj.CollectedValue, time.Time, bool) {
	manager := collectorruntime.GetCollectorManager()
	return manager.GetPointValue(deviceID, pointID)
}

func (ManagerAdapter) GetPointValues(deviceID int, pointIDs []int) (map[int]collectorapp.RuntimePointValue, bool) {
	manager := collectorruntime.GetCollectorManager()
	snapshots, running := manager.GetPointValues(deviceID, pointIDs)
	if !running {
		return map[int]collectorapp.RuntimePointValue{}, false
	}
	result := make(map[int]collectorapp.RuntimePointValue, len(snapshots))
	for pointID, snapshot := range snapshots {
		result[pointID] = collectorapp.RuntimePointValue{
			Value:     snapshot.Value,
			Timestamp: snapshot.Timestamp,
			Found:     true,
		}
	}
	return result, true
}

func (ManagerAdapter) WritePoint(deviceID, pointID int, finalValue valueobj.CollectedValue) error {
	return collectorruntime.GetCollectorManager().WritePoint(deviceID, pointID, finalValue)
}

type MaintenanceAdapter struct{}

// RestoreEnabledDevices 在进程启动后恢复所有 enabled=1 的设备采集。
// 可以理解为“根据配置态重建运行态”。
func (MaintenanceAdapter) RestoreEnabledDevices() {
	devices, err := collectorinfra.ListEnabledDeviceCollectors()
	if err != nil {
		logger.Errorf("[CollectorMaintenance] 查询已启用设备失败: %v", err)
		return
	}
	restored := 0
	for _, item := range devices {
		if err := collectorruntime.GetCollectorManager().StartCollector(item.DeviceID, item.TemplateID); err != nil {
			logger.Errorf("[CollectorMaintenance] 设备 %d 启动采集失败: %v", item.DeviceID, err)
			if statusErr := collectorinfra.UpdateDeviceStatus(item.DeviceID, "stopped", time.Now()); statusErr != nil {
				logger.Warnf("[CollectorMaintenance] 设备 %d 启动失败后状态回写失败: %v", item.DeviceID, statusErr)
			}
			continue
		}
		if err := collectorinfra.UpdateDeviceStatus(item.DeviceID, "running", time.Now()); err != nil {
			logger.Warnf("[CollectorMaintenance] 设备 %d 状态更新失败: %v", item.DeviceID, err)
		}
		restored++
	}
	logger.Infof("[CollectorMaintenance] 共恢复 %d/%d 个已启用设备采集", restored, len(devices))
}

// planRebuild 计算「配置态 → 运行态」的重建计划（纯函数，便于测试）。
// toStop：运行中但不在已启用集合里的残留线程（设备被导入包删除/停用）；
// toRestart：全部已启用设备——即使线程在跑也必须重启，因为导入可能替换了模板与点位定义。
// 两个返回值均按设备 ID 升序：输入分别来自 map 遍历与无 ORDER BY 的查询，顺序不保证。
func planRebuild(running []int, enabled []int) (toStop []int, toRestart []int) {
	enabledSet := make(map[int]struct{}, len(enabled))
	for _, id := range enabled {
		enabledSet[id] = struct{}{}
	}
	for _, id := range running {
		if _, ok := enabledSet[id]; !ok {
			toStop = append(toStop, id)
		}
	}
	toRestart = append(toRestart, enabled...)
	sort.Ints(toStop)
	sort.Ints(toRestart)
	return toStop, toRestart
}

// RebuildEnabledDevices 在配置被整体替换后（配置导入 / 还原点回滚）重建采集运行态，
// 让内存中的采集线程与替换后的 config.db 对齐。
// 不重建的话，导入包里的设备不会启动采集，而 devices.status 又是导入包里的值（running），
// 表现为「设备显示启动、测点值不显示」（恢复出厂后导入配置的现场场景）。
// 单个设备失败只记日志并继续，不影响其它设备——与 RestoreEnabledDevices 同一容错口径。
func (MaintenanceAdapter) RebuildEnabledDevices() {
	devices, err := collectorinfra.ListEnabledDeviceCollectors()
	if err != nil {
		logger.Errorf("[CollectorMaintenance] 配置重建：查询已启用设备失败: %v", err)
		return
	}

	enabledIDs := make([]int, 0, len(devices))
	for _, item := range devices {
		enabledIDs = append(enabledIDs, item.DeviceID)
	}

	manager := collectorruntime.GetCollectorManager()
	toStop, _ := planRebuild(manager.RunningDeviceIDs(), enabledIDs)
	stopped := 0
	for _, deviceID := range toStop {
		if err := manager.StopCollectorWithTimeout(deviceID, collectorruntime.StopCollectorTimeout); err != nil {
			logger.Warnf("[CollectorMaintenance] 配置重建：停止残留线程设备 %d 失败: %v", deviceID, err)
			continue
		}
		stopped++
	}

	restored := 0
	for _, item := range devices {
		if err := manager.RestartCollector(item.DeviceID, item.TemplateID); err != nil {
			logger.Errorf("[CollectorMaintenance] 配置重建：设备 %d 重启失败: %v", item.DeviceID, err)
			continue
		}
		if err := collectorinfra.UpdateDeviceStatus(item.DeviceID, "running", time.Now()); err != nil {
			logger.Warnf("[CollectorMaintenance] 配置重建：设备 %d 状态更新失败: %v", item.DeviceID, err)
		}
		restored++
	}
	logger.Infof("[CollectorMaintenance] 配置重建完成：重启 %d/%d 个已启用设备，停止 %d 个残留线程", restored, len(devices), stopped)
}

// RebuildForTemplate 在模板被替换后，重建所有仍在运行且引用该模板的设备。
// 这样模板修改才能真正体现在运行中的采集器上。
func (MaintenanceAdapter) RebuildForTemplate(templateID int) collectorapp.ReloadSummary {
	deviceIDs, err := collectorinfra.ListRunningEnabledDeviceIDsByTemplate(templateID)
	if err != nil {
		logger.Errorf("[CollectorMaintenance] 查询模板 %d 运行中设备失败: %v", templateID, err)
		return collectorapp.ReloadSummary{}
	}
	if len(deviceIDs) == 0 {
		return collectorapp.ReloadSummary{}
	}

	summary := collectorapp.ReloadSummary{
		Total:  len(deviceIDs),
		Failed: make([]int, 0),
	}
	for _, deviceID := range deviceIDs {
		if err := collectorruntime.GetCollectorManager().RestartCollector(deviceID, templateID); err != nil {
			logger.Errorf("[CollectorMaintenance] 设备 %d 重启失败: %v", deviceID, err)
			summary.Failed = append(summary.Failed, deviceID)
			continue
		}
		summary.Success++
	}
	return summary
}
