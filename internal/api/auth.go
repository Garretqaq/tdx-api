package api

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

// apiKeyAuth 校验 X-API-Key，未配置任何 key 时放行（方便本地调试）。
func apiKeyAuth(keys []string, next http.Handler) http.Handler {
	if len(keys) == 0 {
		return next
	}
	set := make(map[string]struct{}, len(keys))
	for _, k := range keys {
		set[k] = struct{}{}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := strings.TrimSpace(r.Header.Get("X-API-Key"))
		if key == "" {
			key = strings.TrimSpace(r.URL.Query().Get("api_key"))
		}
		if key == "" {
			fail(w, http.StatusUnauthorized, "缺少 API Key")
			return
		}
		if _, ok := set[key]; !ok {
			// 常量时间比较，避免时序侧信道
			for k := range set {
				if subtle.ConstantTimeCompare([]byte(k), []byte(key)) == 1 {
					next.ServeHTTP(w, r)
					return
				}
			}
			fail(w, http.StatusUnauthorized, "API Key 无效")
			return
		}
		next.ServeHTTP(w, r)
	})
}
