package http

import (
	"strings"

	"github.com/gin-gonic/gin"
	"data-collection-system/internal/interfaces/httpx/response"
	"data-collection-system/internal/protocol"
	"data-collection-system/internal/shared/i18n"
)

// ProtocolTypesResponse 协议类型列表响应
type ProtocolTypesResponse struct {
	Types []protocol.ProtocolInfo `json:"types"`
}

// GetProtocolTypes 获取所有协议类型列表
func GetProtocolTypes(c *gin.Context) {
	types := protocol.GetAllProtocolTypes()
	response.API(c, 200, 200, gin.H{"types": types}, i18n.KeyCommonSuccess)
}

// ProtocolParamsResponse 协议参数响应
type ProtocolParamsResponse struct {
	CommonParams   []protocol.ParamDefinition `json:"common_params"`
	ProtocolParams []protocol.ParamDefinition `json:"protocol_params"`
}

// GetProtocolParams 根据协议类型获取参数定义
func GetProtocolParams(c *gin.Context) {
	protocolType := protocol.ProtocolType(strings.ToUpper(strings.TrimSpace(c.Query("protocol_type"))))

	commonParams := protocol.GetCommonParams()
	var protocolParams []protocol.ParamDefinition
	if protocolType != "" {
		protocolParams = protocol.GetProtocolParams(protocolType)
	}

	response.API(c, 200, 200, gin.H{
		"common_params":   commonParams,
		"protocol_params": protocolParams,
	}, i18n.KeyCommonSuccess)
}
