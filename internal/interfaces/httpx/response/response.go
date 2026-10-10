package response

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"data-collection-system/internal/interfaces/httpx/locale"
	"data-collection-system/internal/shared/i18n"
)

func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, data)
}

func Created(c *gin.Context, data interface{}) {
	c.JSON(http.StatusCreated, data)
}

func BadRequest(c *gin.Context, message string) {
	c.JSON(http.StatusBadRequest, gin.H{"error": text(c, message, nil)})
}

func Unauthorized(c *gin.Context, message string) {
	c.JSON(http.StatusUnauthorized, gin.H{"error": text(c, message, nil)})
}

func Forbidden(c *gin.Context, message string) {
	c.JSON(http.StatusForbidden, gin.H{"error": text(c, message, nil)})
}

func NotFound(c *gin.Context, message string) {
	c.JSON(http.StatusNotFound, gin.H{"error": text(c, message, nil)})
}

func InternalError(c *gin.Context, message string) {
	c.JSON(http.StatusInternalServerError, gin.H{"error": text(c, message, nil)})
}

func Message(c *gin.Context, message string) {
	c.JSON(http.StatusOK, gin.H{"message": text(c, message, nil)})
}

func Text(c *gin.Context, keyOrMessage string) string {
	return text(c, keyOrMessage, nil)
}

func TextWithArgs(c *gin.Context, keyOrMessage string, args map[string]interface{}) string {
	return text(c, keyOrMessage, args)
}

func API(c *gin.Context, status, code int, data interface{}, keyOrMessage string) {
	APIWithArgs(c, status, code, data, keyOrMessage, nil)
}

func APIWithArgs(c *gin.Context, status, code int, data interface{}, keyOrMessage string, args map[string]interface{}) {
	payload := gin.H{"code": code}
	if data != nil {
		payload["data"] = data
	}
	if keyOrMessage != "" {
		payload["message"] = text(c, keyOrMessage, args)
	}
	c.JSON(status, payload)
}

func text(c *gin.Context, keyOrMessage string, args map[string]interface{}) string {
	return i18n.Localize(locale.Get(c), keyOrMessage, args)
}
