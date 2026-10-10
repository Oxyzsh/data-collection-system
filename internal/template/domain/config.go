package domain

// TemplateMeta 是模板在配置和装配阶段需要的最小元信息。
type TemplateMeta struct {
	ID            int
	Name          string
	Type          string
	CollectMethod string
	Description   string
	IsBuiltIn     bool
}

// TemplateReference 表示某个模板当前被哪些设备引用。
type TemplateReference struct {
	DeviceID int
	Name     string
	Status   string
	Enabled  bool
}
