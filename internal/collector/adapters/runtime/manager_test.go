package runtime

import (
	"reflect"
	"testing"
)

// TestPlanRebuild 验证「配置态 → 运行态」重建计划的集合运算。
// 背景：配置导入/还原点回滚会在运行中整体替换 config.db 数据，
// 需要停掉已不在「已启用」集合里的残留线程，并重启其余全部已启用设备
// （重启而非仅补启动：线程持有旧模板与旧点位定义，必须按新配置重建）。
func TestPlanRebuild(t *testing.T) {
	tests := []struct {
		name         string
		running      []int
		enabled      []int // 已启用设备 ID（配置态）
		wantStop     []int
		wantRestart  []int
	}{
		{
			// 现场场景（恢复出厂后导入）：运行中只有内置设备 0/1，导入后配置含 0~5。
			// 0/1 也必须重建（导入可能换了模板），2~5 补启动。
			name:        "导入带来新设备且内置设备需按新模板重建",
			running:     []int{0, 1},
			enabled:     []int{0, 1, 2, 3, 4, 5},
			wantStop:    nil,
			wantRestart: []int{0, 1, 2, 3, 4, 5},
		},
		{
			// 导入删除了设备：残留线程必须停掉。
			name:        "导入后设备被删除需停残留线程",
			running:     []int{0, 1, 2, 3, 4, 5},
			enabled:     []int{0, 1},
			wantStop:    []int{2, 3, 4, 5},
			wantRestart: []int{0, 1},
		},
		{
			// 全部设备仅模板变化：全部重启。
			name:        "同设备集但模板变化需全量重启",
			running:     []int{0, 1},
			enabled:     []int{0, 1},
			wantStop:    nil,
			wantRestart: []int{0, 1},
		},
		{
			// 空配置导入（如恢复出厂的空包）：停掉全部残留线程。
			name:        "空配置需停全部",
			running:     []int{0, 1, 2},
			enabled:     nil,
			wantStop:    []int{0, 1, 2},
			wantRestart: nil,
		},
		{
			// 空运行态导入（进程刚起、设备从未启动过）：只启动。
			name:        "空运行态只启动",
			running:     nil,
			enabled:     []int{2, 5},
			wantStop:    nil,
			wantRestart: []int{2, 5},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stop, restart := planRebuild(tt.running, tt.enabled)
			if !reflect.DeepEqual(stop, tt.wantStop) {
				t.Errorf("toStop = %v, want %v", stop, tt.wantStop)
			}
			if !reflect.DeepEqual(restart, tt.wantRestart) {
				t.Errorf("toRestart = %v, want %v", restart, tt.wantRestart)
			}
		})
	}
}
