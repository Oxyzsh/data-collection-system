package application

import (
	alarmdomain "data-collection-system/internal/templatealarm"
	templatebiz "data-collection-system/internal/template/domain"
)

type TemplateImport struct {
	TemplateName string
	Description  string
	TemplateType string
	Points       []templatebiz.DevicePoint
	Alarms       []alarmdomain.DeviceAlarm
}

type ParserProvider interface {
	// 解析器提供者统一屏蔽不同文件格式和协议类型。
	ParseTemplateFileWithType(filePath, userType string) (*TemplateImport, error)
	GetSupportedTemplateTypes() []string
}

type ParserService struct {
	provider ParserProvider
}

// ParserService 负责“文件 -> 模板内存结构”的转换。
// 它不直接保存模板，只负责把外部文件解析成后续 Create/ReplaceTemplate 可消费的结构。
func NewParserService(provider ParserProvider) *ParserService {
	return &ParserService{provider: provider}
}

// ParseTemplateFileWithType 调用底层提供者解析模板文件。
// application 层通过 provider 屏蔽具体文件格式差异，例如 Excel、协议模板等。
func (s *ParserService) ParseTemplateFileWithType(filePath, userType string) (*TemplateImport, error) {
	return s.provider.ParseTemplateFileWithType(filePath, userType)
}

// GetSupportedTemplateTypes 返回当前支持导入的模板类型。
func (s *ParserService) GetSupportedTemplateTypes() []string {
	return s.provider.GetSupportedTemplateTypes()
}
