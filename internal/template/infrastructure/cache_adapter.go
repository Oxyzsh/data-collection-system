package infrastructure

import (
	alarmdomain "data-collection-system/internal/templatealarm"
	"data-collection-system/internal/database"
	templatebiz "data-collection-system/internal/template/domain"
)

// 这个文件主要是“缓存适配器”，不是业务逻辑代码。
// 读这类文件时，更值得关注的是 Go 的对象组织方式，而不是具体功能细节。
//
// 这里定义了 CacheWriter 和 CacheReader 两个结构体，分别负责写缓存和读缓存。
// 这种分离的好处是：
// - 写缓存和读缓存的职责清晰，代码更易维护；
// - 如果将来需要改成分布式缓存，只需要修改这两个结构体的实现，不会影响其他业务逻辑。

//为什么这里不使用接口？
// 这里没有定义接口，是因为目前只有一个缓存实现（内存缓存），没有多种实现的需求。
// 如果将来需要支持多种缓存实现，可以考虑引入接口来抽象读写操作。

//解释接口和方法接收者的关系：
// - 在 Go 中，接口是一组方法的集合，任何实现了这些方法的类型都满足这个接口。
// - 方法接收者（如 func (CacheWriter) SetTemplate(...)）是定义在具体类型上的方法，这些方法可以被接口调用。
// - 通过定义 CacheWriter 和 CacheReader 的方法，我们实际上是在为这些类型实现某些接口（即使我们没有显式定义接口）。

type CacheWriter struct{}

func (CacheWriter) SetTemplate(templateID int, tmpl *templatebiz.DeviceTemplate, collectMethod string, points []templatebiz.DevicePoint, alarms []alarmdomain.DeviceAlarm) {
	SetTemplateToCacheWithMethod(templateID, tmpl, collectMethod, points, copyAlarms(alarms))
}

func (CacheWriter) DeleteTemplate(templateID int) {
	DeleteTemplateFromCache(templateID)
}

type CacheReader struct{}

func (CacheReader) GetTemplate(templateID int) (*templatebiz.TemplateCacheItem, bool) {
	return GetTemplateFromCacheOrLoad(database.ConfigDB, templateID)
}

func (CacheReader) GetTemplates() []*templatebiz.DeviceTemplate {
	return GetAllCachedTemplates()
}
