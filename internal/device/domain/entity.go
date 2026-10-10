package domain

// Device 是设备表对应的领域对象。
// 这里更偏“配置态/持久化视角”，描述的是数据库里保存的设备基本信息，
// 而不是运行时采集线程、实时值、告警状态这些高频内存态数据。
type Device struct {
	ID int `json:"id" db:"id"`
	// Name 是设备实例名，用来区分同一模板下的不同设备。
	Name string `json:"name" db:"name"`
	// TemplateID 指向设备使用的模板，决定它有哪些点位、协议参数和默认告警规则。
	TemplateID int `json:"template_id" db:"template_id"`
	// Status 描述当前运行状态，如 running/stopped。
	// 它表示“现在是否在跑”，不是“配置上是否允许跑”。
	Status string `json:"status" db:"status"`
	// Enabled 表示配置上是否启用。
	// 这里保留 int 是为了贴合数据库字段；在更强调业务语义的结构里，会转成 bool。
	Enabled     int    `json:"enabled" db:"enabled"`
	Description string `json:"description" db:"description"`
	CreatedAt   string `json:"created_at" db:"created_at"`
	UpdatedAt   string `json:"updated_at" db:"updated_at"`
}
