package http

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"data-collection-system/internal/subsystem"
)

// Health 健康检查（指南 §3.2.1）：GET /health
func Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success"})
}

// DeviceTree 设备树（指南 §3.2.2）：
// GET /protocol/sub/system/api/v1/converter/deviceTree
func DeviceTree(c *gin.Context) {
	tree, err := subsystem.BuildDeviceTree()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"errorCode": 500, "errorMsg": err.Error(), "data": nil})
		return
	}
	c.JSON(http.StatusOK, gin.H{"errorCode": 0, "errorMsg": "", "data": tree})
}

// Realtime 测点实时值（指南 §3.2.3）：
// GET /protocol/sub/system/api/v1/converter/realtime?pointStr={pointStr}
func Realtime(c *gin.Context) {
	pointStr := c.Query("pointStr")
	if pointStr == "" {
		c.JSON(http.StatusOK, gin.H{"errorCode": 400, "errorMsg": "缺少 pointStr 参数", "data": nil})
		return
	}

	data, err := subsystem.Realtime(pointStr)
	if err != nil {
		code := 500
		switch {
		case errors.Is(err, subsystem.ErrInvalidPointStr):
			code = 400
		case errors.Is(err, subsystem.ErrPointNotFound):
			code = 404
		case errors.Is(err, subsystem.ErrNoData):
			code = 503
		}
		c.JSON(http.StatusOK, gin.H{"errorCode": code, "errorMsg": err.Error(), "data": nil})
		return
	}
	c.JSON(http.StatusOK, gin.H{"errorCode": 0, "errorMsg": "", "data": data})
}

// SetValue 设置测点值（指南 §3.2.4）：POST /api/set/{pointStr}
// 请求体：{"pointId": "1.3", "value": 28.0}；响应 code 为字符串（"0" 表示成功）。
func SetValue(c *gin.Context) {
	pointStr := c.Param("pointStr")

	var req struct {
		PointID string `json:"pointId"`
		Value   any    `json:"value"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": "1", "message": "请求体解析失败: " + err.Error()})
		return
	}

	if err := subsystem.SetValue(pointStr, req.Value); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": "1", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "0", "message": "success"})
}
