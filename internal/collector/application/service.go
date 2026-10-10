package application

import (
	"time"

	"data-collection-system/internal/valueobj"
)

type ReloadSummary struct {
	Total   int
	Success int
	Failed  []int
}

// RuntimeManager 抽象“单设备运行控制”。
// collector/application 不关心底层到底是 map、线程池还是别的运行时实现，
// 只关心 start/stop/restart 这些能力是否存在。
type RuntimeManager interface {
	Start(deviceID, templateID int) error
	StopWithTimeout(deviceID int, timeout time.Duration) error
	Stop(deviceID int)
	Restart(deviceID, templateID int) error
	IsRunning(deviceID int) bool
	UpdatePointEnabled(deviceID, pointID int, enabled bool)
	GetPointValue(deviceID, pointID int) (valueobj.CollectedValue, time.Time, bool)
	GetPointValues(deviceID int, pointIDs []int) (map[int]RuntimePointValue, bool)
	WritePoint(deviceID, pointID int, finalValue valueobj.CollectedValue) error
}

type RuntimePointValue struct {
	Value     valueobj.CollectedValue
	Timestamp time.Time
	Found     bool
}

// RuntimeMaintenance 抽象“批量维护能力”，例如启动恢复、模板重建。
// 这样 Service 仍然保持薄而稳定，复杂运行逻辑下沉到 runtime adapter。
type RuntimeMaintenance interface {
	RestoreEnabledDevices()
	RebuildEnabledDevices()
	RebuildForTemplate(templateID int) ReloadSummary
}

type Service struct {
	manager     RuntimeManager
	maintenance RuntimeMaintenance
}

// Service 是 collector 模块暴露给外部模块的门面。
// 它本身很薄，主要价值是稳定边界，而不是承载复杂逻辑。
// 这种“薄 service”在重构后很常见：复杂性下沉，跨模块入口保持简单。
func NewService(manager RuntimeManager, maintenance RuntimeMaintenance) *Service {
	return &Service{manager: manager, maintenance: maintenance}
}

func (s *Service) Start(deviceID, templateID int) error {
	return s.manager.Start(deviceID, templateID)
}

func (s *Service) StopWithTimeout(deviceID int, timeout time.Duration) error {
	return s.manager.StopWithTimeout(deviceID, timeout)
}

func (s *Service) Stop(deviceID int) {
	s.manager.Stop(deviceID)
}

func (s *Service) Restart(deviceID, templateID int) error {
	return s.manager.Restart(deviceID, templateID)
}

func (s *Service) IsRunning(deviceID int) bool {
	return s.manager.IsRunning(deviceID)
}

func (s *Service) UpdatePointEnabled(deviceID, pointID int, enabled bool) {
	s.manager.UpdatePointEnabled(deviceID, pointID, enabled)
}

func (s *Service) GetPointValue(deviceID, pointID int) (valueobj.CollectedValue, time.Time, bool) {
	return s.manager.GetPointValue(deviceID, pointID)
}

func (s *Service) GetPointValues(deviceID int, pointIDs []int) (map[int]RuntimePointValue, bool) {
	return s.manager.GetPointValues(deviceID, pointIDs)
}

func (s *Service) WritePoint(deviceID, pointID int, finalValue valueobj.CollectedValue) error {
	return s.manager.WritePoint(deviceID, pointID, finalValue)
}

func (s *Service) RestoreEnabledDevices() {
	s.maintenance.RestoreEnabledDevices()
}

// RebuildEnabledDevices 在配置被整体替换后（配置导入/还原点回滚）重建全部已启用设备的采集线程。
func (s *Service) RebuildEnabledDevices() {
	s.maintenance.RebuildEnabledDevices()
}

func (s *Service) RebuildForTemplate(templateID int) ReloadSummary {
	return s.maintenance.RebuildForTemplate(templateID)
}
