package runtime

import (
	"fmt"

	alarmdomain "data-collection-system/internal/templatealarm"
	templatebiz "data-collection-system/internal/template/domain"
	templateinfra "data-collection-system/internal/template/infrastructure"
)

// Config 为运行链路消费的模板运行配置快照。
// 运行时只需要模板主信息、采集方法、测点和告警，不关心完整模板表字段。
type Config struct {
	Template      *templatebiz.DeviceTemplate
	CollectMethod string
	Points        []templatebiz.DevicePoint
	Alarms        []alarmdomain.DeviceAlarm
}

// GetTemplateConfig 读取模板运行配置（缓存未命中会自动回源 DB）。
// collector/runtime 会通过它拿到“足够启动采集器”的那部分模板数据，
// 而不是直接接触模板模块内部更完整的缓存结构。
// 这种设计的好处是：
// - 隐藏了内部实现细节，降低模块间耦合；
// - 只暴露必要的数据结构，减少误用风险。
func GetTemplateConfig(templateID int) (Config, error) {
	item, ok := templateinfra.CacheReader{}.GetTemplate(templateID)
	if !ok || item == nil {
		return Config{}, fmt.Errorf("template %d not found", templateID)
	}
	return Config{
		Template:      item.Template,
		CollectMethod: item.CollectMethod,
		Points:        item.Points,
		Alarms:        item.Alarms,
	}, nil
}
