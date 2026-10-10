package collector

import (
	"testing"

	templatebiz "data-collection-system/internal/template/domain"
)

func TestResolveSystemPointValue(t *testing.T) {
	info := SystemInfo{
		CPUCores:    []float64{11.1, 22.2, 33.3, 44.4, 55.5, 66.6},
		CPUAverage:  38.85,
		MemPercent:  77.7,
		DiskPercent: 88.8,
	}

	tests := []struct {
		name  string
		point templatebiz.DevicePoint
		want  float64
	}{
		{
			name:  "中文CPU核心点位",
			point: templatebiz.DevicePoint{PointID: 6, Name: "CPU核心6"},
			want:  66.6,
		},
		{
			name:  "英文CPU核心点位",
			point: templatebiz.DevicePoint{PointID: 2, NameEN: "CPU Core 2"},
			want:  22.2,
		},
		{
			name:  "CPU总使用率",
			point: templatebiz.DevicePoint{PointID: 7, Name: "CPU总使用率"},
			want:  38.85,
		},
		{
			name:  "内存使用率",
			point: templatebiz.DevicePoint{PointID: 8, NameEN: "Memory Usage"},
			want:  77.7,
		},
		{
			name:  "磁盘使用率",
			point: templatebiz.DevicePoint{PointID: 9, Name: "磁盘使用率"},
			want:  88.8,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveSystemPointValue(tt.point, info)
			if got != tt.want {
				t.Fatalf("resolveSystemPointValue() = %v, want %v", got, tt.want)
			}
		})
	}
}
