package data

import (
	"context"
	"crypto/md5" //nolint:gosec
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/leonelquinteros/gotext"
	"github.com/samber/lo"
	"github.com/spf13/cast"
	"resty.dev/v3"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/pkg/types"
)

type onePanelAdapter struct {
	t *gotext.Locale
	*migrationClient

	v1    bool // 来源面板为 1Panel v1
	known bool // 大版本已确定，不再回落重试
}

// newOnePanelClient 1Panel 以 JSON 提交，认证为请求头中的 md5("1panel" + 接口密钥 + 时间戳)
func newOnePanelClient(conn *request.ToolboxMigrationConnection) *migrationClient {
	return &migrationClient{
		url: conn.URL,
		sign: func(req *resty.Request) {
			timestamp := strconv.FormatInt(time.Now().Unix(), 10)
			token := md5.Sum([]byte("1panel" + conn.APIKey + timestamp)) //nolint:gosec
			req.SetHeader("1Panel-Timestamp", timestamp).SetHeader("1Panel-Token", hex.EncodeToString(token[:]))
		},
		unwrap: func(body []byte) (any, error) {
			var result struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
				Data    any    `json:"data"`
			}
			if err := json.Unmarshal(body, &result); err != nil {
				return nil, fmt.Errorf("invalid response: %w", err)
			}
			if result.Code != http.StatusOK {
				return nil, errors.New(result.Message)
			}
			return result.Data, nil
		},
		downloadParam: "path",
	}
}

func (a *onePanelAdapter) Probe(ctx context.Context) (*types.MigrationSource, error) {
	data, err := a.api(ctx, http.MethodPost, "/settings/search", nil)
	if err != nil {
		return nil, err
	}
	return &types.MigrationSource{Panel: "onepanel", Version: cast.ToString(cast.ToStringMap(data)["systemVersion"])}, nil
}

func (a *onePanelAdapter) Items(ctx context.Context) ([]types.MigrationItem, error) {
	websites, err := a.websiteItems(ctx)
	if err != nil {
		return nil, err
	}
	return slices.Concat(websites, a.databaseItems(ctx), a.databaseUserItems(ctx)), nil
}

// websiteItems 列出静态、反代与 PHP 运行环境网站
func (a *onePanelAdapter) websiteItems(ctx context.Context) ([]types.MigrationItem, error) {
	// orderBy 用 created_at：v1 只接受下划线写法，v2 两种都接受
	data, err := a.api(ctx, http.MethodPost, "/websites/search", map[string]any{
		"page": 1, "pageSize": 10000, "name": "", "orderBy": "created_at", "order": "descending",
	})
	if err != nil {
		return nil, fmt.Errorf("load websites: %w", err)
	}

	runtimes := a.phpRuntimes(ctx)
	items := make([]types.MigrationItem, 0)
	for _, row := range a.rows(cast.ToStringMap(data)["items"]) {
		id := cast.ToString(row["id"])
		name := lo.CoalesceOrEmpty(cast.ToString(row["alias"]), cast.ToString(row["primaryDomain"]))
		if id == "" || name == "" {
			continue
		}
		item := types.MigrationItem{
			Key: biz.MigrationItemKey("website", id), Type: "website", Name: name,
			Status:     lo.Ternary(strings.EqualFold(cast.ToString(row["status"]), "running"), "running", "stopped"),
			TargetName: name, SourceID: id, SourcePath: cast.ToString(row["sitePath"]),
		}
		// PHP 站点跑在 1Panel 运行环境容器里，其余运行环境与应用商店站点依赖容器编排，无法迁移到 systemd 形态
		switch typ := strings.ToLower(cast.ToString(row["type"])); typ {
		case "static":
			item.Subtype = "static"
		case "proxy":
			item.Subtype = "proxy"
		case "runtime":
			item.Subtype = "php"
			if !strings.EqualFold(cast.ToString(row["runtimeType"]), "php") {
				item.Blockers = []string{a.t.Get("only PHP runtime websites can be migrated: %s", cast.ToString(row["runtimeType"]))}
			}
			// 网站列表只给运行环境名，版本得从运行环境列表里取
			item.Version = runtimes[cast.ToString(row["runtimeName"])]
			// 1Panel 的数据库跑在容器里，提醒修改
			item.Warnings = []string{a.t.Get("if the site connects to a database, change the host in its config from the 1Panel container name to localhost")}
		default:
			item.Subtype = typ
			item.Blockers = []string{a.t.Get("this 1Panel website type is not supported for automatic migration: %s", typ)}
		}
		items = append(items, item)
	}
	return items, nil
}

// phpRuntimes 返回 PHP 运行环境名到版本号的映射
func (a *onePanelAdapter) phpRuntimes(ctx context.Context) map[string]string {
	data, err := a.api(ctx, http.MethodPost, "/runtimes/search", map[string]any{
		"page": 1, "pageSize": 10000, "type": "php",
	})
	if err != nil {
		return nil
	}
	return lo.SliceToMap(a.rows(cast.ToStringMap(data)["items"]), func(row map[string]any) (string, string) {
		return cast.ToString(row["name"]), cast.ToString(cast.ToStringMap(row["params"])["PHP_VERSION"])
	})
}

// databaseItems 列出本机 MySQL / MariaDB / PostgreSQL 数据库
func (a *onePanelAdapter) databaseItems(ctx context.Context) []types.MigrationItem {
	items := make([]types.MigrationItem, 0)
	for _, subtype := range []string{"mysql", "mariadb", "postgresql"} {
		data, err := a.api(ctx, http.MethodGet, "/databases/db/list/"+subtype, nil)
		if err != nil {
			continue
		}
		for _, server := range a.rows(data) {
			// from 为 remote 表示外部数据库，其数据不在来源服务器上
			name := cast.ToString(server["database"])
			if name == "" || strings.EqualFold(cast.ToString(server["from"]), "remote") {
				continue
			}
			items = append(items, a.databasesOf(ctx, subtype, name, cast.ToString(server["version"]))...)
		}
	}
	return items
}

// databaseUserDetail 重新读取用户密码，迁移执行时密码可能已被改过
func (a *onePanelAdapter) databaseUserDetail(ctx context.Context, item types.MigrationItem) (*types.MigrationDatabaseUser, error) {
	data, err := a.api(ctx, http.MethodPost, "/databases/users/search", map[string]any{"database": item.SourceGroup})
	if err != nil {
		return nil, err
	}
	row, ok := lo.Find(a.rows(data), func(row map[string]any) bool {
		return cast.ToString(row["username"]) == item.SourceID && cast.ToString(row["host"]) == item.Version
	})
	if !ok {
		return nil, errors.New(a.t.Get("resource no longer exists on the source server"))
	}

	return &types.MigrationDatabaseUser{
		Type: item.Subtype, Username: item.SourceID,
		Password: cast.ToString(row["password"]), Host: item.Version,
	}, nil
}

// databaseUserItems 列出各数据库服务下的用户，1Panel 的用户接口直接返回明文密码
func (a *onePanelAdapter) databaseUserItems(ctx context.Context) []types.MigrationItem {
	// v1 没有数据库用户接口
	if a.v1 {
		return nil
	}

	items := make([]types.MigrationItem, 0)
	for _, subtype := range []string{"mysql", "mariadb", "postgresql"} {
		data, err := a.api(ctx, http.MethodGet, "/databases/db/list/"+subtype, nil)
		if err != nil {
			continue
		}
		for _, server := range a.rows(data) {
			name := cast.ToString(server["database"])
			if name == "" || strings.EqualFold(cast.ToString(server["from"]), "remote") {
				continue
			}
			items = append(items, a.usersOf(ctx, subtype, name)...)
		}
	}
	return items
}

// usersOf 列出指定数据库服务下的用户
func (a *onePanelAdapter) usersOf(ctx context.Context, subtype, server string) []types.MigrationItem {
	data, err := a.api(ctx, http.MethodPost, "/databases/users/search", map[string]any{"database": server})
	if err != nil {
		return nil
	}

	items := make([]types.MigrationItem, 0)
	for _, row := range a.rows(data) {
		username, password := cast.ToString(row["username"]), cast.ToString(row["password"])
		// 密码不可见的用户迁过去也连不上，不如不迁
		if username == "" || password == "" {
			continue
		}
		host := lo.CoalesceOrEmpty(cast.ToString(row["host"]), "%")
		items = append(items, types.MigrationItem{
			Key:  biz.MigrationItemKey("database_user", subtype+":"+server+":"+username+"@"+host),
			Type: "database_user", Subtype: subtype, Name: username + "@" + host, Status: "running",
			TargetName: username, SourceID: username, SourceGroup: server, Version: host,
		})
	}
	return items
}

// databasesOf 列出指定数据库服务下的库
func (a *onePanelAdapter) databasesOf(ctx context.Context, subtype, server, version string) []types.MigrationItem {
	path := "/databases/search"
	if subtype == "postgresql" {
		path = "/databases/pg/search"
	}
	// orderBy 用 name：v1 只接受 created_at，v2 只接受 createdAt，name 两边都收
	data, err := a.api(ctx, http.MethodPost, path, map[string]any{
		"page": 1, "pageSize": 10000, "info": "", "database": server, "orderBy": "name", "order": "descending",
	})
	if err != nil {
		return nil
	}

	items := make([]types.MigrationItem, 0)
	for _, row := range a.rows(cast.ToStringMap(data)["items"]) {
		id, name := cast.ToString(row["id"]), cast.ToString(row["name"])
		if id == "" || name == "" {
			continue
		}
		items = append(items, types.MigrationItem{
			Key: biz.MigrationItemKey("database", subtype+":"+id), Type: "database", Subtype: subtype, Name: name,
			Status: "running", TargetName: name, SourceID: id, SourceGroup: server, Version: version,
		})
	}
	return items
}

func (a *onePanelAdapter) Detail(ctx context.Context, item types.MigrationItem) (*types.MigrationDetail, error) {
	detail := &types.MigrationDetail{Item: item}
	var err error
	switch item.Type {
	case "website":
		detail.Website, err = a.websiteDetail(ctx, item)
	case "database":
		detail.Database, err = a.databaseDetail(ctx, item)
	case "database_user":
		detail.DatabaseUser, err = a.databaseUserDetail(ctx, item)
	default:
		err = errors.New(a.t.Get("unsupported migration resource type: %s", item.Type))
	}
	return detail, err
}

func (a *onePanelAdapter) websiteDetail(ctx context.Context, item types.MigrationItem) (*types.MigrationWebsite, error) {
	data, err := a.api(ctx, http.MethodGet, "/websites/"+item.SourceID, nil)
	if err != nil {
		return nil, err
	}
	row := cast.ToStringMap(data)

	// 站点目录下的 index 才是网站文件根，siteDir 则是相对它的运行目录
	path := strings.TrimRight(cast.ToString(row["sitePath"]), "/") + "/index"
	website := &types.MigrationWebsite{
		Type: item.Subtype, Path: path, Root: path, Remark: cast.ToString(row["remark"]),
		Index: []string{"index.php", "index.html", "index.htm"}, Enabled: item.Status == "running",
		OpenBasedir: cast.ToBool(row["openBaseDir"]), Rewrite: cast.ToString(row["rewrite"]),
	}
	if dir := cast.ToString(row["siteDir"]); dir != "" && dir != "/" {
		website.Root = path + "/" + strings.TrimLeft(dir, "/")
	}
	if expire := cast.ToTime(row["expireDate"]); !expire.IsZero() && expire.Year() > 1970 {
		website.ExpireAt = &expire
	}
	if item.Subtype == "php" {
		website.PHP = a.runtimePHP(ctx, cast.ToString(row["runtimeID"]))
	}
	website.Domains, website.Listens = a.domains(ctx, item.SourceID)
	website.Proxies = a.proxies(ctx, item.SourceID)
	// 反代站点的根代理写在 proxy 字段，与代理列表重复时以列表为准
	if item.Subtype == "proxy" && !slices.ContainsFunc(website.Proxies, func(proxy types.MigrationProxy) bool {
		return proxy.Location == "/"
	}) {
		if pass := cast.ToString(row["proxy"]); pass != "" {
			website.Proxies = append(website.Proxies, types.MigrationProxy{Location: "/", Pass: a.proxyPass(pass)})
		}
	}
	website.Redirects = a.redirects(ctx, item.SourceID)
	a.applySSL(ctx, item.SourceID, website)
	return website, nil
}

// domains 读取站点域名与监听端口
func (a *onePanelAdapter) domains(ctx context.Context, websiteID string) ([]string, []string) {
	data, err := a.api(ctx, http.MethodGet, "/websites/domains/"+websiteID, nil)
	if err != nil {
		return nil, nil
	}
	domains, listens := make([]string, 0), make([]string, 0)
	for _, row := range a.rows(data) {
		domain, port := cast.ToString(row["domain"]), cast.ToString(row["port"])
		if domain != "" && !lo.Contains(domains, domain) {
			domains = append(domains, domain)
		}
		if port != "" && port != "0" && !lo.Contains(listens, port) {
			listens = append(listens, port)
		}
	}
	return domains, listens
}

// proxies 读取站点反向代理规则
func (a *onePanelAdapter) proxies(ctx context.Context, websiteID string) []types.MigrationProxy {
	data, err := a.api(ctx, http.MethodPost, "/websites/proxies", map[string]any{"id": cast.ToUint(websiteID)})
	if err != nil {
		return nil
	}
	proxies := make([]types.MigrationProxy, 0)
	for _, row := range a.rows(data) {
		pass := cast.ToString(row["proxyPass"])
		if pass == "" || !cast.ToBool(row["enable"]) {
			continue
		}
		proxies = append(proxies, types.MigrationProxy{
			Location: lo.CoalesceOrEmpty(cast.ToString(row["match"]), "/"), Pass: a.proxyPass(pass),
			Host: cast.ToString(row["proxyHost"]), Replaces: cast.ToStringMapString(row["replaces"]),
		})
	}
	return proxies
}

// proxyPass 补全代理目标的协议，1Panel 的 proxy 字段可能只有 host:port
func (a *onePanelAdapter) proxyPass(pass string) string {
	if strings.Contains(pass, "://") {
		return pass
	}

	return "http://" + pass
}

// redirects 读取站点重定向规则，v1 与 v2 都按站点 ID 查询
func (a *onePanelAdapter) redirects(ctx context.Context, websiteID string) []types.MigrationRedirect {
	data, err := a.api(ctx, http.MethodPost, "/websites/redirect", map[string]any{"websiteId": cast.ToUint(websiteID)})
	if err != nil {
		return nil
	}
	redirects := make([]types.MigrationRedirect, 0)
	for _, row := range a.rows(data) {
		target := cast.ToString(row["redirect"])
		if target == "" || !cast.ToBool(row["enable"]) {
			continue
		}
		redirect := types.MigrationRedirect{
			Type: "url", To: target, KeepURI: cast.ToBool(row["keepPath"]),
			StatusCode: cast.ToInt(row["redirectRoot"]),
		}
		switch cast.ToString(row["type"]) {
		case "domain":
			redirect.Type = "host"
			redirect.From = strings.Join(cast.ToStringSlice(row["domains"]), " ")
		default:
			redirect.From = strings.Join(cast.ToStringSlice(row["paths"]), " ")
		}
		if redirect.StatusCode == 0 {
			redirect.StatusCode = http.StatusMovedPermanently
		}
		redirects = append(redirects, redirect)
	}
	return redirects
}

// applySSL 读取证书与 HTTPS 配置
func (a *onePanelAdapter) applySSL(ctx context.Context, websiteID string, website *types.MigrationWebsite) {
	data, err := a.api(ctx, http.MethodGet, "/websites/"+websiteID+"/https", nil)
	if err != nil {
		return
	}
	config := cast.ToStringMap(data)
	ssl := cast.ToStringMap(config["SSL"])
	website.SSLCert = cast.ToString(ssl["pem"])
	website.SSLKey = cast.ToString(ssl["privateKey"])
	website.HTTPRedirect = cast.ToBool(config["httpConfig"] == "HTTPSOnly")
	website.HSTS = cast.ToBool(config["hsts"])
	website.SSL = cast.ToBool(config["enable"]) && website.SSLCert != "" && website.SSLKey != ""
	if !website.SSL {
		return
	}
	website.SSLProtocols = cast.ToStringSlice(config["SSLProtocol"])
	if !lo.Contains(website.Listens, "443") {
		website.Listens = append(website.Listens, "443")
	}
	website.SSLListens = append(website.SSLListens, "443")
}

// runtimePHP 读取运行环境的 PHP 版本，version 可能只有主版本号，params 中的才完整
func (a *onePanelAdapter) runtimePHP(ctx context.Context, runtimeID string) uint {
	if runtimeID == "" || runtimeID == "0" {
		return 0
	}
	data, err := a.api(ctx, http.MethodGet, "/runtimes/"+runtimeID, nil)
	if err != nil {
		return 0
	}
	row := cast.ToStringMap(data)

	if version := a.phpVersion(cast.ToString(cast.ToStringMap(row["params"])["PHP_VERSION"])); version > 0 {
		return version
	}

	return a.phpVersion(cast.ToString(row["version"]))
}

func (a *onePanelAdapter) databaseDetail(ctx context.Context, item types.MigrationItem) (*types.MigrationDatabase, error) {
	path := "/databases/search"
	if item.Subtype == "postgresql" {
		path = "/databases/pg/search"
	}
	data, err := a.api(ctx, http.MethodPost, path, map[string]any{
		"page": 1, "pageSize": 10000, "info": item.Name, "database": item.SourceGroup,
		"orderBy": "name", "order": "descending",
	})
	if err != nil {
		return nil, err
	}
	row, ok := lo.Find(a.rows(cast.ToStringMap(data)["items"]), func(row map[string]any) bool {
		return cast.ToString(row["id"]) == item.SourceID
	})
	if !ok {
		return nil, errors.New(a.t.Get("resource no longer exists on the source server"))
	}
	return &types.MigrationDatabase{
		Type: item.Subtype, Version: item.Version, Name: item.Name, Host: "localhost",
		Username: cast.ToString(row["username"]), Password: cast.ToString(row["password"]),
	}, nil
}

func (a *onePanelAdapter) SetRunning(ctx context.Context, item types.MigrationItem, running bool) error {
	if item.Type != "website" {
		return nil
	}
	_, err := a.api(ctx, http.MethodPost, "/websites/operate", map[string]any{
		"id": cast.ToUint(item.SourceID), "operate": lo.Ternary(running, "start", "stop"),
	})
	return err
}

func (a *onePanelAdapter) Backup(ctx context.Context, detail *types.MigrationDetail) (string, error) {
	item := detail.Item
	name := item.Name
	// 备份记录查询无条件按 name 过滤，而网站记录的 name 在 v1 存主域名、v2 存别名；先读一次站点，顺带确定版本
	if item.Type == "website" {
		data, err := a.api(ctx, http.MethodGet, "/websites/"+item.SourceID, nil)
		if err != nil {
			return "", err
		}
		if a.v1 {
			name = cast.ToString(cast.ToStringMap(data)["primaryDomain"])
		}
	}

	backup := map[string]any{"isImmediate": true}
	switch item.Type {
	case "website":
		backup["type"], backup["name"], backup["detailName"] = "website", name, item.Name
	case "database":
		backup["type"], backup["name"], backup["detailName"] = item.Subtype, item.SourceGroup, item.Name
	default:
		return "", errors.New(a.t.Get("unsupported migration resource type: %s", item.Type))
	}

	// v1 的备份接口是同步的，站点稍大就会超过默认超时；v2 立即返回，放宽超时没有副作用
	if err := a.tryVersions(func() error {
		_, err := a.callTimeout(ctx, http.MethodPost, a.resolve("/backups/backup"), backup, 30*time.Minute)
		return err
	}); err != nil {
		return "", err
	}
	record, err := a.waitBackup(ctx, backup)
	if err != nil {
		return "", err
	}

	// 记录里只有目录与文件名，需换取绝对路径；备份账号 v1 用 source、v2 用 downloadAccountID
	path, err := a.api(ctx, http.MethodPost, "/backups/record/download", map[string]any{
		"source": "LOCAL", "downloadAccountID": 1,
		"fileDir": record["fileDir"], "fileName": record["fileName"],
	})
	if err != nil {
		return "", err
	}
	return cast.ToString(path), nil
}

// waitBackup 轮询最新一条备份记录直到写盘完成，v2 的备份接口只是把任务丢进后台就返回
func (a *onePanelAdapter) waitBackup(ctx context.Context, backup map[string]any) (map[string]any, error) {
	for range 3600 {
		data, err := a.api(ctx, http.MethodPost, "/backups/record/search", map[string]any{
			"page": 1, "pageSize": 1, "type": backup["type"], "name": backup["name"], "detailName": backup["detailName"],
		})
		if err != nil {
			return nil, err
		}
		if records := a.rows(cast.ToStringMap(data)["items"]); len(records) > 0 {
			// v1 的备份记录没有 status，记录出现即代表备份已完成
			switch cast.ToString(records[0]["status"]) {
			case "", "Success":
				return records[0], nil
			case "Failed":
				return nil, errors.New(a.t.Get("the source task failed"))
			}
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Second):
		}
	}

	return nil, errors.New(a.t.Get("the source backup did not finish in time"))
}

func (a *onePanelAdapter) Download(ctx context.Context, remote, local string, progress types.MigrationProgress) error {
	return a.tryVersions(func() error {
		a.downloadPath = a.resolve("/files/download")
		return a.download(ctx, remote, local, progress)
	})
}

// api 调用 1Panel 接口
func (a *onePanelAdapter) api(ctx context.Context, method, path string, body map[string]any) (any, error) {
	var data any
	err := a.tryVersions(func() error {
		var err error
		data, err = a.call(ctx, method, a.resolve(path), body)
		return err
	})

	return data, err
}

// resolve 拼接接口路径，v1 与 v2 的前缀不同，且备份接口在 v1 挂在 /settings 下
func (a *onePanelAdapter) resolve(path string) string {
	if !a.v1 {
		return "/api/v2" + path
	}
	if rest, ok := strings.CutPrefix(path, "/backups"); ok {
		return "/api/v1/settings/backup" + rest
	}

	return "/api/v1" + path
}

// tryVersions 大版本未确定时先按 v2 再按 v1 尝试，两版都失败则同时上报两个错误
func (a *onePanelAdapter) tryVersions(run func() error) error {
	err := run()
	if err == nil {
		a.known = true
		return nil
	}
	if a.known {
		return err
	}

	a.v1 = true
	retry := run()
	if retry == nil {
		a.known = true
		return nil
	}
	a.v1 = false

	// v2 在 v1 面板上只会拿到未认证守卫页（如 416），真实原因通常在后一条
	return fmt.Errorf("v2: %w; v1: %w", err, retry)
}
