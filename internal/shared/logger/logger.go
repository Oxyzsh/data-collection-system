// Package logger 提供进程统一的日志设施：级别过滤、统一格式渲染
// （保持改造前 `时间 [模块] 消息` 结构并新增 [LEVEL] 字段）、格式化包装。
//
// 以标准库 log/slog 为引擎；业务代码经本包 Debugf/Infof/Warnf/Errorf/Fatal
// 输出，不要直接使用标准库 log（标准库输出会被 init 接管兜底，但级别统一为 INFO）。
package logger

import (
	"context"
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"strings"
)

// parseLevel 解析配置级别字符串（大小写不敏感）；空串按 info；
// 非法值 ok=false（由 Setup 警告并回落 info）。
func parseLevel(s string) (slog.Level, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "info":
		return slog.LevelInfo, true
	case "debug":
		return slog.LevelDebug, true
	case "warn", "warning":
		return slog.LevelWarn, true
	case "error":
		return slog.LevelError, true
	default:
		return slog.LevelInfo, false
	}
}

// Setup 按配置级别重新装配全局 logger（main 在 LoadSettings 后调用，重启生效）。
// 非法值：警告（经当前档位输出）并回落 info——不 panic、不静默。
func Setup(level string) {
	lv, ok := parseLevel(level)
	if !ok {
		Infof("[Logger] 非法日志级别 %q，回落 info", level)
		lv = slog.LevelInfo
	}
	setup(os.Stderr, lv)
}

// setup 是 Setup 的可注入实现（测试传入 bytes.Buffer）。
func setup(w io.Writer, lv slog.Level) {
	slog.SetDefault(slog.New(newHandler(w, lv)))
}

func init() {
	// 包加载即安装默认 handler（INFO + 统一格式）：保证任何早期日志
	// （main 装载配置前、各包 init 期）也使用统一格式与级别过滤。
	setup(os.Stderr, slog.LevelInfo)
	// 接管标准库 log：第三方库/残留输出统一格式（视为 INFO，随全局级别过滤）；
	// SetFlags(0) 清掉标准库默认的时间前缀，避免与 handler 的时间戳重复。
	log.SetOutput(stdLogWriter{})
	log.SetFlags(0)
}

// stdLogWriter 将标准库 log 的输出转入 slog（INFO 级）。
type stdLogWriter struct{}

func (stdLogWriter) Write(p []byte) (int, error) {
	slog.Info(strings.TrimRight(string(p), "\n"))
	return len(p), nil
}

// logf 各级别包装的公共实现：先做级别预检，被过滤时不执行 Sprintf
// （采集链路高频调用，避免无效格式化开销）。
func logf(level slog.Level, format string, args ...any) {
	ctx := context.Background()
	if !slog.Default().Enabled(ctx, level) {
		return
	}
	slog.Log(ctx, level, fmt.Sprintf(format, args...))
}

// Debugf 调试细节：周期采集流水、访问日志等默认不输出的内容。
func Debugf(format string, args ...any) { logf(slog.LevelDebug, format, args...) }

// Infof 常规信息：启动、生命周期、状态变更、统计。
func Infof(format string, args ...any) { logf(slog.LevelInfo, format, args...) }

// Warnf 可恢复异常/降级：重试、跳过、水位预警。
func Warnf(format string, args ...any) { logf(slog.LevelWarn, format, args...) }

// Errorf 失败与错误（非致命）。
func Errorf(format string, args ...any) { logf(slog.LevelError, format, args...) }

// Fatal 致命错误：输出后以退出码 1 终止进程（替代标准库 log.Fatal 的
// 启动拒绝路径）。不做级别预检——致命错误在任何配置下都必须可见。
func Fatal(format string, args ...any) {
	slog.Error(fmt.Sprintf(format, args...))
	os.Exit(1)
}
