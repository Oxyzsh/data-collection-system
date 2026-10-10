package logger

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
)

// parseLevel：合法值（含大小写与别名）。
func TestParseLevelValid(t *testing.T) {
	cases := []struct {
		in   string
		want slog.Level
	}{
		{"", slog.LevelInfo},
		{"info", slog.LevelInfo},
		{"INFO", slog.LevelInfo},
		{" debug ", slog.LevelDebug},
		{"Warn", slog.LevelWarn},
		{"warning", slog.LevelWarn},
		{"error", slog.LevelError},
	}
	for _, c := range cases {
		lv, ok := parseLevel(c.in)
		if !ok || lv != c.want {
			t.Fatalf("parseLevel(%q) = (%v,%v), 期望 (%v,true)", c.in, lv, ok, c.want)
		}
	}
}

// parseLevel：非法值 ok=false（由 Setup 警告并回落 info）。
func TestParseLevelInvalid(t *testing.T) {
	if _, ok := parseLevel("verbose"); ok {
		t.Fatal("非法级别应返回 ok=false")
	}
}

// probeStringer 记录格式化是否被求值。
type probeStringer struct{ called bool }

func (p *probeStringer) String() string { p.called = true; return "probe" }

// 预检：被过滤级别不得执行格式化（采集高频路径零开销）。
func TestLogfSkipsFormatWhenFiltered(t *testing.T) {
	defer installTestLoggerTo(t, io.Discard, slog.LevelInfo)()
	probe := &probeStringer{}
	Debugf("x=%s", probe)
	if probe.called {
		t.Fatal("被过滤的日志不应执行格式化")
	}
}

// 放行级别的输出经统一格式（前缀 + 级别标签 + 消息）。
func TestLogfWritesFormatted(t *testing.T) {
	var buf bytes.Buffer
	defer installTestLoggerTo(t, &buf, slog.LevelInfo)()
	Infof("[DB] 配置库初始化成功: %s", "x.db")
	if got := buf.String(); !strings.Contains(got, "[INFO] [DB] 配置库初始化成功: x.db") {
		t.Fatalf("输出 = %q", got)
	}
}

// Setup 非法值：回落 info（不 panic、不可静默——警告走当前档位）。
func TestSetupInvalidLevelFallsBackToInfo(t *testing.T) {
	defer installTestLoggerTo(t, io.Discard, slog.LevelInfo)()
	Setup("bogus")
	if !slog.Default().Enabled(context.Background(), slog.LevelInfo) {
		t.Fatal("非法级别应回落 info（info 应放行）")
	}
	if slog.Default().Enabled(context.Background(), slog.LevelDebug) {
		t.Fatal("回落 info 后 debug 应被过滤")
	}
}

// Setup 生效：warn 档下 info 被过滤、warn 放行。
func TestSetupAppliesLevel(t *testing.T) {
	defer installTestLoggerTo(t, io.Discard, slog.LevelInfo)()
	Setup("warn")
	if slog.Default().Enabled(context.Background(), slog.LevelInfo) {
		t.Fatal("warn 档下 info 应被过滤")
	}
	if !slog.Default().Enabled(context.Background(), slog.LevelWarn) {
		t.Fatal("warn 档下 warn 应放行")
	}
}

// 标准库 log 兜底：接管后输出统一格式、按 INFO 处理。
func TestStdLogWriterUnifiesFormat(t *testing.T) {
	var buf bytes.Buffer
	defer installTestLoggerTo(t, &buf, slog.LevelInfo)()
	if _, err := (stdLogWriter{}).Write([]byte("[bootstrap] 旧调用兜底\n")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if got := buf.String(); !strings.Contains(got, "[INFO] [bootstrap] 旧调用兜底") {
		t.Fatalf("输出 = %q", got)
	}
}

// installTestLoggerTo 安装测试用全局 logger 并返回恢复函数（defer 使用）。
func installTestLoggerTo(t *testing.T, w io.Writer, lv slog.Level) func() {
	t.Helper()
	old := slog.Default()
	slog.SetDefault(slog.New(newHandler(w, lv)))
	return func() { slog.SetDefault(old) }
}
