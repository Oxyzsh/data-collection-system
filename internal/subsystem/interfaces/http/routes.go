package http

import "github.com/gin-gonic/gin"

// RegisterRoutes 注册对外提供的三方子系统接口（路径与对接指南 §3.1 一致）：
//
//	GET  /health                                            健康检查
//	GET  /protocol/sub/system/api/v1/converter/deviceTree   设备树
//	GET  /protocol/sub/system/api/v1/converter/realtime     测点实时值
//	POST /api/set/{pointStr}                                设置测点值
func RegisterRoutes(r *gin.Engine) {
	r.GET("/health", Health)
	r.GET("/protocol/sub/system/api/v1/converter/deviceTree", DeviceTree)
	r.GET("/protocol/sub/system/api/v1/converter/realtime", Realtime)
	r.POST("/api/set/:pointStr", SetValue)
}
