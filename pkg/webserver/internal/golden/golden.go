// Package golden 四个方言的整文件快照测试共用的助手
package golden

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/libtnb/assert/must"

	"github.com/acepanel/panel/v3/pkg/webserver/types"
)

// snapshot 把配置目录下的全部文件按路径顺序拼成一份文本，临时目录前缀换成占位符
func snapshot(t *testing.T, configDir, root string) string {
	t.Helper()
	var b strings.Builder
	fsys := os.DirFS(configDir)
	err := fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		content, err := fs.ReadFile(fsys, path)
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintf(&b, "==> %s <==\n%s", path, content)
		if !strings.HasSuffix(string(content), "\n") {
			b.WriteString("\n\\ No newline at end of file\n")
		}
		return nil
	})
	must.NoError(t, err)

	return strings.ReplaceAll(b.String(), root, "{root}")
}

// assert 与 testdata/<name>.golden 逐字节比较，设置 UPDATE_GOLDEN 时改写基准
func assert(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name+".golden")
	if os.Getenv("UPDATE_GOLDEN") != "" {
		must.NoError(t, os.MkdirAll("testdata", 0755))
		must.NoError(t, os.WriteFile(path, []byte(got), 0644))
		return
	}
	want, err := os.ReadFile(path)
	must.NoError(t, err)
	must.DeepEqual(t, got, string(want))
}

// reapply 模拟打开站点设置后原样保存：读到的值逐项写回再保存。证书路径不是每个方言都回读，由调用方给
func reapply(t *testing.T, v types.Vhost, cert, key string) {
	t.Helper()
	must.NoError(t, v.SetListen(v.Listen()))
	must.NoError(t, v.SetServerName(v.ServerName()))
	must.NoError(t, v.SetIndex(v.Index()))
	must.NoError(t, v.SetRoot(v.Root()))
	if ssl := v.SSLConfig(); ssl != nil {
		ssl.Cert, ssl.Key = cert, key
		must.NoError(t, v.SetSSLConfig(ssl))
	}
	if php, ok := v.(types.VhostPHP); ok {
		must.NoError(t, php.SetPHP(php.PHP()))
	}
	if proxy, ok := v.(types.VhostProxy); ok {
		must.NoError(t, proxy.SetUpstreams(proxy.Upstreams()))
		must.NoError(t, proxy.SetProxies(proxy.Proxies()))
	}
	if redirect, ok := v.(types.VhostRedirect); ok {
		must.NoError(t, redirect.SetRedirects(redirect.Redirects()))
	}
	must.NoError(t, v.SetAccessLog(v.AccessLog()))
	must.NoError(t, v.SetErrorLog(v.ErrorLog()))
	if limit := v.RateLimit(); limit != nil {
		must.NoError(t, v.SetRateLimit(limit))
	}
	if realIP := v.RealIP(); realIP != nil {
		must.NoError(t, v.SetRealIP(realIP))
	}
	must.NoError(t, v.SetBasicAuth(v.BasicAuth()))
	must.NoError(t, v.Save())
}
