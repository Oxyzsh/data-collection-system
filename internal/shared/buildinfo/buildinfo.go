package buildinfo

import (
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
)

// VersionFileName 是二进制同目录下的版本号文件名。
// 由构建脚本写入，供不含编译注入的老二进制兜底。
const VersionFileName = "__version"

// Version 由构建脚本通过
// `-ldflags "-X data-collection-system/internal/shared/buildinfo.Version=<版本号>"`
// 注入。它是版本号的**首选来源**：升级只覆盖二进制、不携带 __version 文件时，
// 版本号仍能正确显示。
var Version string

var (
	once    sync.Once
	version string
)

// executablePath 返回当前可执行文件路径，抽成变量便于测试注入。
var executablePath = os.Executable

// Load 读取后端版本号。
//
// 版本来源优先级：
// 1. 编译期注入值（-ldflags -X）
// 2. 二进制同目录下的 __version 文件
// 3. Go build info
// 4. 默认值 1.0.0
func Load() string {
	once.Do(func() {
		version = strings.TrimSpace(Version)
		if version == "" {
			version = readVersionFile()
		}
		if version == "" {
			if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
				version = info.Main.Version
			}
		}
		if version == "" {
			version = "1.0.0"
		}
	})
	return version
}

func readVersionFile() string {
	exe, err := executablePath()
	if err != nil {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(filepath.Dir(exe), VersionFileName))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}
