package pureftpd

import (
	"net/http"
	"regexp"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/chix"
	"github.com/spf13/cast"

	"github.com/tnborg/panel/internal/app"
	"github.com/tnborg/panel/internal/service"
	"github.com/tnborg/panel/pkg/firewall"
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
	r.Get("/users", s.List)
	r.Post("/users", s.Create)
	r.Delete("/users/{username}", s.Delete)
	r.Post("/users/{username}/password", s.ChangePassword)
	r.Get("/port", s.GetPort)
	r.Post("/port", s.UpdatePort)
}

// List 获取用户列表
func (s *App) List(c fiber.Ctx) error {
	listRaw, err := shell.Execf("pure-pw list")
	if err != nil {
		return service.Success(c, chix.M{
			"total": 0,
			"items": []User{},
		})
	}

	listArr := strings.Split(listRaw, "\n")
	var users []User
	for _, v := range listArr {
		if len(v) == 0 {
			continue
		}

		match := regexp.MustCompile(`(\S+)\s+(\S+)`).FindStringSubmatch(v)
		users = append(users, User{
			Username: match[1],
			Path:     strings.Replace(match[2], "/./", "/", 1),
		})
	}

	paged, total := service.Paginate(c, users)

	return service.Success(c, chix.M{
		"total": total,
		"items": paged,
	})
}

// Create 创建用户
func (s *App) Create(c fiber.Ctx) error {
	req, err := service.Bind[Create](c)
	if err != nil {
		return service.Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if !strings.HasPrefix(req.Path, "/") {
		req.Path = "/" + req.Path
	}
	if !io.Exists(req.Path) {
		return service.Error(c, http.StatusUnprocessableEntity, s.t.Get("directory %s does not exist", req.Path))
	}

	if _, err = shell.Execf(`yes '%s' | pure-pw useradd '%s' -u www -g www -d '%s'`, req.Password, req.Username, req.Path); err != nil {
		return service.Error(c, http.StatusInternalServerError, "%v", err)
	}
	if _, err = shell.Execf("pure-pw mkdb"); err != nil {
		return service.Error(c, http.StatusInternalServerError, "%v", err)
	}

	return service.Success(c, nil)
}

// Delete 删除用户
func (s *App) Delete(c fiber.Ctx) error {
	req, err := service.Bind[Delete](c)
	if err != nil {
		return service.Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if _, err = shell.Execf("pure-pw userdel '%s' -m", req.Username); err != nil {
		return service.Error(c, http.StatusInternalServerError, "%v", err)
	}
	if _, err = shell.Execf("pure-pw mkdb"); err != nil {
		return service.Error(c, http.StatusInternalServerError, "%v", err)
	}

	return service.Success(c, nil)
}

// ChangePassword 修改密码
func (s *App) ChangePassword(c fiber.Ctx) error {
	req, err := service.Bind[ChangePassword](c)
	if err != nil {
		return service.Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if _, err = shell.Execf(`yes '%s' | pure-pw passwd '%s' -m`, req.Password, req.Username); err != nil {
		return service.Error(c, http.StatusInternalServerError, "%v", err)
	}
	if _, err = shell.Execf("pure-pw mkdb"); err != nil {
		return service.Error(c, http.StatusInternalServerError, "%v", err)
	}

	return service.Success(c, nil)
}

// GetPort 获取端口
func (s *App) GetPort(c fiber.Ctx) error {
	port, err := shell.Execf(`cat %s/server/pure-ftpd/etc/pure-ftpd.conf | grep "Bind" | awk '{print $2}' | awk -F "," '{print $2}'`, app.Root)
	if err != nil {
		return service.Error(c, http.StatusInternalServerError, s.t.Get("failed to get port: %v", err))
	}

	return service.Success(c, cast.ToInt(port))
}

// UpdatePort 设置端口
func (s *App) UpdatePort(c fiber.Ctx) error {
	req, err := service.Bind[UpdatePort](c)
	if err != nil {
		return service.Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if _, err = shell.Execf(`sed -i "s/Bind.*/Bind 0.0.0.0,%d/g" %s/server/pure-ftpd/etc/pure-ftpd.conf`, req.Port, app.Root); err != nil {
		return service.Error(c, http.StatusInternalServerError, "%v", err)
	}

	fw := firewall.NewFirewall()
	err = fw.Port(firewall.FireInfo{
		Type:      firewall.TypeNormal,
		PortStart: req.Port,
		PortEnd:   req.Port,
		Direction: firewall.DirectionIn,
		Strategy:  firewall.StrategyAccept,
	}, firewall.OperationAdd)
	if err != nil {
		return service.Error(c, http.StatusInternalServerError, "%v", err)
	}

	if err = systemctl.Restart("pure-ftpd"); err != nil {
		return service.Error(c, http.StatusInternalServerError, "%v", err)
	}

	return service.Success(c, nil)
}
