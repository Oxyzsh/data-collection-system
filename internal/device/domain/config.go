package domain

// DeviceConfigSnapshot 表示设备配置更新前的关键状态。
// 它是一个“最小快照”，只保留后续决策真正需要的信息，
// 例如更新完配置后是否要重启采集器。
type DeviceConfigSnapshot struct {
	TemplateID int
	Status     string
}

// DeviceConfigFull 是设备完整配置快照，供复制等需要完整配置的场景使用。
type DeviceConfigFull struct {
	Name        string
	TemplateID  int
	Status      string
	Description string
	Params      map[string]string
}

// TemplateMeta 表示模板元信息。
// 这里只关心参数整理和协议校验所需的最核心字段，不承载完整模板内容。
type TemplateMeta struct {
	Type            string
	CollectMethod   string
	LocalIOChannels []TemplateIOPointMeta
}

// TemplateIOPointMeta 描述 local_io 模板要求的逻辑通道。
type TemplateIOPointMeta struct {
	Channel string
	Access  string
}
