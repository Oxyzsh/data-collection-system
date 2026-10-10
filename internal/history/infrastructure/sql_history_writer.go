package infrastructure

import (
	runtimeinfra "data-collection-system/internal/runtime/infrastructure"
	"data-collection-system/internal/valueobj"
)

type SQLHistoryWriter struct{}

// Add 将采集值交给整点错峰历史写入器。
// 这里不再让采集线程直接写 device_data，而是先在内存汇总每点位最新值与当日极值；
// 整点前的落盘窗口内由后台调度按设备取模错峰批量 UPSERT。
func (SQLHistoryWriter) Add(deviceID, pointID int, value valueobj.CollectedValue, unit, dataType, timestamp string) {
	runtimeinfra.GetHourlyHistoryWriter().Add(deviceID, pointID, value, unit, dataType, timestamp)
}
