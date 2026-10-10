package locale

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"data-collection-system/internal/shared/i18n"
)

type captureWriter struct {
	gin.ResponseWriter
	body bytes.Buffer
}

func (w *captureWriter) Write(data []byte) (int, error) {
	_, _ = w.body.Write(data)
	return len(data), nil
}

func (w *captureWriter) WriteString(s string) (int, error) {
	_, _ = w.body.WriteString(s)
	return len(s), nil
}

func ResponseMessageLocalizer() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !shouldLocalizeResponse(c.Request) {
			c.Next()
			return
		}

		cw := &captureWriter{ResponseWriter: c.Writer}
		c.Writer = cw
		c.Next()

		if cw.body.Len() == 0 {
			return
		}

		body := cw.body.Bytes()
		localizedBody, ok := localizeJSONBody(Get(c), body)
		if !ok {
			_, _ = cw.ResponseWriter.Write(body)
			return
		}

		cw.ResponseWriter.Header().Del("Content-Length")
		cw.ResponseWriter.Header().Set("Content-Length", strconvItoa(len(localizedBody)))
		_, _ = cw.ResponseWriter.Write(localizedBody)
	}
}

func shouldLocalizeResponse(r *http.Request) bool {
	if r == nil || r.URL == nil {
		return false
	}
	path := r.URL.Path
	if !strings.HasPrefix(path, "/api/") {
		return false
	}
	if strings.Contains(path, "/export") {
		return false
	}
	return true
}

func localizeJSONBody(locale string, body []byte) ([]byte, bool) {
	var payload interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, false
	}
	changed := localizePayload(locale, payload)
	if !changed {
		return nil, false
	}
	out, err := json.Marshal(payload)
	if err != nil {
		return nil, false
	}
	return out, true
}

func localizePayload(locale string, payload interface{}) bool {
	root, ok := payload.(map[string]interface{})
	if !ok {
		return false
	}
	return localizeMap(locale, root)
}

func localizeMap(locale string, m map[string]interface{}) bool {
	changed := false
	for key, value := range m {
		if localized, ok := localizeStringValue(locale, key, value); ok {
			m[key] = localized
			changed = true
			continue
		}
		switch typed := value.(type) {
		case map[string]interface{}:
			if localizeMap(locale, typed) {
				changed = true
			}
		case []interface{}:
			if localizeSlice(locale, key, typed) {
				changed = true
			}
		}
	}
	return changed
}

func localizeSlice(locale, key string, arr []interface{}) bool {
	changed := false
	for i := range arr {
		if localized, ok := localizeStringValue(locale, key, arr[i]); ok {
			arr[i] = localized
			changed = true
			continue
		}
		if child, ok := arr[i].(map[string]interface{}); ok {
			if localizeMap(locale, child) {
				changed = true
			}
		}
	}
	return changed
}

func localizeStringValue(locale, key string, raw interface{}) (string, bool) {
	s, ok := raw.(string)
	if !ok || strings.TrimSpace(s) == "" {
		return "", false
	}
	if !shouldLocalizeKey(key) {
		return "", false
	}

	localized := i18n.Localize(locale, s, nil)
	if localized == s {
		return "", false
	}
	return localized, true
}

func shouldLocalizeKey(key string) bool {
	switch key {
	case "message", "error", "warning", "check_rslt":
		return true
	case "warnings":
		return true
	default:
		return false
	}
}

func strconvItoa(v int) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
