package application

import "database/sql"

// DeviceListItem / DeviceDetail / ParamEntry / HistoryPoint 都是查询用例的输出模型。
// 它们不一定和数据库表一一对应，而是“这个用例要返回什么，就组织成什么”。
type DeviceListItem struct {
	ID           int
	Name         string
	TemplateID   int
	Status       string
	Description  string
	CreatedAt    string
	UpdatedAt    string
	Enabled      bool
	TemplateName string
	IOChannel    string
}

type DeviceDetail struct {
	ID            int
	Name          string
	TemplateID    int
	Status        string
	Description   string
	CreatedAt     string
	UpdatedAt     string
	Enabled       bool
	TemplateName  string
	TemplateType  string
	CollectMethod string
	Params        map[string]string
}

type ParamEntry struct {
	Name  string
	Value string
}

type HistoryPoint struct {
	Timestamp string
	Value     interface{}
	ValueKind string
}

type DeviceQueryRepository interface {
	// repository 负责拿“原始查询结果”；
	// service 再决定是否要继续拼装、映射错误、组织成更完整的返回值。
	ListDevicesWithTemplate() ([]DeviceListItem, error)
	GetDeviceDetail(deviceID int) (DeviceDetail, error)
	GetDeviceParams(deviceID int) ([]ParamEntry, error)
	GetDeviceHistory(deviceID int, pointID, limit string) ([]HistoryPoint, error)
}

type QueryService struct {
	repo DeviceQueryRepository
}

func NewQueryService(repo DeviceQueryRepository) *QueryService {
	return &QueryService{repo: repo}
}

// GetDevices 是最薄的一类查询：service 只转发，不额外拼装。
func (s *QueryService) GetDevices() ([]DeviceListItem, error) {
	return s.repo.ListDevicesWithTemplate()
}

// GetDeviceByID 是“先查主记录，再查参数表，再拼成完整详情”的典型查询流程。
// 这里顺手把 sql.ErrNoRows 统一翻译成业务层错误 ErrDeviceNotFound，
// 避免 HTTP 层直接依赖数据库错误常量。
func (s *QueryService) GetDeviceByID(deviceID int) (DeviceDetail, error) {
	detail, err := s.repo.GetDeviceDetail(deviceID)
	if err != nil {
		if err == sql.ErrNoRows {
			return DeviceDetail{}, ErrDeviceNotFound
		}
		return DeviceDetail{}, err
	}
	params, err := s.repo.GetDeviceParams(deviceID)
	if err != nil {
		return DeviceDetail{}, err
	}
	detail.Params = make(map[string]string, len(params))
	for _, p := range params {
		detail.Params[p.Name] = p.Value
	}
	return detail, nil
}

func (s *QueryService) GetDeviceParams(deviceID int) ([]ParamEntry, error) {
	return s.repo.GetDeviceParams(deviceID)
}

// GetDeviceHistory 直接把历史查询下放给 repository。
// 这是因为这里暂时不需要额外业务编排，service 只保留统一边界。
func (s *QueryService) GetDeviceHistory(deviceID int, pointID, limit string) ([]HistoryPoint, error) {
	return s.repo.GetDeviceHistory(deviceID, pointID, limit)
}
