package template

import (
	alarmdomain "data-collection-system/internal/templatealarm"
	templatebiz "data-collection-system/internal/template/domain"
)

// 这个文件主要是“对外暴露的接口”，不是业务逻辑代码。
// 读这类文件时，更值得关注的是 Go 的对象组织方式，而不是具体功能细节。
//
// 这里的 type Xxx = Yyy 是 Go 的类型别名语法，表示“Xxx 是 Yyy 的别名”。
// 这种写法的好处是：
// - 可以隐藏内部实现细节，让外部只看到需要的类型；
// - 如果内部实现发生变化，只要保持对外接口不变，就不会影响使用者。
type DevicePoint = templatebiz.DevicePoint
type NormalizedPoint = templatebiz.NormalizedPoint
type NormalizedAlarm = templatebiz.NormalizedAlarm
type TemplateDeviceRef = templatebiz.TemplateDeviceRef
type TemplateDetailResponse = templatebiz.TemplateDetailResponse

// 这个结构体是对外暴露的类型，用于表示从模板文件解析出来的结果。
type TemplateImport struct {
	TemplateName string
	Description  string
	TemplateType string
	Points       []DevicePoint
	Alarms       []alarmdomain.DeviceAlarm
}

// 下面是对外暴露的函数接口，这些函数内部调用了 services.go 中定义的单例服务。
func NormalizePointsWithLang(typ string, points []DevicePoint, isEnglish bool) []NormalizedPoint {
	return templatebiz.NormalizePointsWithLang(typ, points, isEnglish)
}

// NormalizeAlarmsWithLang 和上面是同一种模式。
func NormalizeAlarmsWithLang(typ string, alarms []alarmdomain.DeviceAlarm, isEnglish bool) []NormalizedAlarm {
	return templatebiz.NormalizeAlarmsWithLang(typ, alarms, isEnglish)
}

// GetTemplateDetail 和上面是同一种模式。
func ParseTemplateFileWithType(filePath, userType string) (*TemplateImport, error) {
	parsed, err := GetParserService().ParseTemplateFileWithType(filePath, userType)
	if err != nil {
		return nil, err
	}
	return &TemplateImport{
		TemplateName: parsed.TemplateName,
		Description:  parsed.Description,
		TemplateType: parsed.TemplateType,
		Points:       parsed.Points,
		Alarms:       parsed.Alarms,
	}, nil
}

// GetSupportedTemplateTypes 和上面是同一种模式。
func GetSupportedTemplateTypes() []string {
	return GetParserService().GetSupportedTemplateTypes()
}
