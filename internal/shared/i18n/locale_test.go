package i18n

import "testing"

func TestResolveLocale(t *testing.T) {
	tests := []struct {
		name           string
		acceptLanguage string
		legacyLang     string
		want           string
	}{
		{name: "accept english wins", acceptLanguage: "en-US,en;q=0.9", legacyLang: "1", want: LocaleENUS},
		{name: "accept chinese", acceptLanguage: "zh-CN,zh;q=0.8", legacyLang: "2", want: LocaleZHCN},
		{name: "legacy english", acceptLanguage: "", legacyLang: "2", want: LocaleENUS},
		{name: "legacy chinese", acceptLanguage: "", legacyLang: "1", want: LocaleZHCN},
		{name: "fallback default", acceptLanguage: "", legacyLang: "", want: LocaleZHCN},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ResolveLocale(tc.acceptLanguage, tc.legacyLang)
			if got != tc.want {
				t.Fatalf("ResolveLocale() = %q, want %q", got, tc.want)
			}
		})
	}
}
