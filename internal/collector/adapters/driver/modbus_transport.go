package collector

import (
	modbusspec "data-collection-system/internal/protocol/modbus"
)

// modbusTransportProfile 保留本地别名，避免同一包内大量改名。
type modbusTransportProfile = modbusspec.TransportProfile

// buildModbusTransportProfile 保留本地函数签名，透传到 protocol/modbus。
func buildModbusTransportProfile(params map[string]string) (modbusTransportProfile, error) {
	return modbusspec.BuildTransportProfile(params)
}
