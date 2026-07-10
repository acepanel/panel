package rsync

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/chix/v2"
	"github.com/libtnb/utils/str"
	"github.com/samber/do/v2"

	"github.com/acepanel/panel/v3/internal/service"
	"github.com/acepanel/panel/v3/pkg/io"
	"github.com/acepanel/panel/v3/pkg/shell"
	"github.com/acepanel/panel/v3/pkg/systemctl"
	"github.com/acepanel/panel/v3/pkg/types"
)

type App struct {
	t *gotext.Locale
}

func NewApp(i do.Injector) (*App, error) {
	t := do.MustInvoke[*gotext.Locale](i)
	return &App{
		t: t,
	}, nil
}

func (s *App) Route(r chi.Router) {
	r.Get("/modules", s.List)
	r.Post("/modules", s.Create)
	r.Post("/modules/{name}", s.Update)
	r.Delete("/modules/{name}", s.Delete)
	r.Get("/config", s.GetConfig)
	r.Post("/config", s.UpdateConfig)
}

func (s *App) Status() string {
	ok, _ := systemctl.Status("rsyncd")
	return types.AggregateAppStatus(ok)
}

func (s *App) List(w http.ResponseWriter, r *http.Request) {
	config, err := io.Read("/etc/rsyncd.conf")
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
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
						service.Error(w, http.StatusInternalServerError, s.t.Get("failed to get the secret key for module %s", currentModule.AuthUser))
						return
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

	paged, total := service.Paginate(r, modules)

	service.Success(w, chix.M{
		"total": total,
		"items": paged,
	})
}

func (s *App) Create(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[Create](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	config, err := io.Read("/etc/rsyncd.conf")
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	if strings.Contains(config, "["+req.Name+"]") {
		service.Error(w, http.StatusUnprocessableEntity, s.t.Get("module %s already exists", req.Name))
		return
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
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	if err = io.WriteAppend("/etc/rsyncd.secrets", fmt.Sprintf("%s:%s\n", req.AuthUser, req.Secret), 0600); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	if err = systemctl.Restart("rsyncd"); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	service.Success(w, nil)
}

func (s *App) Delete(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[Delete](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	config, err := io.Read("/etc/rsyncd.conf")
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	if !strings.Contains(config, "["+req.Name+"]") {
		service.Error(w, http.StatusUnprocessableEntity, s.t.Get("module %s does not exist", req.Name))
		return
	}

	module := str.Cut(config, "# "+req.Name+"-START", "# "+req.Name+"-END")
	config = strings.ReplaceAll(config, "\n# "+req.Name+"-START"+module+"# "+req.Name+"-END", "")

	match := regexp.MustCompile(`auth users = ([^\n]+)`).FindStringSubmatch(module)
	if len(match) == 2 {
		authUser := match[1]
		if _, err = shell.Execf(`sed -i '/^%s:.*$/d' /etc/rsyncd.secrets`, authUser); err != nil {
			service.Error(w, http.StatusInternalServerError, "%v", err)
			return
		}
	}

	if err = io.Write("/etc/rsyncd.conf", config, 0644); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	if err = systemctl.Restart("rsyncd"); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	service.Success(w, nil)
}

func (s *App) Update(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[Update](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	config, err := io.Read("/etc/rsyncd.conf")
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	if !strings.Contains(config, "["+req.Name+"]") {
		service.Error(w, http.StatusUnprocessableEntity, s.t.Get("module %s does not exist", req.Name))
		return
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
			service.Error(w, http.StatusInternalServerError, "%v", err)
			return
		}
	}

	if err = io.Write("/etc/rsyncd.conf", config, 0644); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	if err = io.WriteAppend("/etc/rsyncd.secrets", fmt.Sprintf("%s:%s\n", req.AuthUser, req.Secret), 0600); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	if err = systemctl.Restart("rsyncd"); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	service.Success(w, nil)
}

func (s *App) GetConfig(w http.ResponseWriter, r *http.Request) {
	config, err := io.Read("/etc/rsyncd.conf")
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	service.Success(w, config)
}

func (s *App) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[UpdateConfig](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = io.Write("/etc/rsyncd.conf", req.Config, 0644); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	if err = systemctl.Restart("rsyncd"); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	service.Success(w, nil)
}
