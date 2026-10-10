package buildinfo

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// reset 清空包级缓存，让下一次 Load 重新走完整优先级链。
// 测试同包内可直接访问未导出变量。
func reset(t *testing.T) {
	t.Helper()
	once = sync.Once{}
	version = ""
	t.Cleanup(func() {
		once = sync.Once{}
		version = ""
	})
}

// fakeExecutable 把可执行文件路径指向临时目录下的假文件。
func fakeExecutable(t *testing.T, dir string) {
	t.Helper()
	original := executablePath
	executablePath = func() (string, error) { return filepath.Join(dir, "data-collect"), nil }
	t.Cleanup(func() { executablePath = original })
}

func TestLoadPrefersInjectedVersion(t *testing.T) {
	reset(t)
	dir := t.TempDir()
	fakeExecutable(t, dir)
	if err := os.WriteFile(filepath.Join(dir, VersionFileName), []byte("file-version\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	Version = "injected-version"
	t.Cleanup(func() { Version = "" })

	if got := Load(); got != "injected-version" {
		t.Fatalf("Load() = %q, want %q", got, "injected-version")
	}
}

func TestLoadFallsBackToVersionFile(t *testing.T) {
	reset(t)
	dir := t.TempDir()
	fakeExecutable(t, dir)
	if err := os.WriteFile(filepath.Join(dir, VersionFileName), []byte("  file-version  \n"), 0o644); err != nil {
		t.Fatal(err)
	}
	Version = ""
	t.Cleanup(func() { Version = "" })

	if got := Load(); got != "file-version" {
		t.Fatalf("Load() = %q, want %q", got, "file-version")
	}
}

func TestLoadFallsBackToDefault(t *testing.T) {
	reset(t)
	dir := t.TempDir()
	fakeExecutable(t, dir)
	Version = ""
	t.Cleanup(func() { Version = "" })

	if got := Load(); got != "1.0.0" {
		t.Fatalf("Load() = %q, want %q", got, "1.0.0")
	}
}
