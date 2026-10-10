package i18n

import "strings"

const (
	LocaleZHCN = "zh-CN"
	LocaleENUS = "en-US"
)

func ResolveLocale(acceptLanguage, legacyLang string) string {
	if locale := parseAcceptLanguage(acceptLanguage); locale != "" {
		return locale
	}
	if locale := parseLegacyLang(legacyLang); locale != "" {
		return locale
	}
	return LocaleZHCN
}

func NormalizeLocale(raw string) string {
	return normalizeLocaleToken(raw)
}

func IsEnglishLocale(locale string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(locale)), "en")
}

func parseAcceptLanguage(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return ""
	}
	parts := strings.Split(raw, ",")
	for _, part := range parts {
		token := strings.TrimSpace(part)
		if token == "" {
			continue
		}
		if idx := strings.Index(token, ";"); idx >= 0 {
			token = strings.TrimSpace(token[:idx])
		}
		if locale := normalizeLocaleToken(token); locale != "" {
			return locale
		}
	}
	return ""
}

func parseLegacyLang(raw string) string {
	switch strings.TrimSpace(raw) {
	case "2", "en", "en-US", "en_US":
		return LocaleENUS
	case "1", "zh", "zh-CN", "zh_CN":
		return LocaleZHCN
	default:
		return ""
	}
}

func normalizeLocaleToken(raw string) string {
	s := strings.ToLower(strings.TrimSpace(strings.ReplaceAll(raw, "_", "-")))
	switch {
	case s == "":
		return ""
	case strings.HasPrefix(s, "en"):
		return LocaleENUS
	case strings.HasPrefix(s, "zh"):
		return LocaleZHCN
	default:
		return ""
	}
}
