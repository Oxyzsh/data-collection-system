package modbus

import (
	"regexp"
	"strings"

	protocolcommon "data-collection-system/internal/protocol/common"
)

// Validator 定义 Modbus 协议专有校验接口
type Validator interface {
	Validate(protocolType string, params map[string]string) []protocolcommon.ValidationError
}

type defaultValidator struct{}

func (v defaultValidator) Validate(protocolType string, params map[string]string) []protocolcommon.ValidationError {
	return ValidateParams(protocolType, params)
}

// ValidateParams 执行 Modbus 协议专有校验（非通用参数类型校验）
func ValidateParams(protocolType string, params map[string]string) []protocolcommon.ValidationError {
	var errs []protocolcommon.ValidationError

	// 串口协议需要额外校验串口路径格式
	if protocolType == ProtocolRS485RTU || protocolType == ProtocolRS232RTU {
		if err := ValidateSerialPort(strings.TrimSpace(params["serial"])); err != nil {
			errs = append(errs, *err)
		}
	}

	return errs
}

// ValidateSerialPort 校验串口路径格式
func ValidateSerialPort(serial string) *protocolcommon.ValidationError {
	if serial == "" {
		return &protocolcommon.ValidationError{
			Param:   "serial",
			Message: "串口路径不能为空",
		}
	}

	// Linux 串口设备名由驱动决定，不能只枚举常见前缀。例如 CH9344 驱动
	// 会创建 /dev/ttyCH9344USB2；这里限制为单个 tty 设备名，避免额外路径层级。
	linuxPattern := `^/dev/tty[A-Za-z0-9._-]+$`
	windowsPattern := `^COM\d+$`

	linuxMatch, _ := regexp.MatchString(linuxPattern, serial)
	windowsMatch, _ := regexp.MatchString(windowsPattern, serial)

	if !linuxMatch && !windowsMatch {
		return &protocolcommon.ValidationError{
			Param:   "serial",
			Message: "串口路径格式无效 (Linux: /dev/ttyUSB0, Windows: COM1)",
		}
	}

	return nil
}
