package bootstrap

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"data-collection-system/internal/shared/logger"

	"github.com/goccy/go-yaml"
)

const (
	defaultConfigFileName = "server.yaml"
	defaultDataDirName    = "data"
)

// Settings 是后端启动期统一装载的配置快照。
//
// 约定：
// - ConfigPath / ConfigDir 描述配置文件来源；
// - DataDir 是运行时数据根目录，所有 data/* 路径都从这里派生；
// - 端口 / 日志级别 / 历史落盘参数可由 server.yaml 或环境变量覆盖；
// - Device / DeviceConfigPath 是 device.yaml 的启动期只读快照。
type Settings struct {
	ConfigPath       string
	ConfigDir        string
	DataDir          string
	ServerPort       string
	Storage          StorageSettings
	Log              LogSettings
	Device           DeviceSettings
	DeviceConfigPath string
}

// StorageSettings 是采集历史落盘相关参数。
type StorageSettings struct {
	// FlushWindowMinutes 落盘窗口：整点前 N 分钟，设备按 id 取模错峰落盘（默认 10）
	FlushWindowMinutes int `yaml:"flush_window_minutes"`
	// ExtremumFlushInterval 极值 dirty 落盘周期（默认 5s）
	ExtremumFlushInterval time.Duration `yaml:"extremum_flush_interval"`
}

// LogSettings 日志配置：级别 debug|info|warn|error（默认 info，重启生效）。
type LogSettings struct {
	Level string `yaml:"level"`
}

type serverConfigFile struct {
	Server struct {
		Port string `yaml:"port"`
	} `yaml:"server"`
	Paths struct {
		DataDir string `yaml:"data_dir"`
	} `yaml:"paths"`
	Storage struct {
		History struct {
			FlushWindowMinutes    int           `yaml:"flush_window_minutes"`
			ExtremumFlushInterval time.Duration `yaml:"extremum_flush_interval"`
		} `yaml:"history"`
	} `yaml:"storage"`
	Log struct {
		Level string `yaml:"level"`
	} `yaml:"log"`
}

var (
	settingsOnce   sync.Once
	loadedSettings Settings
)

// LoadSettings 装载并返回启动配置快照（进程内只装载一次）。
func LoadSettings() Settings {
	return currentSettings()
}

// ConfigPath 返回当前生效的 server.yaml 绝对路径（可能为空，表示使用默认配置）。
func ConfigPath() string {
	return currentSettings().ConfigPath
}

// ConfigDir 返回当前生效的 server.yaml 所在目录。
func ConfigDir() string {
	return currentSettings().ConfigDir
}

// DataDir 返回当前数据根目录的绝对路径。
func DataDir() string {
	return currentSettings().DataDir
}

// DataPath 将相对于 data 根目录的路径转换为绝对路径。
//
// 传入的 part 可以是后端子目录或文件名，不要带前缀 data/。
//
// 注意：首次调用会触发配置一次性装载（server.yaml + 环境变量覆盖），此后进程内不再重读。
// 不要在包级变量初始化里调用本函数 —— 那会把装载时机提前到包 init 期，
// 使测试无法再用 GOLOOK_DATA_DIR 指定数据目录，配置缺失时还会在 init 期直接退出进程。
func DataPath(parts ...string) string {
	base := currentSettings().DataDir
	if base == "" {
		base = defaultDataDirName
	}
	elems := make([]string, 0, len(parts)+1)
	elems = append(elems, base)
	for _, part := range parts {
		if part == "" {
			continue
		}
		elems = append(elems, filepath.FromSlash(part))
	}
	return filepath.Clean(filepath.Join(elems...))
}

func currentSettings() Settings {
	settingsOnce.Do(func() {
		loadedSettings = loadSettings()
	})
	return loadedSettings
}

// testDataDir 为每个测试进程准备一个独立的临时数据目录。
// 创建失败时退回 os.TempDir() 下的固定名字，绝不能退回仓库里的 data/，
// 否则又会让测试写到运行实例的数据目录上。
const testDataDirPrefix = "datacollect-testdata-"

var (
	testDataDirOnce sync.Once
	testDataDirPath string
)

func testDataDir() string {
	testDataDirOnce.Do(func() {
		pruneStaleTestDataDirs()

		dir, err := os.MkdirTemp("", testDataDirPrefix)
		if err != nil {
			logger.Warnf("[bootstrap] 创建测试数据目录失败，退回临时根目录: %v", err)
			dir = filepath.Join(os.TempDir(), testDataDirPrefix+"fallback")
		}
		testDataDirPath = dir
	})
	return testDataDirPath
}

// pruneStaleTestDataDirs 顺手清理上次测试遗留的隔离目录。
// 只删超过 1 小时的，避免和并发运行的其它测试互相踩踏。
func pruneStaleTestDataDirs() {
	entries, err := os.ReadDir(os.TempDir())
	if err != nil {
		return
	}
	deadline := time.Now().Add(-time.Hour)
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), testDataDirPrefix) {
			continue
		}
		info, err := entry.Info()
		if err != nil || info.ModTime().After(deadline) {
			continue
		}
		_ = os.RemoveAll(filepath.Join(os.TempDir(), entry.Name()))
	}
}

func loadSettings() Settings {
	settings := defaultSettings()

	configPath := findConfigFile()
	configDir := ""
	if configPath != "" {
		configDir = filepath.Dir(configPath)
		cfg, err := readServerConfig(configPath)
		if err != nil {
			panic(fmt.Errorf("load server config %s failed: %w", configPath, err))
		}
		applyConfigFile(&settings, cfg)
	}

	applyEnvOverrides(&settings)

	// 测试进程默认把数据目录隔离到临时目录，避免测试误写正在运行的实例的 data/。
	// go test 生成的测试二进制里 testing.Testing() 恒为 true（链接期决定），
	// go build 产出的生产二进制恒为 false，因此这不改变生产行为。
	// 需要在测试里指定目录时显式设置 GOLOOK_DATA_DIR 即可（上面已生效，这里不覆盖）。
	if testing.Testing() && strings.TrimSpace(os.Getenv("GOLOOK_DATA_DIR")) == "" {
		if dir := testDataDir(); dir != "" {
			settings.DataDir = dir
		}
	}

	if settings.DataDir == "" {
		settings.DataDir = defaultDataDirName
	}
	settings.DataDir = resolvePath(configDir, settings.DataDir)
	settings.ConfigPath = configPath
	settings.ConfigDir = configDir

	deviceSettings, deviceConfigPath, err := loadDeviceSettings()
	if err != nil {
		// 跨包测试用例无法注入 bootstrap 里未导出的 device.yaml 搜索点，
		// 测试进程缺配置时退回一份最小快照，避免每个间接依赖 LoadSettings 的用例
		// 都要自造 device.yaml。与上面的 data_dir 隔离同理，仅测试构建生效：
		// testing.Testing() 在生产二进制里恒为 false，缺配置仍会 panic。
		if !testing.Testing() {
			panic(fmt.Errorf("load device config failed: %w", err))
		}
		deviceSettings = DeviceSettings{
			Version: 1,
			Protocols: map[string][]ProtocolEndpointSettings{
				"rs485_rtu": {{Label: "COM1", Value: "/dev/ttyS1"}},
			},
		}
		deviceConfigPath = ""
	}
	settings.Device = deviceSettings
	settings.DeviceConfigPath = deviceConfigPath

	return settings
}

func defaultSettings() Settings {
	return Settings{
		ServerPort: "8080",
		DataDir:    defaultDataDirName,
		Storage: StorageSettings{
			FlushWindowMinutes:    10,
			ExtremumFlushInterval: 5 * time.Second,
		},
		Log: LogSettings{
			Level: "info",
		},
	}
}

// readServerConfig 只负责解析 YAML，不处理默认值和覆盖逻辑。
func readServerConfig(path string) (serverConfigFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return serverConfigFile{}, err
	}

	var cfg serverConfigFile
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return serverConfigFile{}, err
	}
	return cfg, nil
}

// applyConfigFile 将 server.yaml 的字段叠加到默认配置上。
func applyConfigFile(settings *Settings, cfg serverConfigFile) {
	if cfg.Server.Port != "" {
		settings.ServerPort = cfg.Server.Port
	}
	if cfg.Paths.DataDir != "" {
		settings.DataDir = cfg.Paths.DataDir
	}
	// 非正值视为缺省，回落默认 10：窗口计算对 0/负数无意义。
	if cfg.Storage.History.FlushWindowMinutes > 0 {
		settings.Storage.FlushWindowMinutes = cfg.Storage.History.FlushWindowMinutes
	}
	if cfg.Storage.History.ExtremumFlushInterval > 0 {
		settings.Storage.ExtremumFlushInterval = cfg.Storage.History.ExtremumFlushInterval
	}
	if cfg.Log.Level != "" {
		settings.Log.Level = cfg.Log.Level
	}
}

// applyEnvOverrides 让环境变量可以覆盖 server.yaml，便于容器和临时调试场景。
func applyEnvOverrides(settings *Settings) {
	if value := strings.TrimSpace(os.Getenv("GOLOOK_DATA_DIR")); value != "" {
		settings.DataDir = value
	}
	if value := strings.TrimSpace(os.Getenv("SERVER_PORT")); value != "" {
		settings.ServerPort = value
	}
	if value := strings.TrimSpace(os.Getenv("GOLOOK_LOG_LEVEL")); value != "" {
		settings.Log.Level = value
	}
}

// findConfigFile 在显式指定路径和常见工作目录中查找 server.yaml。
//
// 搜索顺序：
// 1. GOLOOK_CONFIG_FILE；
// 2. 当前工作目录及其父级；
// 3. 可执行文件所在目录及其父级。
func findConfigFile() string {
	if override := strings.TrimSpace(os.Getenv("GOLOOK_CONFIG_FILE")); override != "" {
		if !filepath.IsAbs(override) {
			if abs, err := filepath.Abs(override); err == nil {
				override = abs
			}
		}
		if fileExists(override) {
			return filepath.Clean(override)
		}
		return ""
	}

	candidates := []string{
		defaultConfigFileName, "server.yml",
		filepath.Join("config", defaultConfigFileName), filepath.Join("config", "server.yml"),
	}
	searchDirs := uniqueDirs(append(upwardDirs(mustGetwd()), upwardDirs(executableDir())...))
	for _, dir := range searchDirs {
		for _, name := range candidates {
			candidate := filepath.Join(dir, name)
			if fileExists(candidate) {
				return filepath.Clean(candidate)
			}
		}
	}
	return ""
}

// upwardDirs 返回 start 及其所有父目录，直到根目录为止。
func upwardDirs(start string) []string {
	start = strings.TrimSpace(start)
	if start == "" {
		return nil
	}

	current := filepath.Clean(start)
	seen := make([]string, 0, 8)
	for {
		seen = append(seen, current)
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	return seen
}

// uniqueDirs 去重目录列表，保持首次出现顺序。
func uniqueDirs(dirs []string) []string {
	seen := make(map[string]struct{}, len(dirs))
	out := make([]string, 0, len(dirs))
	for _, dir := range dirs {
		dir = filepath.Clean(dir)
		if _, ok := seen[dir]; ok {
			continue
		}
		seen[dir] = struct{}{}
		out = append(out, dir)
	}
	return out
}

// executableDir 返回当前可执行文件所在目录。
func executableDir() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	return filepath.Dir(exe)
}

// mustGetwd 返回当前工作目录；失败时返回空串。
func mustGetwd() string {
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	return wd
}

// fileExists 判断路径是否存在且为普通文件。
func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// resolvePath 将相对路径基于 baseDir 转成绝对/规范化路径。
func resolvePath(baseDir, p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	p = filepath.Clean(filepath.FromSlash(p))
	if filepath.IsAbs(p) || baseDir == "" {
		return p
	}
	return filepath.Clean(filepath.Join(baseDir, p))
}
