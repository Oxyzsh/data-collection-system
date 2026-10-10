package logger

import (
	"bytes"
	"context"
	"log/slog"
	"testing"
	"time"
)

// testRecord 构造固定时间戳的记录，供各测试复用。
func testRecord(level slog.Level, msg string) slog.Record {
	return slog.NewRecord(time.Date(2026, 10, 8, 20, 46, 35, 0, time.Local), level, msg, 0)
}

// 格式渲染：时间戳 + [LEVEL] + 消息，与改造前结构一致，仅新增级别字段。
func TestHandlerFormat(t *testing.T) {
	var buf bytes.Buffer
	h := newHandler(&buf, slog.LevelDebug)
	if err := h.Handle(context.Background(), testRecord(slog.LevelInfo, "[DB] 配置库初始化成功")); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	want := "2026/10/08 20:46:35 [INFO] [DB] 配置库初始化成功\n"
	if got := buf.String(); got != want {
		t.Fatalf("输出 = %q, 期望 %q", got, want)
	}
}

// 级别标签逐级渲染。
func TestLevelName(t *testing.T) {
	cases := []struct {
		level slog.Level
		want  string
	}{
		{slog.LevelDebug, "DEBUG"},
		{slog.LevelInfo, "INFO"},
		{slog.LevelWarn, "WARN"},
		{slog.LevelError, "ERROR"},
	}
	for _, c := range cases {
		if got := levelName(c.level); got != c.want {
			t.Fatalf("levelName(%v) = %q, 期望 %q", c.level, got, c.want)
		}
	}
}

// 过滤：低于配置级别的记录不放行、不输出。
func TestHandlerFiltersBelowLevel(t *testing.T) {
	var buf bytes.Buffer
	h := newHandler(&buf, slog.LevelInfo)
	if h.Enabled(context.Background(), slog.LevelDebug) {
		t.Fatal("info 档下 debug 应被过滤")
	}
	if !h.Enabled(context.Background(), slog.LevelInfo) {
		t.Fatal("info 档下 info 应放行")
	}
	_ = h.Handle(context.Background(), testRecord(slog.LevelDebug, "心跳"))
	if buf.Len() != 0 {
		t.Fatalf("被过滤记录不得输出: %q", buf.String())
	}
}

// 尾部换行归一：消息自带 \n 时不出现双换行（存量调用普遍带尾部 \n）。
func TestHandlerTrimsTrailingNewline(t *testing.T) {
	var buf bytes.Buffer
	h := newHandler(&buf, slog.LevelDebug)
	_ = h.Handle(context.Background(), testRecord(slog.LevelInfo, "hello\n"))
	if got := buf.String(); got != "2026/10/08 20:46:35 [INFO] hello\n" {
		t.Fatalf("输出 = %q", got)
	}
}

// 消息内部换行保留（多行块/堆栈不得被破坏，仅首行带前缀）。
func TestHandlerKeepsInnerNewlines(t *testing.T) {
	var buf bytes.Buffer
	h := newHandler(&buf, slog.LevelDebug)
	_ = h.Handle(context.Background(), testRecord(slog.LevelError, "line1\nline2\n"))
	want := "2026/10/08 20:46:35 [ERROR] line1\nline2\n"
	if got := buf.String(); got != want {
		t.Fatalf("输出 = %q, 期望 %q", got, want)
	}
}

// 属性以 k=v 追加（WithAttrs 与 Record attrs 两种来源）。
func TestHandlerAttrs(t *testing.T) {
	var buf bytes.Buffer
	h := newHandler(&buf, slog.LevelDebug).WithAttrs([]slog.Attr{slog.String("module", "db")})
	r := testRecord(slog.LevelInfo, "msg")
	r.AddAttrs(slog.Int("rows", 3))
	_ = h.Handle(context.Background(), r)
	want := "2026/10/08 20:46:35 [INFO] msg module=db rows=3\n"
	if got := buf.String(); got != want {
		t.Fatalf("输出 = %q, 期望 %q", got, want)
	}
}
