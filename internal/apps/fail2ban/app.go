package fail2ban

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/chix"
	"github.com/libtnb/utils/str"
	"github.com/spf13/cast"

	"github.com/tnborg/panel/internal/app"
	"github.com/tnborg/panel/internal/biz"
	"github.com/tnborg/panel/internal/service"
	"github.com/tnborg/panel/pkg/io"
	"github.com/tnborg/panel/pkg/shell"
)

type App struct {
	t           *gotext.Locale
	websiteRepo biz.WebsiteRepo
}

func NewApp(t *gotext.Locale, website biz.WebsiteRepo) *App {
	return &App{
		t:           t,
		websiteRepo: website,
	}
}

func (s *App) Route(r fiber.Router) {
	r.Get("/jails", s.List)
	r.Post("/jails", s.Create)
	r.Delete("/jails", s.Delete)
	r.Get("/jails/{name}", s.BanList)
	r.Post("/unban", s.Unban)
	r.Post("/white_list", s.SetWhiteList)
	r.Get("/white_list", s.GetWhiteList)
}

// List 所有规则
func (s *App) List(c fiber.Ctx) error {
	raw, err := io.Read("/etc/fail2ban/jail.local")
	if err != nil {
		return service.Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	jailList := regexp.MustCompile(`\[(.*?)]`).FindAllStringSubmatch(raw, -1)

	jails := make([]Jail, 0)
	for i, jail := range jailList {
		if i == 0 {
			continue
		}

		jailName := jail[1]
		jailRaw := str.Cut(raw, "# "+jailName+"-START", "# "+jailName+"-END")
		if len(jailRaw) == 0 {
			continue
		}
		jailEnabled := strings.Contains(jailRaw, "enabled = true")
		jailMaxRetry := regexp.MustCompile(`maxretry = (.*)`).FindStringSubmatch(jailRaw)
		jailFindTime := regexp.MustCompile(`findtime = (.*)`).FindStringSubmatch(jailRaw)
		jailBanTime := regexp.MustCompile(`bantime = (.*)`).FindStringSubmatch(jailRaw)

		jails = append(jails, Jail{
			Name:     jailName,
			Enabled:  jailEnabled,
			MaxRetry: cast.ToInt(jailMaxRetry[1]),
			FindTime: cast.ToInt(jailFindTime[1]),
			BanTime:  cast.ToInt(jailBanTime[1]),
		})
	}

	paged, total := service.Paginate(c, jails)

	return service.Success(c, chix.M{
		"total": total,
		"items": paged,
	})
}

// Create 添加规则
func (s *App) Create(c fiber.Ctx) error {
	req, err := service.Bind[Add](c)
	if err != nil {
		return service.Error(c, http.StatusUnprocessableEntity, "%v", err)
	}
	jailName := req.Name
	jailType := req.Type
	jailMaxRetry := cast.ToString(req.MaxRetry)
	jailFindTime := cast.ToString(req.FindTime)
	jailBanTime := cast.ToString(req.BanTime)
	jailWebsiteName := req.WebsiteName
	jailWebsiteMode := req.WebsiteMode
	jailWebsitePath := req.WebsitePath

	raw, err := io.Read("/etc/fail2ban/jail.local")
	if err != nil {
		return service.Error(c, http.StatusUnprocessableEntity, "%v", err)
	}
	if (strings.Contains(raw, "["+jailName+"]") && jailType == "service") || (strings.Contains(raw, "["+jailWebsiteName+"]"+"-cc") && jailType == "website" && jailWebsiteMode == "cc") || (strings.Contains(raw, "["+jailWebsiteName+"]"+"-path") && jailType == "website" && jailWebsiteMode == "path") {
		return service.Error(c, http.StatusUnprocessableEntity, s.t.Get("rule already exists"))
	}

	switch jailType {
	case "website":
		website, err := s.websiteRepo.GetByName(jailWebsiteName)
		if err != nil {
			return service.Error(c, http.StatusUnprocessableEntity, "%v", err)
		}
		var ports string
		for _, listen := range website.Listens {
			if port, err := cast.ToIntE(listen.Address); err == nil {
				ports += fmt.Sprintf("%d", port) + ","
			}
		}
		ports = strings.TrimSuffix(ports, ",")

		rule := `
# ` + jailWebsiteName + `-` + jailWebsiteMode + `-START
[` + jailWebsiteName + `-` + jailWebsiteMode + `]
enabled = true
filter = haozi-` + jailWebsiteName + `-` + jailWebsiteMode + `
port = ` + ports + `
maxretry = ` + jailMaxRetry + `
findtime = ` + jailFindTime + `
bantime = ` + jailBanTime + `
logpath = ` + app.Root + `/wwwlogs/` + website.Name + `.log
# ` + jailWebsiteName + `-` + jailWebsiteMode + `-END
`
		raw += rule
		if err = io.Write("/etc/fail2ban/jail.local", raw, 0644); err != nil {
			return service.Error(c, http.StatusInternalServerError, "%v", err)
		}

		var filter string
		if jailWebsiteMode == "cc" {
			filter = `
[Definition]
failregex = ^<HOST>\s-.*HTTP/.*$
ignoreregex =
`
		} else {
			filter = `
[Definition]
failregex = ^<HOST>\s-.*\s` + jailWebsitePath + `.*HTTP/.*$
ignoreregex =
`
		}
		if err = io.Write("/etc/fail2ban/filter.d/haozi-"+jailWebsiteName+"-"+jailWebsiteMode+".conf", filter, 0644); err != nil {
			return service.Error(c, http.StatusInternalServerError, "%v", err)
		}

	case "service":
		var filter string
		var port string
		var err error
		switch jailName {
		case "ssh":
			filter = "sshd"
			port, err = shell.Execf("cat /etc/ssh/sshd_config | grep 'Port ' | awk '{print $2}'")
		case "mysql":
			filter = "mysqld-auth"
			port, err = shell.Execf("cat %s/server/mysql/conf/my.cnf | grep 'port' | head -n 1 | awk '{print $3}'", app.Root)
		case "pure-ftpd":
			filter = "pure-ftpd"
			port, err = shell.Execf(`cat %s/server/pure-ftpd/etc/pure-ftpd.conf | grep "Bind" | awk '{print $2}' | awk -F "," '{print $2}'`, app.Root)
		default:
			return service.Error(c, http.StatusUnprocessableEntity, s.t.Get("unknown service"))
		}
		if len(port) == 0 || err != nil {
			return service.Error(c, http.StatusUnprocessableEntity, s.t.Get("get service port failed, please check if it is installed"))
		}

		rule := `
# ` + jailName + `-START
[` + jailName + `]
enabled = true
filter = ` + filter + `
port = ` + port + `
maxretry = ` + jailMaxRetry + `
findtime = ` + jailFindTime + `
bantime = ` + jailBanTime + `
# ` + jailName + `-END
`
		raw += rule
		if err := io.Write("/etc/fail2ban/jail.local", raw, 0644); err != nil {
			return service.Error(c, http.StatusInternalServerError, "%v", err)
		}
	}

	if _, err = shell.Execf("fail2ban-client reload"); err != nil {
		return service.Error(c, http.StatusInternalServerError, "%v", err)
	}

	return service.Success(c, nil)
}

// Delete 删除规则
func (s *App) Delete(c fiber.Ctx) error {
	req, err := service.Bind[Delete](c)
	if err != nil {
		return service.Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	raw, err := io.Read("/etc/fail2ban/jail.local")
	if err != nil {
		return service.Error(c, http.StatusUnprocessableEntity, "%v", err)
	}
	if !strings.Contains(raw, "["+req.Name+"]") {
		return service.Error(c, http.StatusUnprocessableEntity, s.t.Get("rule not found"))
	}

	rule := str.Cut(raw, "# "+req.Name+"-START", "# "+req.Name+"-END")
	raw = strings.ReplaceAll(raw, "\n# "+req.Name+"-START"+rule+"# "+req.Name+"-END", "")
	raw = strings.TrimSpace(raw)
	if err := io.Write("/etc/fail2ban/jail.local", raw, 0644); err != nil {
		return service.Error(c, http.StatusInternalServerError, "%v", err)
	}

	if _, err := shell.Execf("fail2ban-client reload"); err != nil {
		return service.Error(c, http.StatusInternalServerError, "%v", err)
	}

	return service.Success(c, nil)
}

// BanList 获取封禁列表
func (s *App) BanList(c fiber.Ctx) error {
	req, err := service.Bind[BanList](c)
	if err != nil {
		return service.Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	currentlyBan, err := shell.Execf(`fail2ban-client status %s | grep "Currently banned" | awk '{print $4}'`, req.Name)
	if err != nil {
		return service.Error(c, http.StatusInternalServerError, s.t.Get("failed to get current banned list"))
	}
	totalBan, err := shell.Execf(`fail2ban-client status %s | grep "Total banned" | awk '{print $4}'`, req.Name)
	if err != nil {
		return service.Error(c, http.StatusInternalServerError, s.t.Get("failed to get total banned list"))
	}
	bannedIp, err := shell.Execf(`fail2ban-client status %s | grep "Banned IP list" | awk -F ":" '{print $2}'`, req.Name)
	if err != nil {
		return service.Error(c, http.StatusInternalServerError, s.t.Get("failed to get banned ip list"))
	}
	bannedIpList := strings.Split(bannedIp, " ")

	var list []map[string]string
	for _, ip := range bannedIpList {
		if len(ip) > 0 {
			list = append(list, map[string]string{
				"name": req.Name,
				"ip":   ip,
			})
		}
	}
	if list == nil {
		list = []map[string]string{}
	}

	return service.Success(c, chix.M{
		"currently_ban": currentlyBan,
		"total_ban":     totalBan,
		"baned_list":    list,
	})
}

// Unban 解封
func (s *App) Unban(c fiber.Ctx) error {
	req, err := service.Bind[Unban](c)
	if err != nil {
		return service.Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if _, err = shell.Execf("fail2ban-client set %s unbanip %s", req.Name, req.IP); err != nil {
		return service.Error(c, http.StatusInternalServerError, "%v", err)
	}

	return service.Success(c, nil)
}

// SetWhiteList 设置白名单
func (s *App) SetWhiteList(c fiber.Ctx) error {
	req, err := service.Bind[SetWhiteList](c)
	if err != nil {
		return service.Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	raw, err := io.Read("/etc/fail2ban/jail.local")
	if err != nil {
		return service.Error(c, http.StatusUnprocessableEntity, "%v", err)
	}
	// 正则替换
	reg := regexp.MustCompile(`ignoreip\s*=\s*.*\n`)
	if reg.MatchString(raw) {
		raw = reg.ReplaceAllString(raw, "ignoreip = "+req.IP+"\n")
	} else {
		return service.Error(c, http.StatusInternalServerError, s.t.Get("failed to parse the ignoreip of fail2ban"))
	}

	if err = io.Write("/etc/fail2ban/jail.local", raw, 0644); err != nil {
		return service.Error(c, http.StatusInternalServerError, "%v", err)
	}

	if _, err = shell.Execf("fail2ban-client reload"); err != nil {
		return service.Error(c, http.StatusInternalServerError, "%v", err)
	}
	return service.Success(c, nil)
}

// GetWhiteList 获取白名单
func (s *App) GetWhiteList(c fiber.Ctx) error {
	raw, err := io.Read("/etc/fail2ban/jail.local")
	if err != nil {
		return service.Error(c, http.StatusUnprocessableEntity, "%v", err)
	}
	reg := regexp.MustCompile(`ignoreip\s*=\s*(.*)\n`)
	if reg.MatchString(raw) {
		ignoreIp := reg.FindStringSubmatch(raw)[1]
		return service.Success(c, ignoreIp)
	} else {
		return service.Error(c, http.StatusInternalServerError, s.t.Get("failed to parse the ignoreip of fail2ban"))
	}
}
