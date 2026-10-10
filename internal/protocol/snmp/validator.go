package snmp

import (
	"strings"

	protocolcommon "data-collection-system/internal/protocol/common"
)

// Validator 定义 SNMP 协议专有校验接口
type Validator interface {
	Validate(params map[string]string) []protocolcommon.ValidationError
}

type defaultValidator struct{}

func (v defaultValidator) Validate(params map[string]string) []protocolcommon.ValidationError {
	return ValidateParams(params)
}

// ValidateParams 执行 SNMP 协议专有条件校验（v3 认证/加密组合规则）
func ValidateParams(params map[string]string) []protocolcommon.ValidationError {
	var errs []protocolcommon.ValidationError

	snmpVersion := strings.TrimSpace(params["snmp_v"])
	if snmpVersion != "3" {
		return errs
	}

	if strings.TrimSpace(params["user_name"]) == "" {
		errs = append(errs, protocolcommon.ValidationError{
			Param:   "user_name",
			Message: "SNMPv3 为必填项",
		})
	}

	securityLevel := strings.TrimSpace(params["security_level"])
	if securityLevel == "" {
		errs = append(errs, protocolcommon.ValidationError{
			Param:   "security_level",
			Message: "SNMPv3 安全级别为必填项",
		})
		return errs
	}

	switch securityLevel {
	case "noAuthNoPriv":
		// 仅需要 user_name
	case "authNoPriv":
		if strings.TrimSpace(params["auth_protocol"]) == "" {
			errs = append(errs, protocolcommon.ValidationError{
				Param:   "auth_protocol",
				Message: "SNMPv3 认证协议为必填项",
			})
		}
		if strings.TrimSpace(params["auth_password"]) == "" {
			errs = append(errs, protocolcommon.ValidationError{
				Param:   "auth_password",
				Message: "SNMPv3 认证密码为必填项",
			})
		}
	case "authPriv":
		if strings.TrimSpace(params["auth_protocol"]) == "" {
			errs = append(errs, protocolcommon.ValidationError{
				Param:   "auth_protocol",
				Message: "SNMPv3 认证协议为必填项",
			})
		}
		if strings.TrimSpace(params["auth_password"]) == "" {
			errs = append(errs, protocolcommon.ValidationError{
				Param:   "auth_password",
				Message: "SNMPv3 认证密码为必填项",
			})
		}
		if strings.TrimSpace(params["priv_protocol"]) == "" {
			errs = append(errs, protocolcommon.ValidationError{
				Param:   "priv_protocol",
				Message: "SNMPv3 加密协议为必填项",
			})
		}
		if strings.TrimSpace(params["priv_password"]) == "" {
			errs = append(errs, protocolcommon.ValidationError{
				Param:   "priv_password",
				Message: "SNMPv3 加密密码为必填项",
			})
		}
	default:
		errs = append(errs, protocolcommon.ValidationError{
			Param:   "security_level",
			Message: "SNMPv3 安全级别必须是 noAuthNoPriv/authNoPriv/authPriv 之一",
		})
	}

	return errs
}
