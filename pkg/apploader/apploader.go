package apploader

import (
	"reflect"
	"slices"
	"strings"
	"sync"

	"github.com/gofiber/fiber/v3"

	"github.com/tnborg/panel/pkg/types"
)

var apps sync.Map

type Loader struct{}

func (r *Loader) Add(app ...types.App) {
	for item := range slices.Values(app) {
		slug := getSlug(item)
		apps.Store(slug, item)
	}
}

func (r *Loader) Register(router fiber.Router) {
	apps.Range(func(key, value any) bool {
		appInstance := value.(types.App)
		appGroup := router.Group("/" + key.(string))
		appInstance.Route(appGroup)
		return true
	})
}

func Slugs() []string {
	var slugs []string
	apps.Range(func(key, value any) bool {
		slugs = append(slugs, key.(string))
		return true
	})
	return slugs
}

func getSlug(app types.App) string {
	if app == nil {
		return ""
	}

	t := reflect.TypeOf(app)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	pkgPath := t.PkgPath()
	if pkgPath == "" {
		return ""
	}

	parts := strings.Split(pkgPath, "/")
	return parts[len(parts)-1]
}
