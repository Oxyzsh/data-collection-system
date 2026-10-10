package http

import (
	"net/http"
	"strconv"

	devicemodule "data-collection-system/internal/device"
	deviceapp "data-collection-system/internal/device/application"
	"data-collection-system/internal/interfaces/httpx/response"
	"data-collection-system/internal/shared/i18n"

	"github.com/gin-gonic/gin"
)

type selectorValuesRequest struct {
	Items []deviceapp.SelectorValueRequest `json:"items"`
}

// GetSelectorPoints 返回某个设备可供 selector 使用的点位。
// selector 可以理解成“从设备状态里挑一种值供别的模块/前端绑定”的能力，
// 所以这里返回的不只是点，还包括该点支持哪些取值类型。
func GetSelectorPoints(c *gin.Context) {
	deviceID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.API(c, http.StatusBadRequest, 400, nil, "无效的设备ID")
		return
	}

	numericOnly := c.Query("numeric_only") == "1"
	points, err := devicemodule.GetSelectorService().ListPoints(deviceID, numericOnly)
	if err != nil {
		response.API(c, http.StatusInternalServerError, 500, nil, "获取选择器测点失败")
		return
	}

	response.API(c, http.StatusOK, 200, points, i18n.KeyCommonSuccess)
}

// GetSelectorValues 批量查询 selector 值。
// 这里走普通 HTTP 请求；如果是持续推送场景，则走 WebSocket + SelectorRealtimeService。
func GetSelectorValues(c *gin.Context) {
	var req selectorValuesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.API(c, http.StatusBadRequest, 400, nil, i18n.KeyCommonInvalidParams)
		return
	}
	results, err := devicemodule.GetSelectorService().GetValues(req.Items)
	if err != nil {
		response.API(c, http.StatusInternalServerError, 500, nil, "获取选择器值失败")
		return
	}

	response.API(c, http.StatusOK, 200, gin.H{"items": results}, i18n.KeyCommonSuccess)
}
