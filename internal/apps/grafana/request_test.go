package grafana

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/libtnb/assert/check"
	"github.com/libtnb/assert/must"

	"github.com/acepanel/panel/v3/internal/service"
)

func TestDataSourceNameNotOverriddenByPath(t *testing.T) {
	var got *DataSource
	var bindErr error
	router := chi.NewRouter()
	router.Post("/datasources/{name}", func(_ http.ResponseWriter, r *http.Request) {
		got, bindErr = service.Bind[DataSource](r)
	})

	body := `{"name":"renamed","type":"prometheus","url":"http://127.0.0.1:9090"}`
	req := httptest.NewRequest(http.MethodPost, "/datasources/original", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(httptest.NewRecorder(), req)

	must.NoError(t, bindErr)
	// 路径里的是旧名，请求体里的新名不能被它盖掉，否则重命名永远不生效
	check.Equal(t, got.Name, "renamed")
}
