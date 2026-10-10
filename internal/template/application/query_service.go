package application

import (
	"fmt"
	"strings"

	alarmdomain "data-collection-system/internal/templatealarm"
	templatebiz "data-collection-system/internal/template/domain"
)

type TemplateQueryRepository interface {
	// 查询服务只需要模板引用关系，不直接接触写操作。
	ListTemplateReferences(templateID int) ([]templatebiz.TemplateReference, error)
}

type TemplateCacheReader interface {
	// 读取模板列表和单个模板详情都通过缓存走。
	GetTemplate(templateID int) (*templatebiz.TemplateCacheItem, bool)
	GetTemplates() []*templatebiz.DeviceTemplate
}

type QueryService struct {
	repo  TemplateQueryRepository
	cache TemplateCacheReader
}

// QueryService 负责模板的“查询态”。
// 模板详情优先从缓存读取，因为模板属于被高频复用的基础数据：
// 设备详情会读它，采集器启动会读它，告警配置回退模板默认值时也会读它。
func NewQueryService(repo TemplateQueryRepository, cache TemplateCacheReader) *QueryService {
	return &QueryService{repo: repo, cache: cache}
}

type TemplateDetail struct {
	Template      *templatebiz.DeviceTemplate
	CollectMethod string
	Points        []templatebiz.DevicePoint
	Alarms        []alarmdomain.DeviceAlarm
	References    []templatebiz.TemplateReference
}

type ReviewTemplateInput struct {
	TemplateID   int
	TemplateName string
	TemplateType string
	Points       []templatebiz.DevicePoint
	Alarms       []alarmdomain.DeviceAlarm
}

type ReviewTemplateResult struct {
	TemplateID     int
	TemplateName   string
	TemplateType   string
	ReferenceCount int
	References     []templatebiz.TemplateReference
	PointCount     int
	AlarmCount     int
	Warnings       []ReviewWarning
}

type ReviewWarning struct {
	Key  string
	Args map[string]interface{}
}

// GetTemplates 返回所有已缓存模板的列表。
// 这里直接走 cache，不必每次都回数据库扫模板表和点位表。
func (s *QueryService) GetTemplates() []*templatebiz.DeviceTemplate {
	return s.cache.GetTemplates()
}

// GetTemplateDetail 返回单个模板详情。
// 详情主体从缓存拿，引用关系再从 repository 查，
// 因为“模板内容”和“有多少设备引用它”是两类更新频率不同的数据。
func (s *QueryService) GetTemplateDetail(templateID int) (TemplateDetail, error) {
	item, ok := s.cache.GetTemplate(templateID)
	if !ok {
		return TemplateDetail{}, ErrTemplateNotFound
	}

	references, err := s.repo.ListTemplateReferences(templateID)
	if err != nil {
		return TemplateDetail{}, err
	}

	return TemplateDetail{
		Template:      item.Template,
		CollectMethod: item.CollectMethod,
		Points:        item.Points,
		Alarms:        item.Alarms,
		References:    references,
	}, nil
}

// ReviewTemplate 在不落库的情况下预检查模板修改结果。
// 用于“替换前预览”之类的接口。
// 它相当于一次 dry-run：验证类型约束、统计变更影响、生成告警提示，但不真正保存。
func (s *QueryService) ReviewTemplate(input ReviewTemplateInput) (ReviewTemplateResult, error) {
	item, ok := s.cache.GetTemplate(input.TemplateID)
	if !ok {
		return ReviewTemplateResult{}, ErrTemplateNotFound
	}

	if input.TemplateType != "" && input.TemplateType != item.Template.Type {
		return ReviewTemplateResult{}, fmt.Errorf("%w: 当前类型：%s", ErrTemplateTypeImmutable, item.Template.Type)
	}
	//这里的验证逻辑主要是针对点位的，因为点位的合法性通常依赖于模板类型。
	normalizedPoints := normalizeTemplatePoints(input.Points)
	if err := templatebiz.ValidatePointsForTemplateType(item.Template.Type, normalizedPoints); err != nil {
		return ReviewTemplateResult{}, err
	}
	references, err := s.repo.ListTemplateReferences(input.TemplateID)
	if err != nil {
		return ReviewTemplateResult{}, err
	}
	//下面的逻辑主要是根据变更内容和引用情况生成一些预览警告，提示用户可能的影响和注意事项。
	warnings := make([]ReviewWarning, 0, 4)
	if len(references) > 0 {
		warnings = append(warnings, ReviewWarning{
			Key:  "template.review.warn.referenced",
			Args: map[string]interface{}{"count": len(references)},
		})
	}
	if len(normalizedPoints) != len(item.Points) {
		warnings = append(warnings, ReviewWarning{
			Key: "template.review.warn.point_count_changed",
			Args: map[string]interface{}{
				"from": len(item.Points),
				"to":   len(normalizedPoints),
			},
		})
	}
	if len(input.Alarms) != len(item.Alarms) {
		warnings = append(warnings, ReviewWarning{
			Key: "template.review.warn.alarm_count_changed",
			Args: map[string]interface{}{
				"from": len(item.Alarms),
				"to":   len(input.Alarms),
			},
		})
	}
	if strings.TrimSpace(input.TemplateName) != "" && input.TemplateName != item.Template.Name {
		warnings = append(warnings, ReviewWarning{
			Key: "template.review.warn.name_changed",
			Args: map[string]interface{}{
				"from": item.Template.Name,
				"to":   input.TemplateName,
			},
		})
	}
	if len(warnings) == 0 {
		warnings = append(warnings, ReviewWarning{Key: "template.review.warn.ok"})
	}

	return ReviewTemplateResult{
		TemplateID:     input.TemplateID,
		TemplateName:   item.Template.Name,
		TemplateType:   item.Template.Type,
		ReferenceCount: len(references),
		References:     references,
		PointCount:     len(normalizedPoints),
		AlarmCount:     len(input.Alarms),
		Warnings:       warnings,
	}, nil
}
