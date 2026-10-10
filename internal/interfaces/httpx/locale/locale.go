package locale

import (
	"github.com/gin-gonic/gin"

	"data-collection-system/internal/shared/i18n"
)

const contextKey = "request_locale"

func Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		resolved := i18n.ResolveLocale(c.GetHeader("Accept-Language"), c.Query("lang"))
		c.Set(contextKey, resolved)
		c.Header("Content-Language", resolved)
		c.Next()
	}
}

func Get(c *gin.Context) string {
	if c == nil {
		return i18n.LocaleZHCN
	}
	if value, ok := c.Get(contextKey); ok {
		if locale, ok := value.(string); ok && locale != "" {
			return locale
		}
	}
	return i18n.LocaleZHCN
}

func IsEnglish(c *gin.Context) bool {
	return i18n.IsEnglishLocale(Get(c))
}

func Set(c *gin.Context, preferred string) {
	if c == nil {
		return
	}
	normalized := i18n.NormalizeLocale(preferred)
	if normalized == "" {
		return
	}
	c.Set(contextKey, normalized)
	c.Header("Content-Language", normalized)
}
