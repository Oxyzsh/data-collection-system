package http

import "github.com/gin-gonic/gin"

// RegisterRoutes 只负责把 URL 路径绑定到 handler。
// 路由层本身不承载业务逻辑，它更像“HTTP 入口清单”。
func RegisterRoutes(apiGroup *gin.RouterGroup) {
	deviceGroup := apiGroup.Group("/devices")
	{
		deviceGroup.GET("", GetDevices)
		deviceGroup.POST("", AddDevice)
		deviceGroup.GET("/:id", GetDeviceByID)
		deviceGroup.PUT("/:id", UpdateDevice)
		deviceGroup.DELETE("/:id", DeleteDevice)
		deviceGroup.POST("/:id/start", StartDevice)
		deviceGroup.POST("/:id/stop", StopDevice)
		deviceGroup.POST("/:id/copy", CopyDevice)
		deviceGroup.GET("/:id/points", GetDevicePoints)
		deviceGroup.POST("/:id/points/:point_id/write", WriteDevicePoint)
		deviceGroup.GET("/:id/selector-points", GetSelectorPoints)
		deviceGroup.PUT("/:id/points/:point_id/config", SaveDevicePointConfig)
		deviceGroup.DELETE("/:id/points/:point_id/config", DeleteDevicePointConfig)
		deviceGroup.GET("/:id/history", GetDeviceHistory)
		deviceGroup.PUT("/:id/enabled", SetDeviceEnabled)
		deviceGroup.GET("/:id/params", GetDeviceParams)
		deviceGroup.PUT("/:id/params", UpdateDeviceParams)
		deviceGroup.POST("/selector-values", GetSelectorValues)

		deviceGroup.GET("/protocols/types", GetProtocolTypes)
		deviceGroup.GET("/protocols/params", GetProtocolParams)
	}
}
