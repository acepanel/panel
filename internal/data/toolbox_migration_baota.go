package data

import (
	"context"
	"crypto/md5" //nolint:gosec
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
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

type baotaAdapter struct {
	t *gotext.Locale
	*migrationClient
}

// newBaotaClient 宝塔以表单提交，认证为 md5(时间戳 + md5(接口密钥))
func newBaotaClient(conn *request.ToolboxMigrationConnection) *migrationClient {
	return &migrationClient{
		url:  conn.URL,
		form: true,
		// 认证一律走 query，resty 在签名时还未确定请求方法，无法按方法分流
		sign: func(req *resty.Request) {
			timestamp := strconv.FormatInt(time.Now().Unix(), 10)
			secret := md5.Sum([]byte(conn.APIKey))                              //nolint:gosec
			token := md5.Sum([]byte(timestamp + hex.EncodeToString(secret[:]))) //nolint:gosec
			req.SetQueryParams(map[string]string{"request_time": timestamp, "request_token": hex.EncodeToString(token[:])})
		},
		// 宝塔失败响应为 {status: false, msg: "..."}，部分接口的业务数据也含 status 字段，需同时判断 msg
		unwrap: func(body []byte) (any, error) {
			var raw any
			if err := json.Unmarshal(body, &raw); err != nil {
				return nil, fmt.Errorf("invalid response: %w", err)
			}
			result := cast.ToStringMap(raw)
			if status, ok := result["status"].(bool); ok && !status && result["msg"] != nil {
				return nil, errors.New(cast.ToString(result["msg"]))
			}
			if data, ok := result["data"]; ok {
				return data, nil
			}
			return raw, nil
		},
		downloadPath:  "/download",
		downloadParam: "filename",
	}
}

func (a *baotaAdapter) Probe(ctx context.Context) (*types.MigrationSource, error) {
	data, err := a.call(ctx, http.MethodPost, "/system?action=GetSystemTotal", nil)
	if err != nil {
		return nil, err
	}
	return &types.MigrationSource{Panel: "baota", Version: cast.ToString(cast.ToStringMap(data)["version"])}, nil
}

func (a *baotaAdapter) Items(ctx context.Context) ([]types.MigrationItem, error) {
	websites, err := a.websiteItems(ctx)
	if err != nil {
		return nil, err
	}
	rows, subtype := a.databaseRows(ctx)
	databases := a.databaseItems(rows, subtype)
	// 宝塔一键部署的站点与数据库同名，据此建立依赖关系
	for i := range websites {
		if index := slices.IndexFunc(databases, func(database types.MigrationItem) bool {
			return database.Name == websites[i].Name
		}); index >= 0 {
			websites[i].DependsOn = append(websites[i].DependsOn, databases[index].Key)
		}
	}
	return slices.Concat(websites, a.dockerWebsiteItems(ctx), databases, a.databaseUserItems(rows, subtype), a.projectItems(ctx)), nil
}

// websiteItems 列出静态、PHP 与反代网站
func (a *baotaAdapter) websiteItems(ctx context.Context) ([]types.MigrationItem, error) {
	data, err := a.call(ctx, http.MethodPost, "/data?action=getData", map[string]any{
		"table": "sites", "p": 1, "limit": 10000,
	})
	if err != nil {
		return nil, fmt.Errorf("load websites: %w", err)
	}

	items := make([]types.MigrationItem, 0)
	for _, row := range a.rows(data) {
		id, name := cast.ToString(row["id"]), cast.ToString(row["name"])
		if id == "" || name == "" {
			continue
		}
		// PHP / WP2 为 PHP 站点，HTML 为静态站点，Proxy 为反代站点，其余是项目
		var subtype string
		switch strings.ToLower(cast.ToString(row["project_type"])) {
		case "php", "wp2":
			subtype = lo.Ternary(cast.ToString(row["php_version"]) == "静态", "static", "php")
		case "html":
			subtype = "static"
		case "proxy":
			subtype = "proxy"
		default:
			continue
		}
		items = append(items, types.MigrationItem{
			Key: biz.MigrationItemKey("website", id), Type: "website", Subtype: subtype, Name: name,
			Status:     lo.Ternary(cast.ToString(row["status"]) == "1", "running", "stopped"),
			Size:       cast.ToInt64(cast.ToStringMap(row["quota"])["used"]),
			TargetName: name, SourceID: id, SourcePath: cast.ToString(row["path"]),
			// 预检阶段据此提示目标是否缺少对应 PHP 版本
			Version: lo.Ternary(subtype == "php", cast.ToString(row["php_version"]), ""),
		})
	}
	return items, nil
}

// dockerWebsiteItems 列出 Docker 模块里的反代网站，它们存在 Docker 自己的库里，站点表读不到
// 容器里跑 PHP 的站点没有代理目标，不在迁移范围
func (a *baotaAdapter) dockerWebsiteItems(ctx context.Context) []types.MigrationItem {
	data, err := a.call(ctx, http.MethodPost, "/mod/docker/com/get_site_list", map[string]any{
		"type": "all", "p": 1, "row": 10000,
	})
	if err != nil {
		return nil
	}

	items := make([]types.MigrationItem, 0)
	for _, row := range a.rows(data) {
		id, name := cast.ToString(row["id"]), cast.ToString(row["name"])
		if id == "" || name == "" || cast.ToString(row["proxy_pass"]) == "" {
			continue
		}
		items = append(items, types.MigrationItem{
			Key: biz.MigrationItemKey("website", "docker:"+id), Type: "website", Subtype: "proxy", Name: name,
			Status:     lo.Ternary(cast.ToString(row["status"]) == "1", "running", "stopped"),
			TargetName: name, SourceID: id, SourcePath: cast.ToString(row["path"]), SourceGroup: "docker",
		})
	}
	return items
}

// databaseRows 读取 databases 表与本机 MySQL 的实际分支
func (a *baotaAdapter) databaseRows(ctx context.Context) ([]map[string]any, string) {
	data, err := a.call(ctx, http.MethodPost, "/data?action=getData", map[string]any{
		"table": "databases", "p": 1, "limit": 10000,
	})
	if err != nil {
		return nil, ""
	}
	return a.rows(data), a.mysqlSubtype(ctx)
}

// mysqlSubtype 宝塔的 MySQL 可能实际是 MariaDB，版本文件里能区分
func (a *baotaAdapter) mysqlSubtype(ctx context.Context) string {
	version, err := a.fileContent(ctx, "/www/server/mysql/version.pl")
	if err == nil && strings.Contains(strings.ToLower(version), "mariadb") {
		return "mariadb"
	}

	return "mysql"
}

// databaseItems 列出本机 MySQL 数据库
func (a *baotaAdapter) databaseItems(rows []map[string]any, subtype string) []types.MigrationItem {
	items := make([]types.MigrationItem, 0)
	for _, row := range rows {
		id, name := cast.ToString(row["id"]), cast.ToString(row["name"])
		// db_type 与 sid 非 0 表示远程数据库，不在迁移范围内
		if id == "" || name == "" || !strings.EqualFold(cast.ToString(row["type"]), "mysql") ||
			cast.ToInt(row["db_type"]) != 0 || cast.ToInt(row["sid"]) != 0 {
			continue
		}
		items = append(items, types.MigrationItem{
			Key: biz.MigrationItemKey("database", id), Type: "database", Subtype: subtype, Name: name, Status: "running",
			Size: cast.ToInt64(cast.ToStringMap(row["quota"])["used"]), TargetName: name, SourceID: id,
		})
	}
	return items
}

// databaseUserItems 列出数据库用户
func (a *baotaAdapter) databaseUserItems(rows []map[string]any, subtype string) []types.MigrationItem {
	items := make([]types.MigrationItem, 0)
	for _, row := range rows {
		username, password := cast.ToString(row["username"]), cast.ToString(row["password"])
		if username == "" || password == "" || cast.ToInt(row["db_type"]) != 0 || cast.ToInt(row["sid"]) != 0 {
			continue
		}
		host := lo.CoalesceOrEmpty(cast.ToString(row["accept"]), "localhost")
		key := biz.MigrationItemKey("database_user", username+"@"+host)
		// 多个库共用同一用户时只保留一项
		if slices.ContainsFunc(items, func(item types.MigrationItem) bool { return item.Key == key }) {
			continue
		}
		items = append(items, types.MigrationItem{
			Key: key, Type: "database_user", Subtype: subtype, Name: username + "@" + host, Status: "running",
			TargetName: username, SourceID: username, Version: host,
		})
	}
	return items
}

// projectItems 列出各语言项目
func (a *baotaAdapter) projectItems(ctx context.Context) []types.MigrationItem {
	// 宝塔项目模块与 AcePanel 项目类型的对应关系，nodejs 等模块名同时用于拼接接口路径
	baotaProjectModules := map[string]types.ProjectType{
		"nodejs": types.ProjectTypeNodejs,
		"python": types.ProjectTypePython,
		"go":     types.ProjectTypeGo,
		"net":    types.ProjectTypeDotnet,
		"java":   types.ProjectTypeJava,
		"other":  types.ProjectTypeGeneral,
	}
	items := make([]types.MigrationItem, 0)
	for module, projectType := range baotaProjectModules {
		data, err := a.call(ctx, http.MethodPost, a.projectPath(module, "list"), map[string]any{"p": 1, "limit": 10000})
		if err != nil {
			continue
		}
		for _, row := range a.rows(data) {
			id, name := cast.ToString(row["id"]), cast.ToString(row["name"])
			if id == "" || name == "" {
				continue
			}
			config := cast.ToStringMap(row["project_config"])
			item := types.MigrationItem{
				Key: biz.MigrationItemKey("project", module+":"+id), Type: "project", Subtype: string(projectType), Name: name,
				Status:     lo.Ternary(cast.ToBool(row["run"]), "running", "stopped"),
				TargetName: name, SourceID: id, SourcePath: cast.ToString(row["path"]),
				SourceGroup: module, Version: a.projectVersion(config, module),
			}
			// Tomcat 项目的部署形态与 AcePanel 的 systemd 项目不兼容
			if module == "java" && !strings.EqualFold(cast.ToString(config["java_type"]), "springboot") &&
				cast.ToString(config["java_type"]) != "" {
				item.Blockers = []string{a.t.Get("Tomcat projects are not supported for automatic migration")}
			}
			items = append(items, item)
		}
	}
	return items
}

// projectVersion 取项目配置中的运行时版本
func (a *baotaAdapter) projectVersion(config map[string]any, module string) string {
	switch module {
	case "nodejs":
		return cast.ToString(config["nodejs_version"])
	case "python":
		return lo.CoalesceOrEmpty(cast.ToString(config["version"]), cast.ToString(config["python_bin"]))
	case "java":
		// 取 JDK 目录名而非完整路径，避免路径中的其他数字被当成版本号
		if home := cast.ToString(config["jdk_path"]); home != "" {
			return filepath.Base(home)
		}
		return filepath.Base(filepath.Dir(filepath.Dir(cast.ToString(config["project_jdk"]))))
	case "net":
		return cast.ToString(config["dotnet_version"])
	default:
		return ""
	}
}

func (a *baotaAdapter) Detail(ctx context.Context, item types.MigrationItem) (*types.MigrationDetail, error) {
	detail := &types.MigrationDetail{Item: item}
	var err error
	switch item.Type {
	case "website":
		if item.SourceGroup == "docker" {
			detail.Website = a.dockerWebsiteDetail(ctx, item)
		} else {
			detail.Website = a.websiteDetail(ctx, item)
		}
	case "database":
		detail.Database, err = a.databaseDetail(ctx, item)
	case "database_user":
		detail.DatabaseUser, err = a.databaseUserDetail(ctx, item)
	case "project":
		detail.Project, err = a.projectDetail(ctx, item)
	default:
		err = errors.New(a.t.Get("unsupported migration resource type: %s", item.Type))
	}
	return detail, err
}

func (a *baotaAdapter) websiteDetail(ctx context.Context, item types.MigrationItem) *types.MigrationWebsite {
	website := &types.MigrationWebsite{
		Type: item.Subtype, Path: item.SourcePath, Root: item.SourcePath,
		Domains: []string{item.Name}, Listens: []string{"80"},
		Index: []string{"index.php", "index.html", "index.htm"}, Enabled: item.Status == "running",
	}

	// 备注与到期时间来自站点表，停止的站点还会把原运行目录存进 project_config
	var stoppedRunPath string
	if data, err := a.call(ctx, http.MethodPost, "/data?action=getData", map[string]any{
		"table": "sites", "p": 1, "limit": 10000, "search": item.Name,
	}); err == nil {
		if row, ok := lo.Find(a.rows(data), func(row map[string]any) bool { return cast.ToString(row["id"]) == item.SourceID }); ok {
			website.Remark = cast.ToString(row["ps"])
			website.ExpireAt = a.parseDate(cast.ToString(row["edate"]))
			var config struct {
				PHPRunPath string `json:"php_run_path"`
			}
			if json.Unmarshal([]byte(cast.ToString(row["project_config"])), &config) == nil {
				stoppedRunPath = config.PHPRunPath
			}
		}
	}

	if domains, listens := a.domains(ctx, item.SourceID); len(domains) > 0 {
		website.Domains, website.Listens = domains, listens
	}

	if data, err := a.call(ctx, http.MethodPost, "/site?action=GetSitePHPVersion", map[string]any{"siteName": item.Name}); err == nil {
		website.PHP = a.phpVersion(cast.ToString(cast.ToStringMap(data)["phpversion"]))
	}
	if website.PHP > 0 {
		website.Type = "php"
	}

	// 运行目录相对站点目录，同时带出 open_basedir 状态
	if data, err := a.call(ctx, http.MethodPost, "/site?action=GetDirUserINI", map[string]any{
		"id": item.SourceID, "path": website.Path,
	}); err == nil {
		info := cast.ToStringMap(data)
		website.OpenBasedir = cast.ToBool(info["userini"])
		// 站点停止时根目录被指向 /www/server/stop，改用 project_config 中保存的原值
		run := cast.ToStringMap(info["runPath"])
		runPath := cast.ToString(run["runPath"])
		if stoppedRunPath != "" {
			runPath = stoppedRunPath
		} else if !slices.Contains(cast.ToStringSlice(run["dirs"]), runPath) {
			runPath = "/"
		}
		if runPath != "" && runPath != "/" {
			website.Root = filepath.Join(website.Path, strings.TrimPrefix(runPath, "/"))
		}
	}
	if data, err := a.call(ctx, http.MethodPost, "/site?action=GetIndex", map[string]any{"id": item.SourceID}); err == nil {
		if index := strings.FieldsFunc(cast.ToString(data), func(char rune) bool {
			return char == ',' || char == ' ' || char == '\t'
		}); len(index) > 0 {
			website.Index = index
		}
	}
	website.Rewrite = a.rewrite(ctx, item)
	website.Proxies = a.proxies(ctx, item)
	if website.Type == "static" && slices.ContainsFunc(website.Proxies, func(proxy types.MigrationProxy) bool { return proxy.Location == a.proxyLocation("/") }) {
		website.Type = "proxy"
	}
	website.Redirects = a.redirects(ctx, item.Name)
	a.applySSL(ctx, item.Name, website)
	return website
}

// dockerWebsiteDetail 读取 Docker 网站的配置
// 站点接口多按 ID 查站点表，对它不可用，域名与端口改从它自己的配置文件里取
func (a *baotaAdapter) dockerWebsiteDetail(ctx context.Context, item types.MigrationItem) *types.MigrationWebsite {
	config := a.moduleConfig(ctx, "/www/dk_project/sites", item.Name)
	website := &types.MigrationWebsite{
		Type: item.Subtype, Path: item.SourcePath, Root: item.SourcePath,
		Listens: cast.ToStringSlice(config["site_port"]), Remark: cast.ToString(config["remark"]),
		Enabled: item.Status == "running", Proxies: a.moduleProxies(config),
	}
	// 建站时填的域名带端口，之后追加的不带
	for _, domain := range cast.ToStringSlice(config["domain_list"]) {
		host, _, _ := strings.Cut(domain, ":")
		website.Domains = append(website.Domains, host)
	}
	website.Redirects = a.redirects(ctx, item.Name)
	a.applySSL(ctx, item.Name, website)
	return website
}

// parseDate 解析宝塔的日期字段，0000-00-00 表示未设置
func (a *baotaAdapter) parseDate(value string) *time.Time {
	value = strings.TrimSpace(value)
	if value == "" || value == "0000-00-00" {
		return nil
	}
	parsed, err := time.ParseInLocation(time.DateOnly, value, time.Local)
	if err != nil {
		return nil
	}
	return &parsed
}

// domains 读取站点绑定的域名与端口
func (a *baotaAdapter) domains(ctx context.Context, siteID string) ([]string, []string) {
	data, err := a.call(ctx, http.MethodPost, "/data?action=getData", map[string]any{
		"table": "domain", "p": 1, "limit": 10000, "search": siteID,
	})
	if err != nil {
		return nil, nil
	}
	domains, listens := make([]string, 0), make([]string, 0)
	for _, row := range a.rows(data) {
		if cast.ToString(row["pid"]) != siteID {
			continue
		}
		if domain := cast.ToString(row["name"]); domain != "" && !slices.Contains(domains, domain) {
			domains = append(domains, domain)
		}
		if port := cast.ToString(row["port"]); port != "" && !slices.Contains(listens, port) {
			listens = append(listens, port)
		}
	}
	return domains, listens
}

// rewrite 读取站点伪静态规则内容
func (a *baotaAdapter) rewrite(ctx context.Context, item types.MigrationItem) string {
	data, err := a.call(ctx, http.MethodPost, "/site?action=GetRewriteLists", map[string]any{
		"site_ids": "[" + item.SourceID + "]", "site_type": item.Subtype,
	})
	if err != nil {
		return ""
	}
	for _, row := range a.rows(cast.ToStringMap(data)["site_rewrites"]) {
		file := cast.ToString(row["file"])
		if cast.ToString(row["id"]) != item.SourceID || file == "" {
			continue
		}
		content, contentErr := a.fileContent(ctx, file)
		if contentErr != nil {
			return ""
		}
		return content
	}
	return ""
}

// proxies 读取站点反向代理规则
// 反代站点有新旧两代：新版的规则归代理模块管，旧版的和普通站点同一格式，只是存在项目接口下
func (a *baotaAdapter) proxies(ctx context.Context, item types.MigrationItem) []types.MigrationProxy {
	if item.Subtype != "proxy" {
		return a.listProxies(ctx, "/site?action=GetProxyList", item.Name)
	}
	if proxies := a.moduleProxies(a.moduleConfig(ctx, "/www/server/proxy_project/sites", item.Name)); len(proxies) > 0 {
		return proxies
	}

	return a.listProxies(ctx, "/project/proxy/GetProxyList", item.Name)
}

// listProxies 从列表接口读取反向代理规则
func (a *baotaAdapter) listProxies(ctx context.Context, path, name string) []types.MigrationProxy {
	data, err := a.call(ctx, http.MethodPost, path, map[string]any{"sitename": name})
	if err != nil {
		return nil
	}
	proxies := make([]types.MigrationProxy, 0)
	for _, row := range a.rows(data) {
		// type 为 0 表示该代理规则已停用
		pass := cast.ToString(row["proxysite"])
		if pass == "" || cast.ToInt(row["type"]) != 1 {
			continue
		}
		replaces := lo.SliceToMap(a.rows(row["subfilter"]), func(filter map[string]any) (string, string) {
			return cast.ToString(filter["sub1"]), cast.ToString(filter["sub2"])
		})
		delete(replaces, "")
		proxies = append(proxies, types.MigrationProxy{
			Location: a.proxyLocation(cast.ToString(row["proxydir"])),
			Pass:     pass, Host: cast.ToString(row["todomain"]), Replaces: replaces,
		})
	}
	return proxies
}

// moduleConfig 读取代理模块或 Docker 模块为站点保存的配置
// 直接读配置文件：模块接口在面板未绑定账号时会被重定向到绑定页
func (a *baotaAdapter) moduleConfig(ctx context.Context, dir, name string) map[string]any {
	content, err := a.fileContent(ctx, filepath.Join(dir, name, name+".json"))
	if err != nil {
		return nil
	}
	var config map[string]any
	_ = json.Unmarshal([]byte(content), &config)
	return config
}

// moduleProxies 取出模块配置里的反向代理规则
func (a *baotaAdapter) moduleProxies(config map[string]any) []types.MigrationProxy {
	proxies := make([]types.MigrationProxy, 0)
	for _, row := range a.rows(config["proxy_info"]) {
		// AcePanel 不支持 unix 套接字形态的代理目标
		pass := cast.ToString(row["proxy_pass"])
		if pass == "" || strings.HasPrefix(pass, "http://unix:") {
			continue
		}
		replaces := lo.SliceToMap(a.rows(cast.ToStringMap(row["sub_filter"])["sub_filter_str"]), func(filter map[string]any) (string, string) {
			return cast.ToString(filter["oldstr"]), cast.ToString(filter["newstr"])
		})
		delete(replaces, "")
		proxies = append(proxies, types.MigrationProxy{
			Location: a.proxyLocation(cast.ToString(row["proxy_path"])),
			Pass:     pass, Host: cast.ToString(row["proxy_host"]), Replaces: replaces,
		})
	}
	return proxies
}

// proxyLocation 宝塔的反代规则一律生成 location ^~，照此还原才不会被站点里的正则 location 抢走请求
func (a *baotaAdapter) proxyLocation(path string) string {
	return "^~ " + lo.CoalesceOrEmpty(path, "/")
}

// redirects 读取站点重定向规则
func (a *baotaAdapter) redirects(ctx context.Context, name string) []types.MigrationRedirect {
	data, err := a.call(ctx, http.MethodPost, "/site?action=GetRedirectList", map[string]any{"sitename": name, "errorpage": "0"})
	if err != nil {
		return nil
	}
	redirects := make([]types.MigrationRedirect, 0)
	for _, row := range a.rows(data) {
		target := cast.ToString(row["tourl"])
		if target == "" || cast.ToString(row["type"]) == "0" {
			continue
		}
		redirect := types.MigrationRedirect{
			Type: "url", From: cast.ToString(row["redirectpath"]), To: target,
			KeepURI: cast.ToBool(row["holdpath"]), StatusCode: cast.ToInt(row["redirecttype"]),
		}
		if cast.ToString(row["domainorpath"]) == "domain" {
			redirect.Type = "host"
			redirect.From = strings.Join(cast.ToStringSlice(row["redirectdomain"]), " ")
		}
		if redirect.StatusCode == 0 {
			redirect.StatusCode = http.StatusMovedPermanently
		}
		redirects = append(redirects, redirect)
	}
	return redirects
}

// applySSL 读取证书与 TLS 配置
func (a *baotaAdapter) applySSL(ctx context.Context, name string, website *types.MigrationWebsite) {
	data, err := a.call(ctx, http.MethodPost, "/site?action=GetSSL", map[string]any{"siteName": name})
	if err != nil {
		return
	}
	ssl := cast.ToStringMap(data)
	// 没有证书时这两个字段是 false 而不是空字符串
	website.SSLCert, _ = ssl["csr"].(string)
	website.SSLKey, _ = ssl["key"].(string)
	website.HTTPRedirect = cast.ToBool(ssl["httpTohttps"])
	website.SSL = cast.ToBool(ssl["status"]) && website.SSLCert != "" && website.SSLKey != ""
	if !website.SSL {
		return
	}
	protocols := cast.ToStringMap(ssl["tls_versions"])
	for _, protocol := range []string{"TLSv1", "TLSv1.1", "TLSv1.2", "TLSv1.3"} {
		if cast.ToBool(protocols[protocol]) {
			website.SSLProtocols = append(website.SSLProtocols, protocol)
		}
	}
	port := "443"
	if data, err = a.call(ctx, http.MethodPost, "/data?action=get_https_port", map[string]any{"siteName": name}); err == nil {
		if value := cast.ToString(data); value != "" && value != "0" {
			port = value
		}
	}
	if !slices.Contains(website.Listens, port) {
		website.Listens = append(website.Listens, port)
	}
	website.SSLListens = append(website.SSLListens, port)
}

func (a *baotaAdapter) databaseDetail(ctx context.Context, item types.MigrationItem) (*types.MigrationDatabase, error) {
	database := &types.MigrationDatabase{Type: item.Subtype, Name: item.Name, Host: "localhost"}
	data, err := a.call(ctx, http.MethodPost, "/data?action=getData", map[string]any{
		"table": "databases", "p": 1, "limit": 10000, "search": item.Name,
	})
	if err != nil {
		return nil, err
	}
	row, ok := lo.Find(a.rows(data), func(row map[string]any) bool { return cast.ToString(row["id"]) == item.SourceID })
	if !ok {
		return nil, errors.New(a.t.Get("resource no longer exists on the source server"))
	}
	database.Username = cast.ToString(row["username"])
	database.Password = cast.ToString(row["password"])
	database.Host = lo.CoalesceOrEmpty(cast.ToString(row["accept"]), "localhost")
	if version, versionErr := a.fileContent(ctx, "/www/server/mysql/version.pl"); versionErr == nil {
		database.Version = strings.TrimSpace(version)
	}
	return database, nil
}

// databaseUserDetail 重新读取用户密码，迁移执行时密码可能已被改过
func (a *baotaAdapter) databaseUserDetail(
	ctx context.Context,
	item types.MigrationItem,
) (*types.MigrationDatabaseUser, error) {
	data, err := a.call(ctx, http.MethodPost, "/data?action=getData", map[string]any{
		"table": "databases", "p": 1, "limit": 10000, "search": item.SourceID,
	})
	if err != nil {
		return nil, err
	}
	row, ok := lo.Find(a.rows(data), func(row map[string]any) bool {
		return cast.ToString(row["username"]) == item.SourceID
	})
	if !ok {
		return nil, errors.New(a.t.Get("resource no longer exists on the source server"))
	}

	return &types.MigrationDatabaseUser{
		Type: item.Subtype, Username: item.SourceID,
		Password: cast.ToString(row["password"]), Host: item.Version,
	}, nil
}

func (a *baotaAdapter) projectDetail(ctx context.Context, item types.MigrationItem) (*types.MigrationProject, error) {
	data, err := a.call(ctx, http.MethodPost, a.projectPath(item.SourceGroup, "list"), map[string]any{"p": 1, "limit": 10000})
	if err != nil {
		return nil, err
	}
	row, ok := lo.Find(a.rows(data), func(row map[string]any) bool { return cast.ToString(row["id"]) == item.SourceID })
	if !ok {
		return nil, errors.New(a.t.Get("resource no longer exists on the source server"))
	}

	config := cast.ToStringMap(row["project_config"])
	path := a.projectRoot(row, config, item.SourceGroup)
	project := &types.MigrationProject{
		Type: types.ProjectType(item.Subtype), Version: item.Version, Path: path, WorkingDir: path,
		ExecStart: a.projectExecStart(ctx, config, item.SourceGroup, path),
		// 运行用户与开机自启的字段名逐模块不同
		User:    lo.CoalesceOrEmpty(cast.ToString(config["run_user"]), cast.ToString(config["user"]), "www"),
		Port:    cast.ToUint(config["port"]),
		Running: item.Status == "running",
		Enabled: cast.ToBool(lo.CoalesceOrEmpty(config["is_power_on"], config["auto_run"], config["auth"])),
	}
	project.Domains, project.Listens = a.domains(ctx, item.SourceID)
	project.Environments = a.projectEnvironments(ctx, config)
	if project.ExecStart == "" {
		return nil, errors.New(a.t.Get("the source project start command is missing"))
	}
	return project, nil
}

// projectRoot 各语言项目的代码目录字段不同
func (a *baotaAdapter) projectRoot(row, config map[string]any, module string) string {
	path := cast.ToString(row["path"])
	switch module {
	case "nodejs":
		return lo.CoalesceOrEmpty(cast.ToString(config["project_cwd"]), path)
	case "python":
		return lo.CoalesceOrEmpty(cast.ToString(config["path"]), path)
	case "go", "net":
		return lo.CoalesceOrEmpty(cast.ToString(config["project_path"]), path)
	case "java":
		return lo.CoalesceOrEmpty(cast.ToString(config["jar_path"]), filepath.Dir(cast.ToString(config["project_jar"])), path)
	default:
		// 其他项目的 project_exe 即项目目录，与 path 同值
		return path
	}
}

// projectExecStart 还原项目启动命令
func (a *baotaAdapter) projectExecStart(ctx context.Context, config map[string]any, module, path string) string {
	if command := cast.ToString(config["project_cmd"]); command != "" {
		return command
	}
	switch module {
	case "nodejs":
		return a.nodejsExecStart(ctx, config, path)
	case "python":
		return a.pythonExecStart(config, path)
	default:
		return ""
	}
}

// nodejsExecStart 还原 Node.js 项目启动命令
// 传统与 PM2 模式的 project_script 存的是整段启动脚本，只有入口文件字段可用
func (a *baotaAdapter) nodejsExecStart(ctx context.Context, config map[string]any, path string) string {
	if file := cast.ToString(config["project_file"]); file != "" {
		return strings.TrimSpace("node " + file + " " + cast.ToString(config["project_args"]))
	}

	script := cast.ToString(config["project_script"])
	// 进程管理交给 systemd，剥掉 pm2 只留入口
	if fields := strings.Fields(script); len(fields) > 2 && fields[0] == "pm2" && fields[1] == "start" {
		script = strings.Join(fields[2:], " ")
	}
	if script == "" {
		return ""
	}
	if content, err := a.fileContent(ctx, filepath.Join(path, "package.json")); err == nil {
		var packageInfo struct {
			Scripts map[string]string `json:"scripts"`
		}
		if json.Unmarshal([]byte(content), &packageInfo) == nil {
			if _, ok := packageInfo.Scripts[script]; ok {
				return "npm run " + script
			}
		}
	}
	if fields := strings.Fields(script); len(fields) > 0 &&
		slices.Contains([]string{".js", ".cjs", ".mjs"}, strings.ToLower(filepath.Ext(fields[0]))) {
		return "node " + script
	}
	return script
}

// pythonExecStart Python 项目按部署方式生成启动命令
// 不复用宝塔生成的配置文件：日志目录是写死的，uwsgi.ini 还带 daemonize 会让 systemd 判定启动失败
func (a *baotaAdapter) pythonExecStart(config map[string]any, path string) string {
	entry := cast.ToString(config["rfile"])
	listen := "0.0.0.0:" + cast.ToString(config["port"])
	processes := cast.ToString(lo.CoalesceOrEmpty(config["processes"], 4))
	threads := cast.ToString(lo.CoalesceOrEmpty(config["threads"], 2))

	switch strings.ToLower(cast.ToString(config["stype"])) {
	case "python":
		return strings.TrimSpace("python -u " + entry + " " + cast.ToString(config["parm"]))
	case "uwsgi":
		return strings.Join([]string{
			"uwsgi --master --http", listen, "--chdir", path, "--wsgi-file", entry,
			"--callable", cast.ToString(config["call_app"]), "--processes", processes, "--threads", threads,
		}, " ")
	case "gunicorn":
		module := strings.ReplaceAll(strings.TrimSuffix(strings.TrimPrefix(entry, path+"/"), ".py"), "/", ".")
		return strings.Join([]string{
			"python -m gunicorn -b", listen, "-w", processes, "--threads", threads,
			module + ":" + cast.ToString(config["call_app"]),
		}, " ")
	default:
		return ""
	}
}

// projectEnvironments 合并项目配置与环境变量文件中的变量
// Python 与 Go 用 {key,value} 列表 env_list，Node.js 用 env，其他项目用 environment
func (a *baotaAdapter) projectEnvironments(ctx context.Context, config map[string]any) []types.KV {
	values := lo.FilterMap(a.rows(config["env_list"]), func(row map[string]any, _ int) (types.KV, bool) {
		key := cast.ToString(row["key"])
		return types.KV{Key: key, Value: cast.ToString(row["value"])}, key != ""
	})
	for _, key := range []string{"env", "environment"} {
		values = append(values, a.environments(cast.ToString(config[key]))...)
	}
	if envFile := cast.ToString(config["env_file"]); envFile != "" {
		if content, err := a.fileContent(ctx, envFile); err == nil {
			values = append(values, a.environments(content)...)
		}
	}
	return values
}

// environments 把 KEY=VALUE 文本解析为环境变量
func (a *baotaAdapter) environments(content string) []types.KV {
	return types.SliceToKV(lo.FilterMap(strings.Split(content, "\n"), func(line string, _ int) (string, bool) {
		line = strings.TrimSpace(line)
		return line, line != "" && !strings.HasPrefix(line, "#") && strings.Contains(line, "=")
	}))
}

func (a *baotaAdapter) SetRunning(ctx context.Context, item types.MigrationItem, running bool) error {
	action := lo.Ternary(running, "start", "stop")
	switch item.Type {
	case "website":
		if item.SourceGroup == "docker" {
			_, err := a.call(ctx, http.MethodPost, "/mod/docker/com/set_site_status", map[string]any{
				"id": item.SourceID, "site_name": item.Name, "status": lo.Ternary(running, 1, 0),
			})
			return err
		}
		path := lo.Ternary(running, "/site?action=SiteStart", "/site?action=SiteStop")
		_, err := a.call(ctx, http.MethodPost, path, map[string]any{"id": item.SourceID, "name": item.Name})
		return err
	case "project":
		_, err := a.call(ctx, http.MethodPost, a.projectPath(item.SourceGroup, action), map[string]any{
			"project_name": item.Name, "id": item.SourceID,
		})
		return err
	default:
		return nil
	}
}

func (a *baotaAdapter) Backup(ctx context.Context, detail *types.MigrationDetail) (string, error) {
	item := detail.Item
	// 面板的网站备份按 ID 查站点表，Docker 网站不在其中，改为直接打包目录
	if item.SourceGroup == "docker" {
		return a.archive(ctx, item.Name, detail.Website.Path)
	}
	switch item.Type {
	case "website", "database":
		return a.panelBackup(ctx, item)
	case "project":
		return a.archive(ctx, item.Name, detail.Project.Path)
	default:
		return "", errors.New(a.t.Get("unsupported migration resource type: %s", item.Type))
	}
}

// panelBackup 触发宝塔自带备份并等待新备份文件生成
func (a *baotaAdapter) panelBackup(ctx context.Context, item types.MigrationItem) (string, error) {
	backupType := lo.Ternary(item.Type == "database", "1", "0")
	existing, err := a.backupFiles(ctx, item.SourceID, backupType)
	if err != nil {
		return "", err
	}
	path := lo.Ternary(item.Type == "database", "/database?action=ToBackup", "/site?action=ToBackup")
	// 网站备份接口是同步的，站点稍大就会超过默认超时
	if _, err = a.callTimeout(ctx, http.MethodPost, path, map[string]any{"id": item.SourceID}, 30*time.Minute); err != nil {
		return "", err
	}

	// 备份为后台任务，轮询备份表直到出现新文件
	for range 600 {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(time.Second):
		}
		current, listErr := a.backupFiles(ctx, item.SourceID, backupType)
		if listErr != nil {
			continue
		}
		for file := range current {
			if !existing[file] {
				return file, nil
			}
		}
	}
	return "", errors.New(a.t.Get("the source backup did not finish in time"))
}

// backupFiles 列出指定资源已有的备份文件
func (a *baotaAdapter) backupFiles(ctx context.Context, sourceID, backupType string) (map[string]bool, error) {
	data, err := a.call(ctx, http.MethodPost, "/data?action=getData", map[string]any{
		"table": "backup", "p": 1, "limit": 100, "search": sourceID, "type": backupType,
	})
	if err != nil {
		return nil, err
	}
	files := make(map[string]bool)
	for _, row := range a.rows(data) {
		if cast.ToString(row["pid"]) != sourceID {
			continue
		}
		// filename 可能是「本地路径|云存储路径」的组合
		if file := cast.ToString(row["local"]); file != "" {
			files[file] = true
		} else if file = strings.Split(cast.ToString(row["filename"]), "|")[0]; file != "" {
			files[file] = true
		}
	}
	return files, nil
}

// archive 打包来源目录为 tar.gz
func (a *baotaAdapter) archive(ctx context.Context, name, path string) (string, error) {
	if path == "" || path == "/" {
		return "", errors.New(a.t.Get("the source path is empty"))
	}
	target := filepath.Join("/www/backup", "acepanel-migration", fmt.Sprintf("%s-%d.tar.gz", a.safeName(name), time.Now().UnixNano()))
	data, err := a.call(ctx, http.MethodPost, "/files?action=ZipAndDownload", map[string]any{
		"path": filepath.Dir(path), "sfile": filepath.Base(path), "dfile": target, "z_type": "tar.gz",
	})
	if err != nil {
		return "", err
	}
	if taskID := cast.ToString(cast.ToStringMap(data)["task_id"]); taskID != "" {
		if err = a.waitTask(ctx, taskID); err != nil {
			return "", err
		}
	}
	return target, nil
}

// safeName 过滤出可安全用于文件名的字符
func (a *baotaAdapter) safeName(name string) string {
	var result strings.Builder
	for _, char := range name {
		switch {
		case char >= 'a' && char <= 'z', char >= 'A' && char <= 'Z', char >= '0' && char <= '9', char == '-', char == '_', char == '.':
			result.WriteRune(char)
		default:
			result.WriteRune('_')
		}
	}
	if result.Len() == 0 {
		return "resource"
	}
	return result.String()
}

// waitTask 等待宝塔后台任务完成
func (a *baotaAdapter) waitTask(ctx context.Context, taskID string) error {
	for range 3600 {
		data, err := a.call(ctx, http.MethodPost, "/task?action=get_task_lists", map[string]any{"task_id": taskID})
		if err == nil {
			if list := a.rows(data); len(list) > 0 {
				switch cast.ToInt(list[0]["status"]) {
				case 1:
					return nil
				case 0, -1: // 排队中或执行中
				default:
					return errors.New(a.t.Get("the source task failed"))
				}
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
	return errors.New(a.t.Get("the source backup did not finish in time"))
}

func (a *baotaAdapter) Download(ctx context.Context, remote, local string, progress types.MigrationProgress) error {
	return a.download(ctx, remote, local, progress)
}

// projectPath 拼接项目接口路径，python 模块的方法名与其他模块不同
func (a *baotaAdapter) projectPath(module, action string) string {
	name := map[string]string{"list": "get_project_list", "start": "start_project", "stop": "stop_project"}[action]
	if module == "python" && action == "list" {
		name = "GetProjectList"
	}
	return fmt.Sprintf("/project/%s/%s/%s", module, name, module)
}

// fileContent 读取来源服务器上的文件内容
func (a *baotaAdapter) fileContent(ctx context.Context, path string) (string, error) {
	data, err := a.call(ctx, http.MethodPost, "/files?action=GetFileBody", map[string]any{"path": path})
	if err != nil {
		return "", err
	}
	return cast.ToString(data), nil
}
