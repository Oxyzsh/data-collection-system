package domain

import (
	alarmdomain "data-collection-system/internal/templatealarm"
)

// TemplateCacheItem 是模板缓存中的完整条目。
// 缓存里保留模板主信息、采集方法、测点和告警，避免采集链路频繁回查数据库。
type TemplateCacheItem struct {
	Template      *DeviceTemplate
	CollectMethod string
	Points        []DevicePoint
	Alarms        []alarmdomain.DeviceAlarm
}
