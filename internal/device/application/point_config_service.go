package application

import devicedomain "data-collection-system/internal/device/domain"

type PointConfigRepository interface {
	GetDevicePointConfigs(deviceID int) (map[int]*devicedomain.DevicePointConfig, error)
	SaveDevicePointConfig(cfg *devicedomain.DevicePointConfig, nowStr string) error
	ReconcileDevicePointConfigs(deviceID int, validPointIDs map[int]struct{}) error
	DeleteDevicePointConfig(deviceID, pointID int) error
}

type PointConfigService struct {
	repo PointConfigRepository
}

func NewPointConfigService(repo PointConfigRepository) *PointConfigService {
	return &PointConfigService{repo: repo}
}

func (s *PointConfigService) GetDevicePointConfigs(deviceID int) (map[int]*devicedomain.DevicePointConfig, error) {
	return s.repo.GetDevicePointConfigs(deviceID)
}

func (s *PointConfigService) SaveDevicePointConfig(cfg *devicedomain.DevicePointConfig, nowStr string) error {
	cfg.ControlLevel = devicedomain.NormalizeControlLevel(cfg.ControlLevel)
	return s.repo.SaveDevicePointConfig(cfg, nowStr)
}

func (s *PointConfigService) ReconcileDevicePointConfigs(deviceID int, validPointIDs map[int]struct{}) error {
	return s.repo.ReconcileDevicePointConfigs(deviceID, validPointIDs)
}

func (s *PointConfigService) DeleteDevicePointConfig(deviceID, pointID int) error {
	return s.repo.DeleteDevicePointConfig(deviceID, pointID)
}
