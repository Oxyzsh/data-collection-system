package bootstrap

import "os"

// EnsureRuntimeDirectories 负责在首次启动时创建运行时数据目录。
// 只创建采集进程实际使用的目录，避免在磁盘上留下无用路径。
func EnsureRuntimeDirectories(_ Settings) error {
	dirs := []string{
		DataDir(),
		DataPath("backend"),
		DataPath("backend", "db"),
		DataPath("backend", "template"),
		DataPath("backend", "mib"),
		DataPath("tmp"),
		DataPath("uploads"),
	}

	seen := map[string]struct{}{}
	for _, dir := range dirs {
		if dir == "" {
			continue
		}
		if _, ok := seen[dir]; ok {
			continue
		}
		seen[dir] = struct{}{}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return nil
}
