package modbus

import "testing"

func TestValidateSerialPort(t *testing.T) {
	tests := []struct {
		name    string
		serial  string
		wantErr bool
	}{
		{name: "native tty", serial: "/dev/ttyS1"},
		{name: "usb serial", serial: "/dev/ttyUSB0"},
		{name: "acm serial", serial: "/dev/ttyACM0"},
		{name: "ch9344 usb serial", serial: "/dev/ttyCH9344USB2"},
		{name: "windows serial", serial: "COM3"},
		{name: "empty", serial: "", wantErr: true},
		{name: "missing tty prefix", serial: "/dev/USB0", wantErr: true},
		{name: "nested path", serial: "/dev/ttyUSB0/child", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSerialPort(tt.serial)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateSerialPort(%q) error = %v, wantErr %v", tt.serial, err, tt.wantErr)
			}
		})
	}
}
