package application

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	alarmdomain "data-collection-system/internal/templatealarm"
	templatebiz "data-collection-system/internal/template/domain"
)

var (
	ErrTemplateNotFound      = errors.New("template not found")
	ErrTemplateBuiltIn       = errors.New("built-in template can not be modified")
	ErrTemplateTypeImmutable = errors.New("template type can not be modified")
)

type TemplateReferencedError struct {
	References []templatebiz.TemplateReference
}

func (e *TemplateReferencedError) Error() string {
	return "template is referenced by devices"
}

type ReloadSummary struct {
	Total   int
	Success int
	Failed  []int
}

func (s ReloadSummary) HasFailures() bool {
	return len(s.Failed) > 0
}

type TemplateConfigRepository interface {
	// repository 负责模板主数据和引用关系的持久化读写。
	GetTemplateMeta(templateID int) (templatebiz.TemplateMeta, error)
	ListTemplateReferences(templateID int) ([]templatebiz.TemplateReference, error)
	DeleteTemplate(templateID int) error
	InsertTemplate(name, templateType, collectMethod, description string, points []templatebiz.DevicePoint, alarms []alarmdomain.DeviceAlarm, now time.Time) (int64, error)
	ReplaceTemplate(templateID int, name, templateType, collectMethod, description string, points []templatebiz.DevicePoint, alarms []alarmdomain.DeviceAlarm, now time.Time) error
}

type TemplateCacheWriter interface {
	// cache writer 负责把更新后的模板写入内存缓存。
	SetTemplate(templateID int, tmpl *templatebiz.DeviceTemplate, collectMethod string, points []templatebiz.DevicePoint, alarms []alarmdomain.DeviceAlarm)
	DeleteTemplate(templateID int)
}

type TemplateReloader interface {
	// 模板变更后，需要重建该模板下所有正在运行的设备。
	RebuildDevicesForTemplate(templateID int) ReloadSummary
}

// ConfigService 负责模板“配置态”的修改。
// 它的工作重点是维护三件事的一致性：
// - 数据库里的模板主数据；
// - 内存缓存里的模板快照；
// - 正在运行、引用该模板的设备采集器。
type ConfigService struct {
	repo     TemplateConfigRepository
	cache    TemplateCacheWriter
	reloader TemplateReloader
}

func NewConfigService(repo TemplateConfigRepository, cache TemplateCacheWriter, reloader TemplateReloader) *ConfigService {
	return &ConfigService{repo: repo, cache: cache, reloader: reloader}
}

type CreateTemplateInput struct {
	Name         string
	Description  string
	TemplateType string
	Points       []templatebiz.DevicePoint
	Alarms       []alarmdomain.DeviceAlarm
}

type ReplaceTemplateInput struct {
	TemplateID   int
	TemplateName string
	Description  string
	TemplateType string
	Points       []templatebiz.DevicePoint
	Alarms       []alarmdomain.DeviceAlarm
}

type ReplaceTemplateResult struct {
	TemplateID int
	Reload     ReloadSummary
}

// DeleteTemplate 删除模板。
// 只有非内置、且没有设备引用的模板才允许删除。
func (s *ConfigService) DeleteTemplate(templateID int) error {
	meta, err := s.repo.GetTemplateMeta(templateID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrTemplateNotFound
		}
		return err
	}

	if meta.IsBuiltIn {
		return ErrTemplateBuiltIn
	}

	references, err := s.repo.ListTemplateReferences(templateID)
	if err != nil {
		return err
	}
	if len(references) > 0 {
		return &TemplateReferencedError{References: references}
	}

	if err := s.repo.DeleteTemplate(templateID); err != nil {
		return err
	}

	s.cache.DeleteTemplate(templateID)
	return nil
}

// CreateTemplate 创建新模板，并写入缓存。
// 新模板创建后不需要重建运行中设备，因为还没有设备引用它。
func (s *ConfigService) CreateTemplate(input CreateTemplateInput) (int, error) {
	templateType := strings.TrimSpace(input.TemplateType)
	if templateType == "" {
		templateType = "modbus"
	}

	points := normalizeTemplatePoints(input.Points)
	if err := templatebiz.ValidatePointsForTemplateType(templateType, points); err != nil {
		return 0, err
	}
	now := time.Now()

	id, err := s.repo.InsertTemplate(
		input.Name,
		templateType,
		templateType,
		input.Description,
		points,
		input.Alarms,
		now,
	)
	if err != nil {
		return 0, err
	}

	nowStr := now.Format("2006-01-02 15:04:05")
	tmpl := &templatebiz.DeviceTemplate{
		ID:          int(id),
		Name:        input.Name,
		Type:        templateType,
		IsBuiltIn:   false,
		Description: input.Description,
		CreatedAt:   nowStr,
		UpdatedAt:   nowStr,
	}
	s.cache.SetTemplate(int(id), tmpl, templateType, points, input.Alarms)
	return int(id), nil
}

// ReplaceTemplate 替换模板内容。
// 对于正在被设备引用的模板，后续会由 reloader 重建相关设备。
// 这里的顺序很关键：
// 1. 先落库；
// 2. 再刷新缓存；
// 3. 最后重建正在运行的设备。
// 这样重建时读到的一定是最新模板快照。
func (s *ConfigService) ReplaceTemplate(input ReplaceTemplateInput) (ReplaceTemplateResult, error) {
	meta, err := s.repo.GetTemplateMeta(input.TemplateID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ReplaceTemplateResult{}, ErrTemplateNotFound
		}
		return ReplaceTemplateResult{}, err
	}

	if meta.IsBuiltIn {
		return ReplaceTemplateResult{}, ErrTemplateBuiltIn
	}
	if input.TemplateType != "" && input.TemplateType != meta.Type {
		return ReplaceTemplateResult{}, fmt.Errorf("%w: 当前类型：%s", ErrTemplateTypeImmutable, meta.Type)
	}

	tmplName := strings.TrimSpace(input.TemplateName)
	if tmplName == "" {
		tmplName = meta.Name
	}
	tmplDesc := strings.TrimSpace(input.Description)
	if tmplDesc == "" {
		tmplDesc = meta.Description
	}

	points := normalizeTemplatePoints(input.Points)
	if err := templatebiz.ValidatePointsForTemplateType(meta.Type, points); err != nil {
		return ReplaceTemplateResult{}, err
	}
	now := time.Now()
	collectMethod := meta.CollectMethod
	if strings.TrimSpace(collectMethod) == "" {
		collectMethod = meta.Type
	}

	if err := s.repo.ReplaceTemplate(
		input.TemplateID,
		tmplName,
		meta.Type,
		collectMethod,
		tmplDesc,
		points,
		input.Alarms,
		now,
	); err != nil {
		return ReplaceTemplateResult{}, err
	}

	nowStr := now.Format("2006-01-02 15:04:05")
	tmpl := &templatebiz.DeviceTemplate{
		ID:          input.TemplateID,
		Name:        tmplName,
		Type:        meta.Type,
		IsBuiltIn:   false,
		Description: tmplDesc,
		UpdatedAt:   nowStr,
	}
	s.cache.SetTemplate(input.TemplateID, tmpl, collectMethod, points, input.Alarms)

	reload := s.reloader.RebuildDevicesForTemplate(input.TemplateID)
	return ReplaceTemplateResult{TemplateID: input.TemplateID, Reload: reload}, nil
}

// normalizeTemplatePoints 统一模板测点类型。
// 导入和接口编辑时，用户输入可能大小写不一致，这里先归一再做校验。
func normalizeTemplatePoints(points []templatebiz.DevicePoint) []templatebiz.DevicePoint {
	if len(points) == 0 {
		return points
	}

	normalized := make([]templatebiz.DevicePoint, len(points))
	copy(normalized, points)

	for i := range normalized {
		pointType := strings.ToUpper(strings.TrimSpace(normalized[i].Type))
		normalized[i].Type = templatebiz.NormalizePointType(pointType)
	}

	return normalized
}
