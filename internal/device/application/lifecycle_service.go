package application

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	devicedomain "data-collection-system/internal/device/domain"
)

var (
	ErrDeviceAlreadyRunning    = errors.New("device already running")
	ErrDeviceNotRunning        = errors.New("device not running")
	ErrCollectorAlreadyRunning = errors.New("collector already running")
)

type DeviceLifecycleRepository interface {
	// repository 只管设备生命周期数据，不处理采集器和告警引擎。
	GetDeviceLifecycleState(deviceID int) (devicedomain.DeviceLifecycleState, error)
	UpdateDeviceStatus(deviceID int, status string, now time.Time) error
	UpdateDeviceEnabled(deviceID int, enabled bool, now time.Time) error
	UpdateDeviceEnabledAndStatus(deviceID int, enabled bool, status string, now time.Time) error
	DeleteDeviceCascade(deviceID int) error
}

type DeviceCollectorController interface {
	// 这里把采集器抽成接口，service 不直接依赖具体采集模块。
	// 对比 C 里的做法，这相当于“上层只拿一组函数指针约定”，
	// 但 Go 会自动检查某个类型是否实现了这些方法，不需要手写 vtable。
	Start(deviceID, templateID int) error
	StopWithTimeout(deviceID int, timeout time.Duration) error
	Stop(deviceID int)
	IsRunning(deviceID int) bool
}

type LifecycleService struct {
	repo      DeviceLifecycleRepository
	collector DeviceCollectorController
	timeout   time.Duration
}

// LifecycleService 负责配置态和运行态之间的“编排”。
// 它自己不直接做 SQL，也不直接管理 goroutine，只负责决定操作顺序。
func NewLifecycleService(repo DeviceLifecycleRepository, collector DeviceCollectorController, timeout time.Duration) *LifecycleService {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &LifecycleService{repo: repo, collector: collector, timeout: timeout}
}

// DeleteDevice 先停采集，再删配置，避免采集协程继续写已删除设备的数据。
// 这里体现的是“先停运行态，再删配置态”的顺序约束。
func (s *LifecycleService) DeleteDevice(deviceID int) error {
	// 设备 ID 0 是合法的：全局指标虚拟设备（metric.MetricDeviceID = 0）就用 0，
	// 这里只排除负数，否则该行所有操作都会被误判为 “invalid device id”。
	if deviceID < 0 {
		return ErrInvalidDeviceID
	}

	state, err := s.repo.GetDeviceLifecycleState(deviceID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrDeviceNotFound
		}
		return err
	}

	if s.collector.IsRunning(deviceID) {
		if err := s.collector.StopWithTimeout(deviceID, s.timeout); err != nil {
			return fmt.Errorf("stop collector timeout: %w", err)
		}
	}

	if err := s.repo.DeleteDeviceCascade(state.ID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrDeviceNotFound
		}
		return err
	}
	return nil
}

// StartDevice 启动设备采集。
// 先拉起采集器，再把状态写回数据库；失败时会做回滚停止。
// 这段流程可以当作一个很小的事务编排：
// - collector.Start 成功，但 DB 更新失败时，需要补偿性 Stop；
// - 这样可以避免“内存里已经在跑，但数据库还显示 stopped”的不一致。
func (s *LifecycleService) StartDevice(deviceID int) error {
	// 设备 ID 0 是合法的：全局指标虚拟设备（metric.MetricDeviceID = 0）就用 0，
	// 这里只排除负数，否则该行所有操作都会被误判为 “invalid device id”。
	if deviceID < 0 {
		return ErrInvalidDeviceID
	}

	state, err := s.repo.GetDeviceLifecycleState(deviceID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrDeviceNotFound
		}
		return err
	}

	if state.Status == "running" {
		return ErrDeviceAlreadyRunning
	}
	if s.collector.IsRunning(state.ID) {
		return ErrCollectorAlreadyRunning
	}

	if err := s.collector.Start(state.ID, state.TemplateID); err != nil {
		return err
	}

	if err := s.repo.UpdateDeviceStatus(state.ID, "running", time.Now()); err != nil {
		s.collector.Stop(state.ID)
		return err
	}

	return nil
}

// StopDevice 停止设备采集，并写回状态。
// StopWithTimeout 会等待后台 goroutine 退出，避免函数返回时采集循环还在继续写运行态。
func (s *LifecycleService) StopDevice(deviceID int) error {
	// 设备 ID 0 是合法的：全局指标虚拟设备（metric.MetricDeviceID = 0）就用 0，
	// 这里只排除负数，否则该行所有操作都会被误判为 “invalid device id”。
	if deviceID < 0 {
		return ErrInvalidDeviceID
	}

	state, err := s.repo.GetDeviceLifecycleState(deviceID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrDeviceNotFound
		}
		return err
	}

	if state.Status != "running" && !s.collector.IsRunning(state.ID) {
		return ErrDeviceNotRunning
	}

	if err := s.collector.StopWithTimeout(state.ID, s.timeout); err != nil {
		return err
	}

	if err := s.repo.UpdateDeviceStatus(state.ID, "stopped", time.Now()); err != nil {
		return err
	}

	return nil
}

// SetDeviceEnabled 只切换启用状态。
// 必要时会联动启动或停止采集器。
// 这里把 enabled 和 status 拆开看：
// - enabled 更像“配置上允不允许跑”；
// - status 更像“当前实际上是否在跑”。
// 两者分开保存，能更准确表达设备生命周期。
func (s *LifecycleService) SetDeviceEnabled(deviceID int, enabled bool) error {
	// 设备 ID 0 是合法的：全局指标虚拟设备（metric.MetricDeviceID = 0）就用 0，
	// 这里只排除负数，否则该行所有操作都会被误判为 “invalid device id”。
	if deviceID < 0 {
		return ErrInvalidDeviceID
	}

	state, err := s.repo.GetDeviceLifecycleState(deviceID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrDeviceNotFound
		}
		return err
	}

	if enabled {
		if state.Status != "running" {
			if err := s.collector.Start(state.ID, state.TemplateID); err != nil {
				return err
			}
			if err := s.repo.UpdateDeviceEnabledAndStatus(state.ID, true, "running", time.Now()); err != nil {
				s.collector.Stop(state.ID)
				return err
			}
			return nil
		}
		return s.repo.UpdateDeviceEnabled(state.ID, true, time.Now())
	}

	if state.Status == "running" {
		if err := s.collector.StopWithTimeout(state.ID, s.timeout); err != nil {
			return err
		}
	}

	return s.repo.UpdateDeviceEnabledAndStatus(state.ID, false, "stopped", time.Now())
}
