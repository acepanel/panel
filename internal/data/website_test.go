package data

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/assert/check"
	"github.com/libtnb/assert/must"
	"github.com/libtnb/sqlite"
	"gorm.io/gorm"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/pkg/io"
	webservertypes "github.com/acepanel/panel/v3/pkg/webserver/types"
)

type nginxSetting struct{ biz.SettingRepo }

func (nginxSetting) Get(biz.SettingKey, ...string) (string, error)          { return "nginx", nil }
func (nginxSetting) GetBool(biz.SettingKey, ...bool) (bool, error)          { return false, nil }
func (nginxSetting) GetSlice(biz.SettingKey, ...[]string) ([]string, error) { return nil, nil }

// newWebsiteRepoForTest 把 app.Root 指到临时目录，返回仓库和名为 demo 的站点目录
func newWebsiteRepoForTest(t *testing.T) (*websiteRepo, string) {
	t.Helper()
	root := t.TempDir()
	// .user.ini 带 +i 时 TempDir 自带的清理删不掉
	t.Cleanup(func() { _ = io.Remove(root) })
	oldRoot := app.Root
	app.Root = root
	t.Cleanup(func() { app.Root = oldRoot })

	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{SkipDefaultTransaction: true})
	must.NoError(t, err)
	must.NoError(t, db.AutoMigrate(&biz.Cert{}, &biz.Website{}))

	siteDir := filepath.Join(root, "sites", "demo")
	for _, scope := range []string{"site", "shared"} {
		must.NoError(t, os.MkdirAll(filepath.Join(siteDir, "config", scope), 0755))
	}

	return &websiteRepo{t: gotext.NewLocale("", "en"), db: db, setting: nginxSetting{}}, siteDir
}

func TestWebsiteUpdateUserIni(t *testing.T) {
	repo, siteDir := newWebsiteRepoForTest(t)
	sitePath := filepath.Join(siteDir, "public")
	subRoot := filepath.Join(sitePath, "web")
	must.NoError(t, os.MkdirAll(subRoot, 0755))
	website := &biz.Website{Name: "demo", Type: biz.WebsiteTypePHP, Status: true, Path: sitePath}
	must.NoError(t, repo.db.Create(website).Error)

	// 以 runRoot 为运行目录保存一次，返回该目录下 .user.ini 的内容
	save := func(runRoot string) string {
		t.Helper()
		_, err := repo.Update(context.Background(), &request.WebsiteUpdate{
			ID:          website.ID,
			Listens:     []webservertypes.Listen{{Address: "80"}},
			Domains:     []string{"demo.test"},
			Path:        sitePath,
			Root:        runRoot,
			Index:       []string{"index.php"},
			OpenBasedir: true,
		})
		must.NoError(t, err)
		content, err := io.Read(filepath.Join(runRoot, ".user.ini"))
		must.NoError(t, err)
		return content
	}
	userIni := filepath.Join(sitePath, ".user.ini")
	generated := "open_basedir=" + sitePath + "/:" + sitePath + ":/tmp/"

	check.Equal(t, save(sitePath), generated)

	// 目录没变，手工加的内容要留着
	custom := generated + "\nupload_max_filesize=64M\n"
	must.NoError(t, io.Write(userIni, custom, 0644))
	check.Equal(t, save(sitePath), custom)

	// 运行目录变了才重写
	check.Equal(t, save(subRoot), "open_basedir="+subRoot+"/:"+sitePath+":/tmp/")
	check.Equal(t, save(sitePath), generated)

	// 现成文件里没有 open_basedir 视为之前没有开启
	must.NoError(t, io.Write(userIni, "upload_max_filesize=64M\n", 0644))
	check.Equal(t, save(sitePath), generated)
}

func TestWebsiteRestoreConfig(t *testing.T) {
	repo, siteDir := newWebsiteRepoForTest(t)
	configDir := filepath.Join(siteDir, "config")
	configFile := filepath.Join(configDir, "nginx.conf")
	userIni := filepath.Join(siteDir, "public", ".user.ini")
	must.NoError(t, io.Write(configFile, "old", 0600))
	must.NoError(t, io.Write(userIni, "old", 0644))
	website := &biz.Website{Name: "demo", Type: biz.WebsiteTypePHP, Status: true, Path: "/old", SSL: true, CertID: 3}
	must.NoError(t, repo.db.Create(website).Error)

	backup := &configBackup{
		website:       *website,
		configDir:     configDir,
		backupDir:     configDir + ".bak",
		userIni:       userIni,
		userIniData:   []byte("old"),
		userIniExists: true,
	}

	// 重建到一半：配置目录已换新，库字段与 .user.ini 都被改过
	must.NoError(t, os.Rename(configDir, backup.backupDir))
	must.NoError(t, io.Write(configFile, "new", 0600))
	must.NoError(t, io.Write(userIni, "new", 0644))
	changed := *website
	changed.Type, changed.Status, changed.Path, changed.SSL, changed.CertID = biz.WebsiteTypeStatic, false, "/new", false, 0
	must.NoError(t, repo.db.Save(&changed).Error)

	cause := errors.New("rebuild failed")
	err := repo.restoreConfig(context.Background(), backup, cause)
	must.Error(t, err)
	check.Equal(t, err.Error(), cause.Error())

	content, _ := io.Read(configFile)
	check.Equal(t, content, "old")
	check.False(t, io.Exists(backup.backupDir))
	content, _ = io.Read(userIni)
	check.Equal(t, content, "old")

	restored := new(biz.Website)
	must.NoError(t, repo.db.First(restored, website.ID).Error)
	check.Equal(t, restored.Type, website.Type)
	check.Equal(t, restored.Status, website.Status)
	check.Equal(t, restored.Path, website.Path)
	check.Equal(t, restored.SSL, website.SSL)
	check.Equal(t, restored.CertID, website.CertID)
	check.True(t, restored.UpdatedAt.Equal(website.UpdatedAt))
}

func TestWebsiteRebuildPHPMyAdmin(t *testing.T) {
	// 配置目录按 0600 创建，非 root 进不去
	if os.Geteuid() != 0 {
		t.Skip("需要 root")
	}
	repo, _ := newWebsiteRepoForTest(t)
	configDir := filepath.Join(app.Root, "sites", "phpmyadmin", "config")
	oldConf := filepath.Join(configDir, "openlitespeed.conf")
	// 安装脚本为 OpenLiteSpeed 手写的站点配置
	must.NoError(t, io.Write(filepath.Join(configDir, "openlitespeed.listen.conf"), "listen                   8888\ndomain                   *\n", 0644))
	must.NoError(t, io.Write(oldConf, "docRoot                  /opt/ace/server/phpmyadmin\n\nscripthandler {\n  include                /opt/ace/server/openlitespeed/conf/panel/php/83.conf\n}\n", 0644))

	// 没有 PHP 时不动旧配置
	rebuilt, err := repo.RebuildPHPMyAdmin()
	check.Error(t, err)
	check.False(t, rebuilt)
	check.True(t, io.Exists(oldConf))

	for _, version := range []string{"83", "84"} {
		must.NoError(t, io.Write(filepath.Join(app.Root, "server", "php", version, "bin", "php"), "", 0755))
	}
	rebuilt, err = repo.RebuildPHPMyAdmin()
	must.NoError(t, err)
	check.True(t, rebuilt)

	// 旧服务器的配置不留
	check.False(t, io.Exists(oldConf))
	check.False(t, io.Exists(filepath.Join(configDir, "openlitespeed.listen.conf")))

	d, err := repo.dialect()
	must.NoError(t, err)
	vhost, err := d.NewPHPVhost(configDir)
	must.NoError(t, err)
	check.DeepEqual(t, vhost.Listen(), []webservertypes.Listen{{Address: "8888", Args: []string{"default_server"}}})
	check.Equal(t, vhost.Root(), filepath.Join(app.Root, "server", "phpmyadmin"))
	check.DeepEqual(t, vhost.Index(), []string{"index.php"})
	check.Equal(t, vhost.PHP(), uint(84))
	check.Equal(t, vhost.AccessLog(), filepath.Join(app.Root, "sites", "phpmyadmin", "log", "phpmyadmin.log"))

	// 已是当前服务器的配置，不再重建
	rebuilt, err = repo.RebuildPHPMyAdmin()
	must.NoError(t, err)
	check.False(t, rebuilt)
}
