package common

import (
	"fmt"
	"net"
	"strconv"
	"strings"
)

// ValidationError 校验错误
type ValidationError struct {
	Param   string `json:"param"`
	Message string `json:"message"`
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("参数 %s: %s", e.Param, e.Message)
}

// ValidateParamValue 校验单个参数值（协议无关）
func ValidateParamValue(param ParamDefinition, value string) *ValidationError {
	if param.Required && value == "" {
		return &ValidationError{
			Param:   param.Name,
			Message: "是必填项",
		}
	}

	if value == "" {
		return nil
	}

	switch param.Type {
	case ParamTypeInt:
		v, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return &ValidationError{
				Param:   param.Name,
				Message: "必须是整数",
			}
		}
		if param.Min != 0 || param.Max != 0 {
			if v < param.Min || v > param.Max {
				return &ValidationError{
					Param:   param.Name,
					Message: fmt.Sprintf("必须在 %.0f 到 %.0f 之间", param.Min, param.Max),
				}
			}
		}
	case ParamTypeEnum:
		valid := false
		for _, opt := range param.Options {
			if value == opt {
				valid = true
				break
			}
		}
		if !valid {
			return &ValidationError{
				Param:   param.Name,
				Message: fmt.Sprintf("必须是以下值之一: %s", strings.Join(param.Options, ", ")),
			}
		}
	case ParamTypeBool:
		lower := strings.ToLower(value)
		if lower != "true" && lower != "false" && value != "0" && value != "1" {
			return &ValidationError{
				Param:   param.Name,
				Message: "必须是布尔值 (true/false 或 0/1)",
			}
		}
	case ParamTypeString:
		if param.Name == "ip_address" && net.ParseIP(value) == nil {
			return &ValidationError{
				Param:   param.Name,
				Message: "IP地址格式无效",
			}
		}
	}

	return nil
}
