package i18n

import "testing"

func TestTranslate(t *testing.T) {
	if got := Translate(LocaleENUS, KeyCommonInvalidParams, nil); got != "Invalid parameters" {
		t.Fatalf("Translate english mismatch: %q", got)
	}
	if got := Translate(LocaleZHCN, KeyCommonInvalidParams, nil); got != "参数错误" {
		t.Fatalf("Translate chinese mismatch: %q", got)
	}
}

func TestLocalizeAlias(t *testing.T) {
	if got := Localize(LocaleENUS, "未授权", nil); got != "Unauthorized" {
		t.Fatalf("Localize alias mismatch: %q", got)
	}
	if got := Localize(LocaleENUS, "system.time_sync_success", nil); got != "Time synced successfully" {
		t.Fatalf("Localize key mismatch: %q", got)
	}
}

func TestLocalizePrefixMessage(t *testing.T) {
	got := Localize(LocaleENUS, "请求参数错误: field required", nil)
	if got != "Invalid request parameters: field required" {
		t.Fatalf("Localize prefix mismatch: %q", got)
	}
}

func TestTimeSyncMessageKeys(t *testing.T) {
	cases := []struct {
		key string
		en  string
	}{
		{"system.ntp_saved", "NTP server saved"},
		{"system.hwclock_write_failed", "Failed to write hardware clock; time may be lost after power off"},
	}
	for _, c := range cases {
		if got := Localize(LocaleENUS, c.key, nil); got != c.en {
			t.Errorf("Localize(%q) = %q, want %q", c.key, got, c.en)
		}
		// 中文必须存在且不是回退成 key 本身
		if got := Localize(LocaleZHCN, c.key, nil); got == c.key {
			t.Errorf("Localize(%q) 中文缺失，回退成了 key", c.key)
		}
	}
}
