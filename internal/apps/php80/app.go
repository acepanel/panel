package php80

import (
	"github.com/gofiber/fiber/v3"
	"github.com/leonelquinteros/gotext"

	"github.com/tnborg/panel/internal/apps/php"
	"github.com/tnborg/panel/internal/biz"
)

type App struct {
	php *php.App
}

func NewApp(t *gotext.Locale, task biz.TaskRepo) *App {
	return &App{
		php: php.NewApp(t, task),
	}
}

func (s *App) Route(r fiber.Router) {
	s.php.Route(80)(r)
}
