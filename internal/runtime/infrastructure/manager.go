package infrastructure

import (
	"context"
	"sync"
	"time"

	"data-collection-system/internal/shared/logger"
	"data-collection-system/internal/valueobj"
)

// DataManager 数据管理器（单例）。
// 它属于“纯运行态内存结构”，和配置表不同：
// - 设备启动时注册；
// - 设备停止时注销；
// - HTTP 查询、告警引擎、历史落盘都从这里拿当前运行数据。
//
// 从职责上看，它像一个进程内注册中心：
// - devices 保存每个设备当前的实时值；
// - histories 保存每个设备对应的历史缓冲/落盘器。
type DataManager struct {
	devices map[int]*DeviceRuntime // deviceID → DeviceRuntime
	mu      sync.RWMutex
}

var dataManagerInstance *DataManager
var dataManagerOnce sync.Once

// GetDataManager 获取数据管理器单例。
// sync.Once 保证整个进程里只创建一个 DataManager，避免多个运行态中心并存。
func GetDataManager() *DataManager {
	dataManagerOnce.Do(func() {
		dataManagerInstance = &DataManager{
			devices: make(map[int]*DeviceRuntime),
		}
	})
	return dataManagerInstance
}

// RegisterDevice 注册设备运行时数据。
// 这个动作发生在采集协程启动前，相当于先把“内存工作区”准备好。
//
// pointMetas 不保存实时值，只描述每个点的元信息；
// 真正的采集值会在后续循环里不断写入 PointRuntime。
func (dm *DataManager) RegisterDevice(deviceID, templateID int, deviceName string, pointMetas []PointMeta) *DeviceRuntime {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	dr := NewDeviceRuntime(deviceID, templateID, deviceName, pointMetas)
	dm.devices[deviceID] = dr

	return dr
}

// UnregisterDevice 注销设备运行时数据。
// 停止时先 Flush 历史缓冲，再删除内存引用，避免最后一批数据丢失。
func (dm *DataManager) UnregisterDevice(deviceID int) {
	// 收尾落盘失败只记日志：不回滚设备停止动作，与旧行为一致。
	if err := GetHourlyHistoryWriter().FlushDevice(context.Background(), deviceID); err != nil {
		logger.Warnf("[History] 设备 %d 收尾落盘失败: %v\n", deviceID, err)
	}

	dm.mu.Lock()
	defer dm.mu.Unlock()
	delete(dm.devices, deviceID)
}

// GetDeviceRuntime 获取设备运行时（只读）
func (dm *DataManager) GetDeviceRuntime(deviceID int) (*DeviceRuntime, bool) {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	dr, ok := dm.devices[deviceID]
	return dr, ok
}

// GetPointValue 从内存获取设备测点的实时值
// 返回值：value, lastUpdatedAt, found
func (dm *DataManager) GetPointValue(deviceID, pointID int) (float64, string, bool) {
	dr, ok := dm.GetDeviceRuntime(deviceID)
	if !ok {
		return 0, "", false
	}

	val, t, found := dr.GetNumericPointValue(pointID)
	if !found {
		return 0, "", false
	}

	return val, t.Format("2006-01-02 15:04:05"), true
}

// GetPointValueAny 返回统一的 CollectedValue。
// 之所以不用单纯 float64，是因为项目里的测点可能是数字、字符串、布尔等多种类型。
func (dm *DataManager) GetPointValueAny(deviceID, pointID int) (valueobj.CollectedValue, string, bool) {
	dr, ok := dm.GetDeviceRuntime(deviceID)
	if !ok {
		return valueobj.CollectedValue{}, "", false
	}
	val, t, found := dr.GetPointValue(pointID)
	if !found {
		return valueobj.CollectedValue{}, "", false
	}
	return val, t.Format("2006-01-02 15:04:05"), true
}

// GetPointRuntime 获取指定测点的 PointRuntime 引用
func (dm *DataManager) GetPointRuntime(deviceID, pointID int) (*PointRuntime, bool) {
	dr, ok := dm.GetDeviceRuntime(deviceID)
	if !ok {
		return nil, false
	}
	return dr.GetPointRuntime(pointID)
}

// GetPointHistory 从内存获取设备测点的秒级历史数据。
func (dm *DataManager) GetPointHistory(deviceID, pointID int, count int) []float64 {
	dr, ok := dm.GetDeviceRuntime(deviceID)
	if !ok {
		return nil
	}

	rt, ok := dr.GetPointRuntime(pointID)
	if !ok {
		return nil
	}
	return rt.GetHistory(count)
}

// GetPointRecentSeries 获取指定测点的最近时序数据。
func (dm *DataManager) GetPointRecentSeries(deviceID, pointID int, granularity string, count int) ([]RecentHistoryPoint, bool) {
	dr, ok := dm.GetDeviceRuntime(deviceID)
	if !ok {
		return nil, false
	}

	rt, ok := dr.GetPointRuntime(pointID)
	if !ok {
		return nil, false
	}
	series := rt.GetRecentSeries(granularity, count)
	if len(series) == 0 {
		return nil, false
	}
	return series, true
}

// GetPointRecentSeriesRange 获取指定测点在时间范围内的最近时序数据。
func (dm *DataManager) GetPointRecentSeriesRange(deviceID, pointID int, granularity string, startTime, endTime time.Time) ([]RecentHistoryPoint, bool) {
	dr, ok := dm.GetDeviceRuntime(deviceID)
	if !ok {
		return nil, false
	}

	rt, ok := dr.GetPointRuntime(pointID)
	if !ok {
		return nil, false
	}
	series, ok := rt.GetRecentSeriesRange(granularity, startTime, endTime)
	if !ok {
		return nil, false
	}
	if len(series) == 0 {
		return []RecentHistoryPoint{}, true
	}
	return series, true
}

// FlushAll 兼容保留：当前运行态不再承载历史落盘缓冲，因此这里为空实现。
func (dm *DataManager) FlushAll() {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
}

// IsDeviceRunning 检查设备是否正在运行
func (dm *DataManager) IsDeviceRunning(deviceID int) bool {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	_, ok := dm.devices[deviceID]
	return ok
}
