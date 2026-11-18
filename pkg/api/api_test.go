package api

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/libtnb/assert/check"
	"github.com/libtnb/assert/must"
)

func newTestAPI() *API {
	return NewAPI("3.0.0", "en")
}

func TestGetDecodesData(t *testing.T) {
	var query url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/version/intermediate" {
			http.NotFound(w, r)
			return
		}
		query = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"message":"success","data":[{"version":"3.0.1","type":"stable"}]}`))
	}))
	defer srv.Close()
	api := NewAPI("3.0.0", "en", srv.URL)

	versions, err := api.IntermediateVersions("stable")
	must.NoError(t, err)
	check.DeepEqual(t, *versions, Versions{{Version: "3.0.1", Type: "stable"}})
	check.Equal(t, query.Get("start"), "3.0.0")
	check.Equal(t, query.Get("channel"), "stable")
	check.Equal(t, query.Get("locale"), "en")

	_, err = api.Apps()
	check.Error(t, err)
}

func TestGetLatestVersion(t *testing.T) {
	version, err := newTestAPI().LatestVersion("stable")
	must.NoError(t, err)
	check.NotEmpty(t, version.Version)
}

func TestGetIntermediateVersions(t *testing.T) {
	// 已是最新版时列表为空是正常的，只能校验不报错
	_, err := newTestAPI().IntermediateVersions("stable")
	check.NoError(t, err)
}

func TestGetVersions(t *testing.T) {
	versions, err := newTestAPI().Versions("stable")
	must.NoError(t, err)
	check.NotEmpty(t, *versions)
}

func TestGetCategories(t *testing.T) {
	categories, err := newTestAPI().Categories()
	must.NoError(t, err)
	check.NotEmpty(t, *categories)
}

func TestGetApps(t *testing.T) {
	apps, err := newTestAPI().Apps()
	must.NoError(t, err)
	check.NotEmpty(t, *apps)
}

func TestAppCallback(t *testing.T) {
	check.NoError(t, newTestAPI().AppCallback("nginx"))
}

func TestGetTemplates(t *testing.T) {
	templates, err := newTestAPI().Templates()
	must.NoError(t, err)
	check.NotEmpty(t, *templates)
}

func TestTemplateCallback(t *testing.T) {
	check.NoError(t, newTestAPI().TemplateCallback("nginx"))
}
