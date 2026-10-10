package http

import (
	"github.com/gin-gonic/gin"
)

// RegisterRoutes 注册模板相关 HTTP 路由。
// 可以粗分成：
// - 查询：templates / template-types；
// - 预检查：review / import/preview；
// - 真正变更：delete / replace / import/confirm。
func RegisterRoutes(apiGroup *gin.RouterGroup) {
	templateGroup := apiGroup.Group("/devices")
	{
		templateGroup.GET("/templates", GetDeviceTemplates)
		templateGroup.GET("/template-types", GetSupportedTemplateTypes)
		templateGroup.GET("/templates/:id/detail", GetTemplateDetail)
		templateGroup.GET("/templates/:id", GetDeviceTemplateByID)
		templateGroup.POST("/templates/:id/review", ReviewTemplate)
		templateGroup.DELETE("/templates/:id", DeleteTemplate)
		templateGroup.POST("/templates/import", ImportTemplatePreview)
		templateGroup.POST("/templates/import/preview", ImportTemplatePreview)
		templateGroup.POST("/templates/import/confirm", ImportTemplateConfirm)
		templateGroup.POST("/templates/upload", UploadTemplate)
		templateGroup.POST("/templates/:id/replace", ReplaceTemplate)
	}
}
