package template

import (
	"sync"

	collectormodule "data-collection-system/internal/collector"
	"data-collection-system/internal/database"
	templateapp "data-collection-system/internal/template/application"
	templateinfra "data-collection-system/internal/template/infrastructure"
)

// 这个文件主要是“装配代码”，不是业务逻辑代码。
// 读这类文件时，更值得关注的是 Go 的对象组织方式，而不是具体功能细节。
//
// 这里最重要的两个语法/用法：
// 1. sync.Once：保证初始化函数只执行一次，常用于模块级单例。
// 2. 构造函数注入：NewXxx(...) 通过参数接收依赖，而不是在内部自己创建依赖。
var (
	configSvcOnce sync.Once
	configSvc     *templateapp.ConfigService
	querySvcOnce  sync.Once
	querySvc      *templateapp.QueryService
	parserSvcOnce sync.Once
	parserSvc     *templateapp.ParserService
)

type runtimeReloader struct{}

func (runtimeReloader) RebuildDevicesForTemplate(templateID int) templateapp.ReloadSummary {
	summary := collectormodule.GetService().RebuildForTemplate(templateID)
	return templateapp.ReloadSummary{Total: summary.Total, Success: summary.Success, Failed: append([]int(nil), summary.Failed...)}
}

// 与告警配置中的ConfigService类似，这里也是通过sync.Once和构造函数注入的方式，提供模块级单例的ConfigService和QueryService。
// GetConfigService 返回一个模块级单例。
// sync.Once.Do(func() { ... }) 的含义是：
// - 第一次调用时执行这个匿名函数；
// - 以后再调用时，不会重复初始化。
//
// 这里的 func() { ... } 是一个匿名函数。
// 匿名函数可以直接作为参数传给 Do，这在 Go 里很常见。
//
// NewConfigService(...) 这种写法表示“把依赖作为参数传入”。
// service 依赖的是接口能力，不需要知道这些具体类型内部怎么实现。
func GetConfigService() *templateapp.ConfigService {
	configSvcOnce.Do(func() {
		configSvc = templateapp.NewConfigService(
			templateinfra.NewSQLConfigRepository(database.ConfigDB),
			templateinfra.CacheWriter{},
			runtimeReloader{},
		)
	})
	return configSvc
}

// GetQueryService 和上面是同一种单例模式。
func GetQueryService() *templateapp.QueryService {
	querySvcOnce.Do(func() {
		querySvc = templateapp.NewQueryService(
			templateinfra.NewSQLConfigRepository(database.ConfigDB),
			templateinfra.CacheReader{},
		)
	})
	return querySvc
}

// GetParserService 和上面是同一种单例模式。
func GetParserService() *templateapp.ParserService {
	parserSvcOnce.Do(func() {
		parserSvc = templateapp.NewParserService(
			templateinfra.ParserProvider{},
		)
	})
	return parserSvc
}
