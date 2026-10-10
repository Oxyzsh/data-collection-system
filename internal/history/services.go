package history

import (
	"sync"

	historyapp "data-collection-system/internal/history/application"
	historyinfra "data-collection-system/internal/history/infrastructure"
)

var (
	writeServiceOnce sync.Once
	writeService     *historyapp.WriteService
)

// GetWriteService 返回采集历史写入服务（内存缓冲 + 整点错峰批量落库）。
func GetWriteService() *historyapp.WriteService {
	writeServiceOnce.Do(func() {
		writeService = historyapp.NewWriteService(historyinfra.SQLHistoryWriter{})
	})
	return writeService
}
