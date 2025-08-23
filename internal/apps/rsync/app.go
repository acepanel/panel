package rsync

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/chix"
	"github.com/libtnb/utils/str"

	"github.com/tnborg/panel/internal/service"
	"github.com/tnborg/panel/pkg/io"
	"github.com/tnborg/panel/pkg/shell"
	"github.com/tnborg/panel/pkg/systemctl"
)

type App struct {
	t *gotext.Locale
}

func NewApp(t *gotext.Locale) *App {
	return &App{
		t: t,
	}
}

func (s *App) Route(r fiber.Router) {
	r.Get("/modules", s.List)
	r.Post("/modules", s.Create)
	r.Post("/modules/{name}", s.Update)
	r.Delete("/modules/{name}", s.Delete)
	r.Get("/config", s.GetConfig)
	r.Post("/config", s.UpdateConfig)
}

func (s *App) List(c fiber.Ctx) error {
	config, err := io.Read("/etc/rsyncd.conf")
	if err != nil {
		return service.Error(c, http.StatusInternalServerError, "%v", err)
	}

	var modules []Module
	lines := strings.Split(config, "\n")
	var currentModule *Module

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			if currentModule != nil {
				modules = append(modules, *currentModule)
			}
			moduleName := line[1 : len(line)-1]
			currentModule = &Module{
				Name: moduleName,
			}
		} else if currentModule != nil {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])

				switch key {
				case "path":
					currentModule.Path = value
				case "comment":
					currentModule.Comment = value
				case "read only":
					currentModule.ReadOnly = value == "yes" || value == "true"
				case "auth users":
					currentModule.AuthUser = value
					currentModule.Secret, err = shell.Execf(`grep -E '^%s:.*$' /etc/rsyncd.secrets | awk -F ':' '{print $2}'`, currentModule.AuthUser)
					if err != nil {
						return service.Error(c, http.StatusInternalServerError, s.t.Get("failed to get the secret key for module %s", currentModule.AuthUser))
					}
				case "hosts allow":
					currentModule.HostsAllow = value
				}
			}
		}
	}

	if currentModule != nil {
		modules = append(modules, *currentModule)
	}

	paged, total := service.Paginate(c, modules)

	return service.Success(c, chix.M{
		"total": total,
		"items": paged,
	})
}

func (s *App) Create(c fiber.Ctx) error {
	req, err := service.Bind[Create](c)
	if err != nil {
		return service.Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	config, err := io.Read("/etc/rsyncd.conf")
	if err != nil {
		return service.Error(c, http.StatusInternalServerError, "%v", err)
	}
	if strings.Contains(config, "["+req.Name+"]") {
		return service.Error(c, http.StatusUnprocessableEntity, s.t.Get("module %s already exists", req.Name))
	}

	conf := `# ` + req.Name + `-START
[` + req.Name + `]
path = ` + req.Path + `
comment = ` + req.Comment + `
read only = no
auth users = ` + req.AuthUser + `
hosts allow = ` + req.HostsAllow + `
secrets file = /etc/rsyncd.secrets
# ` + req.Name + `-END
`

	if err = io.WriteAppend("/etc/rsyncd.conf", conf, 0644); err != nil {
		return service.Error(c, http.StatusInternalServerError, "%v", err)
	}
	if err = io.WriteAppend("/etc/rsyncd.secrets", fmt.Sprintf(`%s:%s\n`, req.AuthUser, req.Secret), 0600); err != nil {
		return service.Error(c, http.StatusInternalServerError, "%v", err)
	}

	if err = systemctl.Restart("rsyncd"); err != nil {
		return service.Error(c, http.StatusInternalServerError, "%v", err)
	}

	return service.Success(c, nil)
}

func (s *App) Delete(c fiber.Ctx) error {
	req, err := service.Bind[Delete](c)
	if err != nil {
		return service.Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	config, err := io.Read("/etc/rsyncd.conf")
	if err != nil {
		return service.Error(c, http.StatusInternalServerError, "%v", err)
	}
	if !strings.Contains(config, "["+req.Name+"]") {
		return service.Error(c, http.StatusUnprocessableEntity, s.t.Get("module %s does not exist", req.Name))
	}

	module := str.Cut(config, "# "+req.Name+"-START", "# "+req.Name+"-END")
	config = strings.ReplaceAll(config, "\n# "+req.Name+"-START"+module+"# "+req.Name+"-END", "")

	match := regexp.MustCompile(`auth users = ([^\n]+)`).FindStringSubmatch(module)
	if len(match) == 2 {
		authUser := match[1]
		if _, err = shell.Execf(`sed -i '/^%s:.*$/d' /etc/rsyncd.secrets`, authUser); err != nil {
			return service.Error(c, http.StatusInternalServerError, "%v", err)
		}
	}

	if err = io.Write("/etc/rsyncd.conf", config, 0644); err != nil {
		return service.Error(c, http.StatusInternalServerError, "%v", err)
	}

	if err = systemctl.Restart("rsyncd"); err != nil {
		return service.Error(c, http.StatusInternalServerError, "%v", err)
	}

	return service.Success(c, nil)
}

func (s *App) Update(c fiber.Ctx) error {
	req, err := service.Bind[Update](c)
	if err != nil {
		return service.Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	config, err := io.Read("/etc/rsyncd.conf")
	if err != nil {
		return service.Error(c, http.StatusInternalServerError, "%v", err)
	}
	if !strings.Contains(config, "["+req.Name+"]") {
		return service.Error(c, http.StatusUnprocessableEntity, s.t.Get("module %s does not exist", req.Name))
	}

	newConf := `# ` + req.Name + `-START
[` + req.Name + `]
path = ` + req.Path + `
comment = ` + req.Comment + `
read only = no
auth users = ` + req.AuthUser + `
hosts allow = ` + req.HostsAllow + `
secrets file = /etc/rsyncd.secrets
# ` + req.Name + `-END`

	module := str.Cut(config, "# "+req.Name+"-START", "# "+req.Name+"-END")
	config = strings.ReplaceAll(config, "# "+req.Name+"-START"+module+"# "+req.Name+"-END", newConf)

	match := regexp.MustCompile(`auth users = ([^\n]+)`).FindStringSubmatch(module)
	if len(match) == 2 {
		authUser := match[1]
		if _, err = shell.Execf(`sed -i '/^%s:.*$/d' /etc/rsyncd.secrets`, authUser); err != nil {
			return service.Error(c, http.StatusInternalServerError, "%v", err)
		}
	}

	if err = io.Write("/etc/rsyncd.conf", config, 0644); err != nil {
		return service.Error(c, http.StatusInternalServerError, "%v", err)
	}
	if err = io.WriteAppend("/etc/rsyncd.secrets", fmt.Sprintf(`%s:%s\n`, req.AuthUser, req.Secret), 0600); err != nil {
		return service.Error(c, http.StatusInternalServerError, "%v", err)
	}

	if err = systemctl.Restart("rsyncd"); err != nil {
		return service.Error(c, http.StatusInternalServerError, "%v", err)
	}

	return service.Success(c, nil)
}

func (s *App) GetConfig(c fiber.Ctx) error {
	config, err := io.Read("/etc/rsyncd.conf")
	if err != nil {
		return service.Error(c, http.StatusInternalServerError, "%v", err)
	}

	return service.Success(c, config)
}

func (s *App) UpdateConfig(c fiber.Ctx) error {
	req, err := service.Bind[UpdateConfig](c)
	if err != nil {
		return service.Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if err = io.Write("/etc/rsyncd.conf", req.Config, 0644); err != nil {
		return service.Error(c, http.StatusInternalServerError, "%v", err)
	}

	if err = systemctl.Restart("rsyncd"); err != nil {
		return service.Error(c, http.StatusInternalServerError, "%v", err)
	}

	return service.Success(c, nil)
}
