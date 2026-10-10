package protocol

import protocolcommon "data-collection-system/internal/protocol/common"

// 对外保持 protocol.ParamType / ParamDefinition 不变，
// 实际定义已下沉到 protocol/common。
type ParamType = protocolcommon.ParamType
type ParamDefinition = protocolcommon.ParamDefinition

const (
	ParamTypeInt    = protocolcommon.ParamTypeInt
	ParamTypeString = protocolcommon.ParamTypeString
	ParamTypeEnum   = protocolcommon.ParamTypeEnum
	ParamTypeBool   = protocolcommon.ParamTypeBool
)

func GetCommonParams() []ParamDefinition {
	return protocolcommon.GetCommonParams()
}

func GetCommonParamDefaults() map[string]string {
	return protocolcommon.GetCommonParamDefaults()
}
