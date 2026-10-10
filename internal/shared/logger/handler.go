package logger

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"sync"
)

// 统一日志格式（时间戳 + 模块标签 + 级别）：
//
//	2026/10/08 20:46:35 [INFO] [DB] 配置库初始化成功: /opt/data-collect/data/backend/db/config.db
//
// 并发说明：所有派生 handler（WithAttrs）共享同一个 lockedWriter，
// 保证多 goroutine 输出不逐行交错。
type handler struct {
	w     *lockedWriter
	level slog.Level
	attrs []slog.Attr // WithAttrs 累积；当前调用面不使用，为将来直用 slog 结构化调用保留
}

// lockedWriter 串行化对底层 Writer 的写入（stdout/stderr 的单次写入不保证原子）。
type lockedWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (lw *lockedWriter) Write(p []byte) (int, error) {
	lw.mu.Lock()
	defer lw.mu.Unlock()
	return lw.w.Write(p)
}

func newHandler(w io.Writer, level slog.Level) *handler {
	return &handler{w: &lockedWriter{w: w}, level: level}
}

// Enabled 实现级别过滤：低于配置级别的记录直接丢弃（高频路径零格式化开销）。
func (h *handler) Enabled(_ context.Context, l slog.Level) bool {
	return l >= h.level
}

// Handle 渲染一条日志。只归一尾部换行（保证以单个 \n 结尾），消息内部换行
// （panic 堆栈、多行块输出）保留原样，仅首行带前缀。
//
// slog.Logger 在调用 Handle 前会先做 Enabled 检查；此处再自查一次级别，
// 保证绕过 Logger 直接调用 Handle 时同样遵守过滤。
func (h *handler) Handle(_ context.Context, r slog.Record) error {
	if r.Level < h.level {
		return nil
	}
	var b strings.Builder
	b.WriteString(r.Time.Format("2006/01/02 15:04:05"))
	b.WriteString(" [")
	b.WriteString(levelName(r.Level))
	b.WriteString("] ")
	b.WriteString(strings.TrimRight(r.Message, "\n"))
	for _, a := range h.attrs {
		writeAttr(&b, a)
	}
	r.Attrs(func(a slog.Attr) bool {
		writeAttr(&b, a)
		return true
	})
	b.WriteByte('\n')
	_, err := io.WriteString(h.w, b.String())
	return err
}

// WithAttrs 返回携带属性的新 handler（共享底层 lockedWriter，写不交错）。
func (h *handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &handler{w: h.w, level: h.level, attrs: append(append([]slog.Attr{}, h.attrs...), attrs...)}
}

// WithGroup 项目当前不使用分组；保持最小实现直接返回自身。
func (h *handler) WithGroup(string) slog.Handler { return h }

// writeAttr 以 ` k=v` 形式追加属性（文本可读优先）。
func writeAttr(b *strings.Builder, a slog.Attr) {
	b.WriteString(" ")
	b.WriteString(a.Key)
	b.WriteString("=")
	b.WriteString(a.Value.String())
}

// levelName 渲染大写级别标签。
func levelName(l slog.Level) string {
	switch {
	case l < slog.LevelInfo:
		return "DEBUG"
	case l < slog.LevelWarn:
		return "INFO"
	case l < slog.LevelError:
		return "WARN"
	default:
		return "ERROR"
	}
}
