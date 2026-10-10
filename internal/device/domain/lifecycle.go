package domain

// DeviceLifecycleState 表示设备生命周期相关的最小状态集合。
// 可以把它理解成设备“当前是否该跑、跑到什么状态了”的快照。
type DeviceLifecycleState struct {
	ID         int
	Name       string
	TemplateID int
	// Status 表示实际运行状态。
	Status string
	// Enabled 表示配置层是否允许设备运行。
	Enabled bool
}
