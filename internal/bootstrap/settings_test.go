package bootstrap

import (
	"os"
	"path/filepath"
	"testing"
)

// serverYAMLMinimal 最小可加载的 server.yaml（内容为空也可，loader 全部取默认值）。
const serverYAMLMinimal = ""

// writeServerYAML 把临时 server.yaml 指给 GOLOOK_CONFIG_FILE（loadSettings 的入口），
// 并屏蔽宿主 /etc/mmdc 等装置对 device.yaml 搜索的干扰（沿用 settings_device_test.go 的可注入点）；
// device.yaml 有「找不到即报错」的约束，同目录放一份最小有效配置（minimalDeviceYAML）。
func writeServerYAML(t *testing.T, content string) {
	t.Helper()
	setupDeviceSearch(t)
	dir := t.TempDir()
	path := filepath.Join(dir, defaultConfigFileName)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, defaultDeviceConfigFileName), []byte(minimalDeviceYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOLOOK_CONFIG_FILE", path)
}

// 窗口配置加载（Review Focus#5）：暂停线无独立配置字段，唯一来源是
// storage.history.flush_window_minutes；yaml 覆盖生效，缺省回落默认 10。
func TestFlushWindowMinutesLoad(t *testing.T) {
	writeServerYAML(t, serverYAMLMinimal+`storage:
  history:
    flush_window_minutes: 15
`)
	settings := loadSettings()
	if settings.Storage.FlushWindowMinutes != 15 {
		t.Fatalf("FlushWindowMinutes = %d, 期望 15", settings.Storage.FlushWindowMinutes)
	}
}

func TestFlushWindowMinutesDefault(t *testing.T) {
	writeServerYAML(t, serverYAMLMinimal)
	settings := loadSettings()
	if settings.Storage.FlushWindowMinutes != 10 {
		t.Fatalf("FlushWindowMinutes 缺省 = %d, 期望默认 10", settings.Storage.FlushWindowMinutes)
	}
}

// server.yaml 的 log.level 叠加到默认值；空值时保持默认 info。
func TestApplyConfigFileLogLevel(t *testing.T) {
	s := defaultSettings()
	var cfg serverConfigFile
	cfg.Log.Level = "debug"
	applyConfigFile(&s, cfg)
	if s.Log.Level != "debug" {
		t.Fatalf("log.level = %q, 期望 debug", s.Log.Level)
	}

	s2 := defaultSettings()
	applyConfigFile(&s2, serverConfigFile{})
	if s2.Log.Level != "info" {
		t.Fatalf("缺省 log.level = %q, 期望默认 info", s2.Log.Level)
	}
}

// 环境变量 GOLOOK_LOG_LEVEL 覆盖 server.yaml。
func TestApplyEnvOverridesLogLevel(t *testing.T) {
	s := defaultSettings()
	t.Setenv("GOLOOK_LOG_LEVEL", "warn")
	applyEnvOverrides(&s)
	if s.Log.Level != "warn" {
		t.Fatalf("log.level = %q, 期望 warn", s.Log.Level)
	}
}

// 仓库布局的向上查找：server.yaml 位于某层目录的 config/ 子目录时也应命中
// （部署布局的平面 server.yaml 优先级不变）。仓库把 server.yaml 移入 config/ 后,
// 仓库内运行的测试与工具依赖本机制自动装载配置——否则每个包都要显式指定
// GOLOOK_CONFIG_FILE, 且经 bootstrap.DataPath 的隐式依赖会随机爆 Fatal。
func TestFindConfigFileFindsConfigSubdir(t *testing.T) {
	t.Setenv("GOLOOK_CONFIG_FILE", "") // 走向上查找路径

	root := t.TempDir()
	configDir := filepath.Join(root, "config")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(configDir, defaultConfigFileName)
	if err := os.WriteFile(want, []byte(serverYAMLMinimal), 0o600); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(sub)

	if got := findConfigFile(); got != want {
		t.Fatalf("findConfigFile() = %q, want %q", got, want)
	}
}
