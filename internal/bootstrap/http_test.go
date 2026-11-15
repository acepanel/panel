package bootstrap

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/libtnb/assert/check"
)

func TestSPAHandlerCacheControl(t *testing.T) {
	const immutable = "public, max-age=31536000, immutable"

	handler := newPrecompressedSPAHandler(http.FS(fstest.MapFS{
		"index.html.br":               {Data: []byte("index")},
		"assets/index-Bk19xnH-.js.br": {Data: []byte("js")},
		"assets/font-DF1abBS2.woff2":  {Data: []byte("font")},
		"favicon.ico":                 {Data: []byte("icon")},
	}))

	tests := []struct {
		name string
		path string
		want string
	}{
		{name: "带哈希的压缩产物", path: "/assets/index-Bk19xnH-.js", want: immutable},
		{name: "带哈希的未压缩产物", path: "/assets/font-DF1abBS2.woff2", want: immutable},
		{name: "入口页", path: "/", want: "no-cache"},
		{name: "前端路由回落入口页", path: "/website/list", want: "no-cache"},
		{name: "已不存在的旧产物回落入口页", path: "/assets/index-OLDHASH0.js", want: "no-cache"},
		{name: "其他静态文件", path: "/favicon.ico"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			req.Header.Set("Accept-Encoding", "br")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			check.Equal(t, rec.Code, http.StatusOK)
			check.Equal(t, rec.Header().Get("Cache-Control"), tt.want)
		})
	}
}
