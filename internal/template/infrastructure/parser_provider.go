package infrastructure

import templateapp "data-collection-system/internal/template/application"

type ParserProvider struct{}

// ParseTemplateFileWithType 调用底层 Excel 解析器并转换为 application 层输入。
func (ParserProvider) ParseTemplateFileWithType(filePath, userType string) (*templateapp.TemplateImport, error) {
	parsed, err := ParseTemplateFileWithType(filePath, userType)
	if err != nil {
		return nil, err
	}
	return &templateapp.TemplateImport{
		TemplateName: parsed.TemplateName,
		Description:  parsed.Description,
		TemplateType: parsed.TemplateType,
		Points:       parsed.Points,
		Alarms:       parsed.Alarms,
	}, nil
}

// GetSupportedTemplateTypes 返回当前支持导入的模板类型列表。
func (ParserProvider) GetSupportedTemplateTypes() []string {
	return GetSupportedTemplateTypes()
}
