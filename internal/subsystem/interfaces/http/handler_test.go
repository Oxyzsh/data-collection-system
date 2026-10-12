package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestHandlerResponsesFollowGuide 校验接口响应结构与对接指南 §3.2 一致。
func TestHandlerResponsesFollowGuide(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterRoutes(r)

	// /health 必须是 {"code":0,"message":"success"}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/health", nil))
	var health map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &health); err != nil {
		t.Fatalf("/health 响应不是 JSON: %v", err)
	}
	if health["code"].(float64) != 0 || health["message"] != "success" {
		t.Fatalf("/health 响应不符合指南: %s", w.Body.String())
	}

	// realtime 缺少 pointStr → errorCode=400
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/protocol/sub/system/api/v1/converter/realtime", nil))
	var rt map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &rt)
	if rt["errorCode"].(float64) != 400 {
		t.Fatalf("缺少 pointStr 应返回 errorCode=400: %s", w.Body.String())
	}

	// realtime 非法 pointStr → errorCode=400
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/protocol/sub/system/api/v1/converter/realtime?pointStr=abc", nil))
	_ = json.Unmarshal(w.Body.Bytes(), &rt)
	if rt["errorCode"].(float64) != 400 {
		t.Fatalf("非法 pointStr 应返回 errorCode=400: %s", w.Body.String())
	}

	// set 请求体非法 → code="1"
	w = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/set/1.1", nil)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	var set map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &set)
	if set["code"] != "1" {
		t.Fatalf("非法请求体应返回 code=\"1\": %s", w.Body.String())
	}
}
