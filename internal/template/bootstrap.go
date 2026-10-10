package template

import (
	"data-collection-system/internal/database"
	templateinfra "data-collection-system/internal/template/infrastructure"
)

// InitOnStartup 初始化模板域运行所需的缓存与内置模板。
// 启动时先预热缓存，再把数据库中的模板和内置模板装入内存。
func InitOnStartup() error {
	return templateinfra.InitOnStartup(database.ConfigDB)
}
