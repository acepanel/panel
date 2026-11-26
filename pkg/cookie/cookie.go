// Package cookie 读写仅供服务端使用的 Cookie
package cookie

import (
	"net/http"
	"time"
)

// Get 取 Cookie 的值，不存在时返回空串
func Get(r *http.Request, name string) string {
	c, err := r.Cookie(name)
	if err != nil {
		return ""
	}

	return c.Value
}

// Set 写入脚本不可读、不随跨站请求发送的 Cookie
func Set(w http.ResponseWriter, r *http.Request, name, value string, ttl time.Duration) {
	// 面板可以运行在明文 HTTP 上，Secure 只能跟随当前连接
	//nolint:gosec
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		MaxAge:   int(ttl.Seconds()),
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteLaxMode,
	})
}

// Clear 让浏览器删除 Cookie
func Clear(w http.ResponseWriter, r *http.Request, name string) {
	Set(w, r, name, "", -time.Second)
}
