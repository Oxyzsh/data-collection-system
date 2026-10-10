package application

import "data-collection-system/internal/valueobj"

type HistoryWriter interface {
	// 写服务只依赖一个最小写接口，方便替换底层存储实现。
	Add(deviceID, pointID int, value valueobj.CollectedValue, unit, dataType, timestamp string)
}

type WriteService struct {
	writer HistoryWriter
}

func NewWriteService(writer HistoryWriter) *WriteService {
	return &WriteService{writer: writer}
}

// AddPoint 将单个采集值交给历史写入器。
func (s *WriteService) AddPoint(deviceID, pointID int, value valueobj.CollectedValue, unit, dataType, timestamp string) {
	s.writer.Add(deviceID, pointID, value, unit, dataType, timestamp)
}
