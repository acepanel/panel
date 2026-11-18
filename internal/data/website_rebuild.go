package data

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/samber/lo"
	"github.com/spf13/cast"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/pkg/io"
	"github.com/acepanel/panel/v3/pkg/shell"
	"github.com/acepanel/panel/v3/pkg/types"
	"github.com/acepanel/panel/v3/pkg/webserver"
	webservertypes "github.com/acepanel/panel/v3/pkg/webserver/types"
)

// configBackup 重建会动到配置目录、库字段与 .user.ini，三者都留底才能整体回滚
type configBackup struct {
	website       biz.Website
	configDir     string
	backupDir     string
	userIni       string
	userIniData   []byte
	userIniExists bool
	openBasedir   bool
}

func (r *websiteRepo) SwitchType(ctx context.Context, req *request.WebsiteSwitchType) (*biz.Website, error) {
	website := new(biz.Website)
	if err := r.db.Where("id", req.ID).First(website).Error; err != nil {
		return nil, err
	}

	targetType := biz.WebsiteType(req.Type)
	if targetType == website.Type {
		return nil, errors.New(r.t.Get("website type is unchanged"))
	}

	setting, err := r.Get(req.ID)
	if err != nil {
		return nil, err
	}
	d, err := r.dialect()
	if err != nil {
		return nil, err
	}

	update := r.carryOverUpdate(setting)
	update.Listens = setting.Listens
	update.Index = []string{"index.html"}
	update.StatEnabled = setting.StatEnabled
	update.AccessLog = setting.AccessLog
	update.ErrorLog = setting.ErrorLog
	update.RateLimit = setting.RateLimit
	update.RealIP = setting.RealIP
	update.CustomConfigs = lo.Map(setting.CustomConfigs, func(config types.WebsiteCustomConfig, _ int) request.WebsiteCustomConfig {
		return request.WebsiteCustomConfig{
			Name:    config.Name,
			Scope:   config.Scope,
			Content: config.Content,
		}
	})
	switch targetType {
	case biz.WebsiteTypePHP:
		update.Index = []string{"index.php", "index.html"}
		update.PHP = req.PHP
		update.OpenBasedir = true
	case biz.WebsiteTypeProxy:
		update.Proxies = []webservertypes.Proxy{{
			Location: "^~ /",
			Pass:     req.Proxy,
			Host:     webservertypes.DefaultProxyHost(req.Proxy, setting.Upstreams),
		}}
	default:
		// 静态站点沿用默认配置
	}
	// 整站反代是反代站点的类型本身：切入时只留新的根规则，切出时丢弃；静态与 PHP 互切时子路径反代随通用设置保留
	if website.Type != biz.WebsiteTypeProxy && targetType != biz.WebsiteTypeProxy {
		update.Proxies = setting.Proxies
	}

	backup, err := r.backupConfig(website, setting, "switch")
	if err != nil {
		return nil, err
	}
	// 回滚需还原备份的配置目录，被取消会把站点配置留在中间态
	restoreCtx := context.WithoutCancel(ctx)

	oldType := website.Type
	website.Type = targetType
	if err = r.rebuildVhost(ctx, d, website, update); err != nil {
		return nil, r.restoreConfig(restoreCtx, backup, err)
	}

	if oldType == biz.WebsiteTypePHP && targetType != biz.WebsiteTypePHP && setting.OpenBasedir {
		if err = io.Remove(backup.userIni); err != nil {
			return nil, r.restoreConfig(restoreCtx, backup, err)
		}
	}
	if err = r.ReloadWebServer(ctx); err != nil {
		err = r.restoreConfig(restoreCtx, backup, err)
		return nil, errors.Join(err, r.ReloadWebServer(restoreCtx))
	}

	_ = io.Remove(backup.backupDir)
	return website, nil
}

func (r *websiteRepo) ResetConfig(ctx context.Context, id uint) error {
	website := new(biz.Website)
	if err := r.db.Where("id", id).First(&website).Error; err != nil {
		return err
	}

	d, err := r.dialect()
	if err != nil {
		return err
	}

	setting, err := r.Get(id)
	if err != nil {
		return err
	}
	listens := lo.Filter(setting.Listens, func(listen webservertypes.Listen, _ int) bool {
		if slices.Contains(listen.Args, "ssl") || slices.Contains(listen.Args, "quic") {
			return false
		}
		port := listen.Address
		if strings.Contains(listen.Address, ":") {
			if _, parsedPort, splitErr := net.SplitHostPort(listen.Address); splitErr == nil {
				port = parsedPort
			}
		}
		return !website.SSL || port != "443"
	})
	if len(listens) == 0 {
		listens = []webservertypes.Listen{{Address: "80"}}
	}

	update := &request.WebsiteUpdate{
		ID:          website.ID,
		Listens:     listens,
		Domains:     setting.Domains,
		Path:        website.Path,
		Root:        setting.Root,
		Index:       []string{"index.html"},
		SSL:         false,
		StatEnabled: d.Features().Stat,
		AccessLog:   filepath.Join(app.Root, "sites", website.Name, "log", "access.log"),
		ErrorLog:    filepath.Join(app.Root, "sites", website.Name, "log", "error.log"),
	}
	switch website.Type {
	case biz.WebsiteTypePHP:
		update.Index = []string{"index.php", "index.html"}
		update.PHP = setting.PHP
		update.OpenBasedir = true
	case biz.WebsiteTypeProxy:
		update.Upstreams = setting.Upstreams
		if len(setting.Proxies) > 0 && setting.Proxies[0].Pass != "" {
			update.Proxies = []webservertypes.Proxy{{
				Location: "^~ /",
				Pass:     setting.Proxies[0].Pass,
				Host:     webservertypes.DefaultProxyHost(setting.Proxies[0].Pass, setting.Upstreams),
			}}
		}
	default:
	}

	// 配置目录是删掉重建的，中途取消会让站点配置凭空消失且无处可恢复，这段不可取消
	resetCtx := context.WithoutCancel(ctx)
	backup, err := r.backupConfig(website, setting, "reset")
	if err != nil {
		return err
	}
	if website.Type == biz.WebsiteTypePHP {
		if err = io.Remove(backup.userIni); err != nil {
			return r.restoreConfig(resetCtx, backup, err)
		}
	}

	website.Status = true
	if err = r.rebuildVhost(resetCtx, d, website, update); err != nil {
		return r.restoreConfig(resetCtx, backup, err)
	}
	if err = io.ChmodR(resetCtx, backup.configDir, 0600); err != nil {
		return r.restoreConfig(resetCtx, backup, err)
	}
	_ = io.Remove(backup.backupDir)

	return r.ReloadWebServer(resetCtx)
}

// Rebuild 用上一个 Web 服务器留下的配置为当前 Web 服务器重建站点配置，返回是否重建及需人工处理的提示
func (r *websiteRepo) Rebuild(ctx context.Context, website *biz.Website) (bool, []string, error) {
	d, err := r.dialect()
	if err != nil {
		return false, nil, err
	}
	configDir := filepath.Join(app.Root, "sites", website.Name, "config")
	source, ok := r.sourceDialect(configDir)
	if !ok || source.Type == d.Type {
		return false, nil, nil
	}

	oldVhost, err := newVhost(source, website)
	if err != nil {
		return false, nil, err
	}
	setting, err := r.loadSetting(website, oldVhost)
	if err != nil {
		return false, nil, err
	}

	// 目标不支持的 IPv6 监听直接丢弃
	listens := setting.Listens
	if !d.Features().IPv6Listen {
		listens = lo.Filter(listens, func(listen webservertypes.Listen, _ int) bool {
			return !strings.HasPrefix(listen.Address, "[")
		})
	}
	if len(listens) == 0 {
		listens = []webservertypes.Listen{{Address: "80"}}
	}
	customProxy := false
	for i := range setting.Proxies {
		customProxy = customProxy || setting.Proxies[i].Custom != ""
		setting.Proxies[i].Custom = ""
	}
	index := setting.Index
	if len(index) == 0 {
		index = []string{"index.html"}
		if website.Type == biz.WebsiteTypePHP {
			index = []string{"index.php", "index.html"}
		}
	}
	update := r.carryOverUpdate(setting)
	update.Listens = listens
	update.Index = index
	update.PHP = setting.PHP
	update.OpenBasedir = setting.OpenBasedir
	update.Proxies = setting.Proxies
	update.StatEnabled = d.Features().Stat

	// 只迁移各服务器通用的设置，伪静态、自定义配置（含代理规则的自定义指令）、限流与真实 IP 是各自的语法或语义，直接丢弃
	var notes []string
	if setting.Rewrite != "" || len(setting.CustomConfigs) > 0 || customProxy || setting.RateLimit != nil || setting.RealIP != nil {
		notes = append(notes, r.t.Get("rewrite rules, custom configs, rate limit and real IP settings are not migrated"))
	}

	backup, err := r.backupConfig(website, setting, "rebuild")
	if err != nil {
		return false, nil, err
	}
	if err = r.rebuildVhost(ctx, d, website, update); err != nil {
		return false, nil, r.restoreConfig(context.WithoutCancel(ctx), backup, err)
	}

	_ = io.Remove(backup.backupDir)
	return true, notes, nil
}

// RebuildPHPMyAdmin phpMyAdmin 的站点不在库里，沿用原端口为当前 Web 服务器重建，返回是否重建
func (r *websiteRepo) RebuildPHPMyAdmin() (bool, error) {
	d, err := r.dialect()
	if err != nil {
		return false, err
	}
	configDir := filepath.Join(app.Root, "sites", "phpmyadmin", "config")
	source, ok := r.sourceDialect(configDir)
	if !ok || source.Type == d.Type {
		return false, nil
	}

	// 没有 PHP 时生成的站点会把源码当静态文件输出
	php := r.latestPHP()
	if php == 0 {
		return false, errors.New(r.t.Get("PHP is not installed"))
	}
	oldVhost, err := source.NewPHPVhost(configDir)
	if err != nil {
		return false, err
	}
	listens := lo.Map(r.listenPorts(oldVhost.Listen()), func(port string, _ int) webservertypes.Listen {
		return webservertypes.Listen{Address: port}
	})

	if err = io.Remove(configDir); err != nil {
		return false, err
	}
	if err = r.makeConfigDirs(configDir); err != nil {
		return false, err
	}
	vhost, err := d.NewPHPVhost(configDir)
	if err != nil {
		return false, err
	}
	// 站点没有域名，靠默认站点标志接住端口上的全部请求，路径与安装脚本一致
	logFile := filepath.Join(filepath.Dir(configDir), "log", "phpmyadmin.log")
	if err = errors.Join(
		vhost.SetListen(listens),
		vhost.SetDefault(true),
		vhost.SetRoot(filepath.Join(app.Root, "server", "phpmyadmin")),
		vhost.SetIndex([]string{"index.php"}),
		vhost.SetPHP(php),
		vhost.SetAccessLog(logFile),
		vhost.SetErrorLog(logFile),
	); err != nil {
		return false, err
	}
	if err = vhost.Save(); err != nil {
		return false, err
	}

	return true, nil
}

// sourceDialect 按站点目录里最新修改的主配置文件识别上一个 Web 服务器
func (r *websiteRepo) sourceDialect(configDir string) (webserver.Dialect, bool) {
	var latest webserver.Dialect
	var modTime time.Time
	for _, t := range webserver.Types() {
		d, err := webserver.Get(t)
		if err != nil {
			continue
		}
		info, err := os.Stat(filepath.Join(configDir, d.ConfigFile()))
		if err != nil || (!modTime.IsZero() && !info.ModTime().After(modTime)) {
			continue
		}
		latest, modTime = d, info.ModTime()
	}

	return latest, !modTime.IsZero()
}

// latestPHP 取已安装的最高 PHP 版本，没有安装时为 0
func (r *websiteRepo) latestPHP() uint {
	bins, _ := filepath.Glob(filepath.Join(app.Root, "server", "php", "*", "bin", "php"))
	return lo.Max(lo.Map(bins, func(bin string, _ int) uint {
		return cast.ToUint(filepath.Base(filepath.Dir(filepath.Dir(bin))))
	}))
}

// carryOverUpdate 只含类型切换与换服务器重建都原样沿用的设置，其余字段由两边各自取舍
func (r *websiteRepo) carryOverUpdate(setting *types.WebsiteSetting) *request.WebsiteUpdate {
	return &request.WebsiteUpdate{
		ID:           setting.ID,
		Domains:      setting.Domains,
		Path:         setting.Path,
		Root:         setting.Root,
		SSL:          setting.SSL,
		SSLCert:      setting.SSLCert,
		SSLKey:       setting.SSLKey,
		HSTS:         setting.HSTS,
		OCSP:         setting.OCSP,
		HTTPRedirect: setting.HTTPRedirect,
		SSLProtocols: setting.SSLProtocols,
		Upstreams:    setting.Upstreams,
		Redirects:    setting.Redirects,
		BasicAuth:    setting.BasicAuth,
	}
}

// backupConfig 返回后配置目录是空的，setting 必须在调用前读出
func (r *websiteRepo) backupConfig(
	website *biz.Website,
	setting *types.WebsiteSetting,
	tag string,
) (*configBackup, error) {
	configDir := filepath.Join(app.Root, "sites", website.Name, "config")
	backup := &configBackup{
		website:     *website,
		configDir:   configDir,
		backupDir:   fmt.Sprintf("%s.%s-backup-%d", configDir, tag, time.Now().UnixNano()),
		userIni:     filepath.Join(setting.Root, ".user.ini"),
		openBasedir: setting.OpenBasedir,
	}

	data, err := os.ReadFile(backup.userIni)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	backup.userIniData, backup.userIniExists = data, err == nil

	if err = os.Rename(configDir, backup.backupDir); err != nil {
		return nil, err
	}
	if err = r.makeConfigDirs(configDir); err != nil {
		return nil, errors.Join(err, io.Remove(configDir), os.Rename(backup.backupDir, configDir))
	}

	return backup, nil
}

// restoreConfig 不区分失败在哪一步，库字段一律按快照写回；返回并入了还原错误的 cause
func (r *websiteRepo) restoreConfig(ctx context.Context, backup *configBackup, cause error) error {
	err := errors.Join(
		cause,
		io.Remove(backup.configDir),
		os.Rename(backup.backupDir, backup.configDir),
		r.db.Model(&biz.Website{}).Where("id = ?", backup.website.ID).UpdateColumns(map[string]any{
			"type":       backup.website.Type,
			"status":     backup.website.Status,
			"path":       backup.website.Path,
			"ssl":        backup.website.SSL,
			"cert_id":    backup.website.CertID,
			"updated_at": backup.website.UpdatedAt,
		}).Error,
		io.Remove(backup.userIni),
	)
	if backup.userIniExists {
		err = errors.Join(err, io.Write(backup.userIni, string(backup.userIniData), 0644))
		if backup.openBasedir {
			_, attrErr := shell.Execf(ctx, `chattr +i '%s'`, backup.userIni)
			err = errors.Join(err, attrErr)
		}
	}

	return err
}

// rebuildVhost 配置目录是空的，applyUpdate 之外还要补上建站时才写的默认片段和停用标记
func (r *websiteRepo) rebuildVhost(
	ctx context.Context,
	d webserver.Dialect,
	website *biz.Website,
	update *request.WebsiteUpdate,
) error {
	if err := r.applyUpdate(ctx, update, website); err != nil {
		return err
	}

	vhost, err := newVhost(d, website)
	if err != nil {
		return err
	}
	if err = vhost.SetConfig("001-acme.conf", webservertypes.ScopeSite, ""); err != nil {
		return err
	}
	if err = r.writeTypeConfigs(d, vhost, website.Type); err != nil {
		return err
	}
	if err = vhost.SetEnable(website.Status); err != nil {
		return err
	}

	return vhost.Save()
}
