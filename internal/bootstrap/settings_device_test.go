package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const minimalDeviceYAML = `version: 1
protocols:
  rs485_rtu:
    - label: COM1
      value: /dev/ttyS1
`

// setupDeviceSearch 把两个可注入点指向临时目录，避免测试命中真实 /etc/mmdc
// 或仓库里遗留的 device.yaml。返回可执行文件目录，便于构造 config/device.yaml。
func setupDeviceSearch(t *testing.T) (exeDir string) {
	t.Helper()
	exeDir = t.TempDir()
	origSystem, origExe := systemDeviceConfigPath, deviceExecutableDir
	systemDeviceConfigPath = filepath.Join(t.TempDir(), "device.yaml") // 默认不存在
	deviceExecutableDir = func() string { return exeDir }
	t.Cleanup(func() {
		systemDeviceConfigPath, deviceExecutableDir = origSystem, origExe
	})
	return exeDir
}

func writeDeviceYAML(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(minimalDeviceYAML), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestFindDeviceConfigPrefersSystemPath(t *testing.T) {
	exeDir := setupDeviceSearch(t)
	writeDeviceYAML(t, filepath.Join(exeDir, "config", "device.yaml"))
	writeDeviceYAML(t, systemDeviceConfigPath)

	got := findDeviceConfigFile()
	if got != systemDeviceConfigPath {
		t.Fatalf("findDeviceConfigFile() = %q, want %q", got, systemDeviceConfigPath)
	}
}

func TestFindDeviceConfigFallsBackToBundled(t *testing.T) {
	exeDir := setupDeviceSearch(t)
	bundled := filepath.Join(exeDir, "config", "device.yaml")
	writeDeviceYAML(t, bundled)

	got := findDeviceConfigFile()
	if got != bundled {
		t.Fatalf("findDeviceConfigFile() = %q, want %q", got, bundled)
	}
}

// 程序目录及向上各级的 device.yaml 是已废弃的旧位置，不能再被命中，
// 否则现场改了 config/ 下的配置却读到一份历史文件，问题极难定位。
func TestFindDeviceConfigIgnoresLegacyPositions(t *testing.T) {
	exeDir := setupDeviceSearch(t)
	writeDeviceYAML(t, filepath.Join(exeDir, "device.yaml"))

	if got := findDeviceConfigFile(); got != "" {
		t.Fatalf("findDeviceConfigFile() = %q，旧位置不应再被命中", got)
	}
}

func TestFindDeviceConfigReturnsEmptyWhenNothingFound(t *testing.T) {
	setupDeviceSearch(t)
	if got := findDeviceConfigFile(); got != "" {
		t.Fatalf("findDeviceConfigFile() = %q, want 空串", got)
	}
}

// 现场配置存在但内容非法时必须报错，而不是静默回落到自带默认。
func TestLoadDeviceSettingsFailsOnInvalidSystemConfig(t *testing.T) {
	setupDeviceSearch(t)
	if err := os.WriteFile(systemDeviceConfigPath, []byte("version: 2\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, path, err := loadDeviceSettings()
	if err == nil {
		t.Fatal("期望非法内容报错，实际返回 nil")
	}
	if path != systemDeviceConfigPath {
		t.Fatalf("报错时应指出文件路径，got %q", path)
	}
}

// 启动期 panic 只打印 err 本身，因此路径必须由 err 携带：
// device.yaml 有两个候选位置，不点明读的是哪一个，现场就会以为生效的是另一个文件。
func TestLoadDeviceSettingsErrorNamesPath(t *testing.T) {
	setupDeviceSearch(t)
	if err := os.WriteFile(systemDeviceConfigPath, []byte("version: 2\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, path, err := loadDeviceSettings()
	if err == nil {
		t.Fatal("期望非法内容报错，实际返回 nil")
	}
	if !strings.Contains(err.Error(), path) {
		t.Fatalf("错误信息应包含路径 %q，got %v", path, err)
	}
}
