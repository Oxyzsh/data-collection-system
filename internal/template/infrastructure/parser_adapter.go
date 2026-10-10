package infrastructure

import (
	alarmdomain "data-collection-system/internal/templatealarm"
	templatebiz "data-collection-system/internal/template/domain"
	templateparsers "data-collection-system/internal/template/infrastructure/parsers"
)

type TemplateImport struct {
	TemplateName string
	Description  string
	TemplateType string
	Points       []templatebiz.DevicePoint
	Alarms       []alarmdomain.DeviceAlarm
}

func ParseTemplateFileWithType(filePath, userType string) (*TemplateImport, error) {
	parsed, err := templateparsers.ParseTemplateFileWithType(filePath, userType)
	if err != nil {
		return nil, err
	}

	return &TemplateImport{
		TemplateName: parsed.TemplateName,
		Description:  parsed.Description,
		TemplateType: parsed.TemplateType,
		Points:       append([]templatebiz.DevicePoint(nil), parsed.Points...),
		Alarms:       append([]alarmdomain.DeviceAlarm(nil), parsed.Alarms...),
	}, nil
}

func GetSupportedTemplateTypes() []string {
	return templateparsers.GetSupportedTypes()
}
