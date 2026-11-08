package apploader

import (
	"reflect"
	"slices"
	"strings"
	"sync"

	"github.com/go-chi/chi/v5"

	"github.com/acepanel/panel/v3/pkg/types"
)

var apps sync.Map

var supportedSlugs = []string{
	"apache",
	"caddy",
	"clickhouse",
	"codeserver",
	"docker",
	"elasticsearch",
	"fail2ban",
	"frp",
	"gitea",
	"grafana",
	"kafka",
	"mariadb",
	"memcached",
	"minio",
	"mongodb",
	"mysql",
	"nginx",
	"openlitespeed",
	"openresty",
	"opensearch",
	"percona",
	"pgadmin",
	"phpmyadmin",
	"podman",
	"postgresql",
	"prometheus",
	"pureftpd",
	"redis",
	"rocketmq",
	"rsync",
	"s3fs",
	"stalwart",
	"supervisor",
	"valkey",
}

type Loader struct{}

func (r *Loader) Add(app ...types.App) {
	for item := range slices.Values(app) {
		slug := getSlug(item)
		apps.Store(slug, item)
	}
}

func (r *Loader) Register(mux chi.Router) {
	apps.Range(func(key, value any) bool {
		slug, ok1 := key.(string)
		app, ok2 := value.(types.App)
		if ok1 && ok2 {
			mux.Route("/"+slug, app.Route)
		}
		return true
	})
}

func (r *Loader) Get(slug string) (types.App, bool) {
	v, ok := apps.Load(slug)
	if !ok {
		return nil, false
	}
	app, ok := v.(types.App)
	return app, ok
}

func Slugs() []string {
	var slugs []string
	apps.Range(func(key, value any) bool {
		if slug, ok := key.(string); ok {
			slugs = append(slugs, slug)
		}
		return true
	})
	return slugs
}

// SupportedSlugs 返回当前版本内置的应用标识，无需构造应用实例。
func SupportedSlugs() []string {
	return slices.Clone(supportedSlugs)
}

func getSlug(app types.App) string {
	if app == nil {
		return ""
	}

	t := reflect.TypeOf(app)
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	pkgPath := t.PkgPath()
	if pkgPath == "" {
		return ""
	}

	parts := strings.Split(pkgPath, "/")
	return parts[len(parts)-1]
}
