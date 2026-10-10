package http

import (
	"github.com/gin-gonic/gin"

	devicehttp "data-collection-system/internal/device/interfaces/http"
	"data-collection-system/internal/devicecatalog"
	"data-collection-system/internal/interfaces/httpx/response"
	"data-collection-system/internal/shared/i18n"
	templatehttp "data-collection-system/internal/template/interfaces/http"
)

// RegisterRoutes 装配裁剪后「数据采集进程」的 HTTP 路由：
//
//	设备模板导入/查询 + 按模板添加设备/设备管理 + 设备接入目录。
//
// 已裁剪：认证、告警、操作日志、系统运维（网络/NTP/升级）、视频、门禁、
// 组态画面、联动、北向接口、维护调度等模块及其路由。
func RegisterRoutes(r *gin.Engine) {
	apiGroup := r.Group("/api")
	{
		// 设备接入目录：添加设备时查询 device.yaml 中可选的串口 / 本地 IO 端点。
		apiGroup.GET("/device-catalog", func(c *gin.Context) {
			entries := devicecatalog.Get().List(c.Query("selector"))
			array := make([]gin.H, 0, len(entries))
			for _, entry := range entries {
				array = append(array, gin.H{
					"ref":      entry.Ref,
					"label":    entry.Label,
					"kind":     entry.Kind,
					"selector": entry.Selector,
					"summary":  entry.Summary,
					"defaults": entry.Defaults,
					"channels": entry.Channels,
				})
			}
			response.API(c, 200, 200, gin.H{"array": array}, i18n.KeyCommonSuccess)
		})

		// 设备模板导入 / 查询。
		templatehttp.RegisterRoutes(apiGroup)
		// 按模板添加设备、设备配置与测点、历史查询。
		devicehttp.RegisterRoutes(apiGroup)
	}
}
