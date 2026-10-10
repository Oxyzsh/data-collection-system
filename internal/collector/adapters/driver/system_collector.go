package collector

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"data-collection-system/internal/shared/logger"
	templatebiz "data-collection-system/internal/template/domain"
	"data-collection-system/internal/valueobj"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
)

type SystemCollector struct{}

func init() {
	RegisterCollector("system", func(params map[string]string) Collector {
		return &SystemCollector{}
	})
}

type SystemInfo struct {
	CPUCores    []float64 `json:"cpuCores"`
	CPUAverage  float64   `json:"cpuAverage"`
	MemPercent  float64   `json:"memPercent"`
	DiskPercent float64   `json:"diskPercent"`
	Timestamp   string    `json:"timestamp"`
	Error       string    `json:"error,omitempty"`
}

const cpuSampleInterval = 500 * time.Millisecond

func GetSystemInfo() (info SystemInfo) {
	coreCount := GetCPUCoreCount()
	defer func() {
		if r := recover(); r != nil {
			info.Error = fmt.Sprintf("panic recovered: %v", r)
			info.CPUCores = make([]float64, coreCount)
			info.Timestamp = time.Now().Format("15:04:05")
		}
	}()

	info.Timestamp = time.Now().Format("15:04:05")

	cpuCores, cpuErr := cpu.Percent(cpuSampleInterval, true)
	if cpuErr != nil {
		info.Error = fmt.Sprintf("cpu error: %v", cpuErr)
		info.CPUCores = make([]float64, coreCount)
	} else if len(cpuCores) > 0 {
		info.CPUCores = cpuCores
		total := 0.0
		for _, c := range cpuCores {
			total += c
		}
		info.CPUAverage = total / float64(len(cpuCores))
	} else {
		info.CPUCores = make([]float64, coreCount)
	}

	virtualMem, memErr := mem.VirtualMemory()
	if memErr != nil {
		if info.Error != "" {
			info.Error += "; "
		}
		info.Error += fmt.Sprintf("mem error: %v", memErr)
	} else {
		info.MemPercent = virtualMem.UsedPercent
	}

	diskUsage, diskErr := disk.Usage("/")
	if diskErr != nil {
		if info.Error != "" {
			info.Error += "; "
		}
		info.Error += fmt.Sprintf("disk error: %v", diskErr)
	} else {
		info.DiskPercent = diskUsage.UsedPercent
	}

	return info
}

func GetSystemInfoSafe() SystemInfo {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "GetSystemInfoSafe panic: %v\n", r)
			for i := 0; i < 10; i++ {
				pc, file, line, ok := runtime.Caller(i)
				if !ok {
					break
				}
				fmt.Fprintf(os.Stderr, "  %s:%d (0x%x)\n", file, line, pc)
			}
		}
	}()

	return GetSystemInfo()
}

func GetCPUCoreCount() int {
	count, err := cpu.Counts(true)
	if err == nil && count > 0 {
		return count
	}
	if fallback := runtime.NumCPU(); fallback > 0 {
		return fallback
	}
	return 1
}

func (c *SystemCollector) Collect(ctx context.Context, deviceID int, points []templatebiz.DevicePoint, params map[string]string) (map[int]valueobj.CollectedValue, error) {
	result := make(map[int]valueobj.CollectedValue)

	logger.Debugf("[SystemCollector] 设备ID: %d, 测点数: %d\n", deviceID, len(points))

	systemInfo := GetSystemInfoSafe()

	if systemInfo.Error != "" {
		logger.Warnf("[SystemCollector] 获取系统信息失败: %s\n", systemInfo.Error)
	}

	for _, point := range points {
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		default:
		}
		if point.PointID <= 0 {
			continue
		}
		result[point.PointID] = valueobj.NewNumberValue(resolveSystemPointValue(point, systemInfo))
	}

	return result, nil
}

// Write 默认实现：系统采集器不支持写操作。
func (c *SystemCollector) Write(ctx context.Context, deviceID int, point templatebiz.DevicePoint, rawValue interface{}) error {
	return fmt.Errorf("系统采集器不支持写操作")
}

func (c *SystemCollector) Stop() {
	logger.Infof("[SystemCollector] 停止采集")
}

func resolveSystemPointValue(point templatebiz.DevicePoint, systemInfo SystemInfo) float64 {
	if coreIdx, ok := systemCPUCoreIndex(point); ok {
		if coreIdx >= 0 && coreIdx < len(systemInfo.CPUCores) {
			return systemInfo.CPUCores[coreIdx]
		}
		return 0
	}

	switch normalizedSystemPointName(point) {
	case "cpu_total_usage":
		return systemInfo.CPUAverage
	case "memory_usage":
		return systemInfo.MemPercent
	case "disk_usage":
		return systemInfo.DiskPercent
	default:
		return 0
	}
}

func systemCPUCoreIndex(point templatebiz.DevicePoint) (int, bool) {
	candidates := []string{strings.TrimSpace(point.Name), strings.TrimSpace(point.NameEN)}
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		switch {
		case strings.HasPrefix(candidate, "CPU核心"):
			indexStr := strings.TrimSpace(strings.TrimPrefix(candidate, "CPU核心"))
			if idx, err := strconv.Atoi(indexStr); err == nil && idx > 0 {
				return idx - 1, true
			}
		case strings.HasPrefix(candidate, "CPU Core "):
			indexStr := strings.TrimSpace(strings.TrimPrefix(candidate, "CPU Core "))
			if idx, err := strconv.Atoi(indexStr); err == nil && idx > 0 {
				return idx - 1, true
			}
		}
	}
	return 0, false
}

func normalizedSystemPointName(point templatebiz.DevicePoint) string {
	switch {
	case point.Name == "CPU总使用率" || point.NameEN == "CPU Total Usage":
		return "cpu_total_usage"
	case point.Name == "内存使用率" || point.NameEN == "Memory Usage":
		return "memory_usage"
	case point.Name == "磁盘使用率" || point.NameEN == "Disk Usage":
		return "disk_usage"
	default:
		return ""
	}
}
