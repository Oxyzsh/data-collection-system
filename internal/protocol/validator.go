package protocol

import (
	protocolcommon "data-collection-system/internal/protocol/common"
)

// 对外保持 protocol.ValidationError / ValidateParamValue 兼容
type ValidationError = protocolcommon.ValidationError

func ValidateParamValue(param ParamDefinition, value string) *ValidationError {
	return protocolcommon.ValidateParamValue(param, value)
}

// ValidateDeviceParams 校验设备参数
// - 公共参数：通用校验
// - 协议参数：通用校验
// - 协议专有逻辑：委托子协议 validator
func ValidateDeviceParams(protocolType ProtocolType, params map[string]string) []ValidationError {
	var errs []ValidationError

	for _, param := range GetCommonParams() {
		if err := ValidateParamValue(param, params[param.Name]); err != nil {
			errs = append(errs, *err)
		}
	}

	for _, param := range GetProtocolParams(protocolType) {
		if err := ValidateParamValue(param, params[param.Name]); err != nil {
			errs = append(errs, *err)
		}
	}

	if validator := getProtocolValidator(protocolType); validator != nil {
		errs = append(errs, validator(params)...)
	}

	return errs
}

// ApplyParamDefaults 应用参数默认值
func ApplyParamDefaults(protocolType ProtocolType, params map[string]string) map[string]string {
	result := make(map[string]string)

	for _, param := range GetCommonParams() {
		if params[param.Name] != "" {
			result[param.Name] = params[param.Name]
		} else if param.Default != "" {
			result[param.Name] = param.Default
		}
	}

	for _, param := range GetProtocolParams(protocolType) {
		if params[param.Name] != "" {
			result[param.Name] = params[param.Name]
		} else if param.Default != "" {
			result[param.Name] = param.Default
		}
	}

	return result
}
