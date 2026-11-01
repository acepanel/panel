package biz

import (
	"cmp"
	"context"
	"errors"
	"os"
	"os/user"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/samber/lo"
	"github.com/spf13/cast"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/pkg/systemctl"
	"github.com/acepanel/panel/v3/pkg/types"
	webtypes "github.com/acepanel/panel/v3/pkg/webserver/types"
)

// checkConflicts 标注目标侧已存在或不受支持的资源
func (uc *ToolboxMigrationUsecase) checkConflicts(ctx context.Context, items []types.MigrationItem) {
	websitePath, _ := uc.setting.Get(SettingKeyWebsitePath, filepath.Join(app.Root, "sites"))
	projectPath, _ := uc.setting.Get(SettingKeyProjectPath, filepath.Join(app.Root, "projects"))
	projects, _, _ := uc.project.List(ctx, "", "", 1, 10000)
	databases, _, _ := uc.database.List(ctx, 1, 10000, "", "")
	servers, _, _ := uc.databaseServer.List(ctx, 1, 10000, "")

	for i := range items {
		item := &items[i]
		switch item.Type {
		case "website":
			item.TargetPath = filepath.Join(websitePath, item.TargetName, "public")
			if item.TargetName == "default" || item.TargetName == "phpmyadmin" {
				item.Blockers = append(item.Blockers, uc.t.Get("the name is reserved by AcePanel"))
			} else if !uc.resourceName.MatchString(item.TargetName) {
				item.Blockers = append(item.Blockers, uc.t.Get("the name contains characters not allowed by AcePanel"))
			}
			if _, err := uc.website.GetByName(item.TargetName); err == nil {
				item.Blockers = append(item.Blockers, uc.t.Get("a website with the same name already exists on the target server"))
			}
			// 版本不一致不阻断迁移，导入时会退到最接近的已装版本，这里提前告知
			parts := uc.versionParts(item.Version)
			if item.Subtype == "php" && len(parts) >= 2 {
				if _, warning := uc.resolvePHP(uint(parts[0]*10 + parts[1])); warning != "" {
					item.Warnings = append(item.Warnings, warning)
				}
			}
		case "database":
			server := "local_" + item.Subtype
			if item.Subtype == "mariadb" {
				server = "local_mysql"
			}
			nameRegex := uc.postgresName
			if item.Subtype != "postgresql" {
				nameRegex = uc.mysqlName
			}
			if !nameRegex.MatchString(item.TargetName) {
				item.Blockers = append(item.Blockers, uc.t.Get("the name contains characters not allowed by AcePanel"))
			}
			index := slices.IndexFunc(servers, func(candidate *DatabaseServer) bool { return candidate.Name == server })
			if index < 0 {
				item.Blockers = append(item.Blockers, uc.t.Get("the target server does not have a compatible %s database server", item.Subtype))
				continue
			}
			if slices.ContainsFunc(databases, func(database *Database) bool {
				return database.ServerID == servers[index].ID && database.Name == item.TargetName
			}) {
				item.Blockers = append(item.Blockers, uc.t.Get("a database with the same name already exists on the target server"))
			}
		case "database_user":
			if !uc.mysqlName.MatchString(item.TargetName) {
				item.Blockers = append(item.Blockers, uc.t.Get("the name contains characters not allowed by AcePanel"))
			}
		case "project":
			item.TargetPath = filepath.Join(projectPath, item.TargetName)
			if !uc.resourceName.MatchString(item.TargetName) {
				item.Blockers = append(item.Blockers, uc.t.Get("the name contains characters not allowed by AcePanel"))
			}
			if slices.ContainsFunc(projects, func(project *types.ProjectDetail) bool { return project.Name == item.TargetName }) {
				item.Blockers = append(item.Blockers, uc.t.Get("a project with the same name already exists on the target server"))
			}
			if _, ok := uc.runtimeSlug(ctx, types.ProjectType(item.Subtype), item.Version); !ok {
				item.Blockers = append(item.Blockers, uc.t.Get(
					"the target server does not have a compatible %s runtime for version %s",
					item.Subtype, lo.CoalesceOrEmpty(item.Version, uc.t.Get("unknown")),
				))
			}
		}
	}
}

// importDatabase 创建目标数据库并导入备份
func (uc *ToolboxMigrationUsecase) importDatabase(ctx context.Context, detail *types.MigrationDetail, archive string) ([]string, error) {
	database := detail.Database
	targetType := database.Type
	if targetType == "mariadb" {
		targetType = "mysql"
	}
	server, err := uc.databaseServer.GetByName(ctx, "local_"+targetType)
	if err != nil {
		return nil, errors.New(uc.t.Get("no compatible database server is installed on the target"))
	}

	// PostgreSQL 不支持从高版本导入低版本
	if targetType == "postgresql" {
		if installed, installErr := uc.app.GetInstalled("postgresql"); installErr == nil {
			if cast.ToInt(uc.version.FindString(database.Version)) > cast.ToInt(uc.version.FindString(installed.Version)) {
				return nil, errors.New(uc.t.Get("the source PostgreSQL major version is newer than the target version"))
			}
		}
	}

	create := &request.DatabaseCreate{ServerID: server.ID, Name: database.Name}
	warnings := make([]string, 0)
	switch {
	case database.Username != "" && database.Password != "":
		create.CreateUser = !uc.databaseUserExists(ctx, server.ID, database.Username)
		create.Username = database.Username
		create.Password = database.Password
		create.Host = lo.CoalesceOrEmpty(database.Host, "localhost")
	case database.Username != "":
		warnings = append(warnings, uc.t.Get(
			"the database was imported but the source password was unavailable; reset the database user password and update the website configuration manually",
		))
	}
	if database.Type == "mariadb" {
		warnings = append(warnings, uc.t.Get("the source is MariaDB and the target is MySQL"))
	}
	if err = uc.database.Create(ctx, create); err != nil {
		return warnings, err
	}

	backupType := BackupTypeMySQL
	if targetType == "postgresql" {
		backupType = BackupTypePostgres
	}
	if err = uc.backup.Restore(ctx, backupType, archive, database.Name); err != nil {
		return warnings, errors.New(uc.t.Get("database import failed: %v", err))
	}
	return warnings, nil
}

// importDatabaseUser 在目标创建数据库用户
func (uc *ToolboxMigrationUsecase) importDatabaseUser(ctx context.Context, detail *types.MigrationDetail) ([]string, error) {
	user := detail.DatabaseUser
	targetType := user.Type
	if targetType == "mariadb" {
		targetType = "mysql"
	}
	server, err := uc.databaseServer.GetByName(ctx, "local_"+targetType)
	if err != nil {
		return nil, errors.New(uc.t.Get("no compatible database server is installed on the target"))
	}

	if err = uc.databaseUser.Create(ctx, &request.DatabaseUserCreate{
		ServerID: server.ID, Username: detail.Item.TargetName,
		Password: user.Password, Host: user.Host,
	}); err != nil {
		return nil, errors.New(uc.t.Get("failed to create database user on target: %v", err))
	}
	return nil, nil
}

// databaseUserExists 目标该数据库服务上是否已有同名用户
func (uc *ToolboxMigrationUsecase) databaseUserExists(ctx context.Context, serverID uint, username string) bool {
	users, _, err := uc.databaseUser.List(ctx, 1, 10000, "")
	if err != nil {
		return false
	}

	return slices.ContainsFunc(users, func(user *DatabaseUser) bool {
		return user.ServerID == serverID && user.Username == username
	})
}

// importWebsite 创建目标网站、导入文件并还原配置
func (uc *ToolboxMigrationUsecase) importWebsite(ctx context.Context, detail *types.MigrationDetail, archive string) ([]string, error) {
	website := detail.Website
	targetPath := detail.Item.TargetPath
	if !uc.archive.IsEmpty(targetPath) {
		return nil, errors.New(uc.t.Get("the target website directory is not empty"))
	}

	domains := slices.DeleteFunc(slices.Clone(website.Domains), func(domain string) bool { return domain == "" })
	if len(domains) == 0 {
		domains = []string{detail.Item.Name}
	}
	// 建站阶段目标尚无证书，先只监听非 SSL 端口
	listens := lo.Filter(website.Listens, func(listen string, _ int) bool {
		return listen != "" && !slices.Contains(website.SSLListens, listen)
	})
	if len(listens) == 0 {
		listens = []string{"80"}
	}

	create := &request.WebsiteCreate{
		Type: website.Type, Name: detail.Item.TargetName, Listens: listens,
		Domains: domains, Path: targetPath, PHP: website.PHP,
	}
	var warnings []string
	switch website.Type {
	case "php":
		php, warning := uc.resolvePHP(website.PHP)
		if warning != "" {
			warnings = append(warnings, warning)
		}
		website.PHP, create.PHP = php, php
	case "proxy":
		if len(website.Proxies) == 0 || website.Proxies[0].Pass == "" {
			return nil, errors.New(uc.t.Get("the source reverse proxy target could not be determined"))
		}
		create.Proxy = website.Proxies[0].Pass
	}
	created, createWarnings, err := uc.website.Create(ctx, create)
	if err != nil {
		return nil, err
	}
	warnings = append(warnings, createWarnings...)
	// 走备份还原而非直接解包，那里已兼容各家面板导出的归档结构
	if err = uc.backup.Restore(ctx, BackupTypeWebsite, archive, detail.Item.TargetName); err != nil {
		return nil, errors.New(uc.t.Get("website file import failed: %v", err))
	}

	update := &request.WebsiteUpdate{
		ID: created.ID, Listens: uc.websiteListens(website), Domains: domains,
		Path: targetPath, Root: uc.relocate(website.Root, website.Path, targetPath),
		Index: lo.Ternary(len(website.Index) > 0, website.Index, []string{"index.php", "index.html", "index.htm"}),
		SSL:   website.SSL, SSLCert: website.SSLCert, SSLKey: website.SSLKey, SSLProtocols: website.SSLProtocols,
		HSTS: website.HSTS, OCSP: website.OCSP, HTTPRedirect: website.HTTPRedirect,
		PHP: website.PHP, Rewrite: website.Rewrite, OpenBasedir: website.OpenBasedir,
	}
	for _, proxy := range website.Proxies {
		update.Proxies = append(update.Proxies, webtypes.Proxy{
			Location: lo.CoalesceOrEmpty(proxy.Location, "/"), Pass: proxy.Pass, Host: lo.CoalesceOrEmpty(proxy.Host, webtypes.DefaultProxyHost(proxy.Pass, nil)),
			HTTPVersion: "1.1", Replaces: proxy.Replaces,
		})
	}
	for _, redirect := range website.Redirects {
		update.Redirects = append(update.Redirects, webtypes.Redirect{
			Type: webtypes.RedirectType(redirect.Type), From: redirect.From, To: redirect.To,
			KeepURI: redirect.KeepURI, StatusCode: redirect.StatusCode,
		})
	}
	if err = uc.website.Update(ctx, update); err != nil {
		return nil, errors.New(uc.t.Get("website configuration import failed: %v", err))
	}
	if website.Remark != "" {
		_ = uc.website.UpdateRemark(created.ID, website.Remark)
	}
	if website.ExpireAt != nil {
		_ = uc.website.UpdateExpireAt(created.ID, website.ExpireAt)
	}
	if !website.Enabled {
		_ = uc.website.UpdateStatus(ctx, created.ID, false)
	}
	return warnings, nil
}

// resolvePHP 目标缺少来源所用的 PHP 版本时退到最接近的已装版本，一个都没装则不启用 PHP
func (uc *ToolboxMigrationUsecase) resolvePHP(version uint) (uint, string) {
	installed := lo.FilterMap(uc.environment.InstalledSlugs("php"), func(slug string, _ int) (uint, bool) {
		return cast.ToUint(slug), cast.ToUint(slug) > 0
	})
	switch {
	case len(installed) == 0:
		return 0, uc.t.Get("the target server has no PHP installed, the website was created without PHP")
	case slices.Contains(installed, version):
		return version, ""
	}

	distance := func(installed uint) uint {
		if installed > version {
			return installed - version
		}
		return version - installed
	}
	// 版本号相差最小的优先，相差一样时取更高的版本
	closest := slices.MinFunc(installed, func(a, b uint) int {
		if diff := cmp.Compare(distance(a), distance(b)); diff != 0 {
			return diff
		}
		return cmp.Compare(b, a)
	})

	return closest, uc.t.Get("the source uses PHP %d, the target does not have it installed, PHP %d was used instead", version, closest)
}

// websiteListens 还原监听端口，SSL 端口带 ssl 参数
func (uc *ToolboxMigrationUsecase) websiteListens(website *types.MigrationWebsite) []webtypes.Listen {
	listens := make([]webtypes.Listen, 0, len(website.Listens))
	for _, address := range website.Listens {
		if address == "" {
			continue
		}
		listen := webtypes.Listen{Address: address}
		if website.SSL && slices.Contains(website.SSLListens, address) {
			listen.Args = []string{"ssl"}
		}
		listens = append(listens, listen)
	}
	if len(listens) == 0 {
		listens = append(listens, webtypes.Listen{Address: "80"})
	}
	return listens
}

// importProject 还原项目文件并创建 systemd 服务
func (uc *ToolboxMigrationUsecase) importProject(ctx context.Context, detail *types.MigrationDetail, archive string) ([]string, error) {
	project := detail.Project
	if strings.TrimSpace(project.ExecStart) == "" {
		return nil, errors.New(uc.t.Get("the source project start command is missing"))
	}
	slug, ok := uc.runtimeSlug(ctx, project.Type, project.Version)
	if !ok {
		return nil, errors.New(uc.t.Get(
			"the target server does not have a compatible %s runtime for version %s",
			project.Type, lo.CoalesceOrEmpty(project.Version, uc.t.Get("unknown")),
		))
	}
	runUser, err := uc.projectUser(lo.CoalesceOrEmpty(detail.Item.TargetUser, project.User))
	if err != nil {
		return nil, err
	}
	targetPath := detail.Item.TargetPath
	if !uc.archive.IsEmpty(targetPath) {
		return nil, errors.New(uc.t.Get("the target project directory is not empty"))
	}
	if err = uc.restoreFiles(ctx, archive, targetPath); err != nil {
		return nil, err
	}
	// 依赖目录与目标运行时不兼容，需要在目标重新安装
	uc.removeDependencies(targetPath, project.Type)

	create := &request.ProjectCreate{
		Name: detail.Item.TargetName, Type: project.Type, Description: uc.t.Get("Migrated from %s", detail.Item.Name),
		RootDir: targetPath, WorkingDir: uc.relocate(project.WorkingDir, project.Path, targetPath),
		ExecStart: uc.rewriteExecStart(project, targetPath, slug), User: runUser, Restart: "on-failure",
		Environments: project.Environments,
	}
	if _, err = uc.project.Create(ctx, create); err != nil {
		return nil, err
	}
	warnings := uc.projectWebsite(ctx, detail, project)

	// Node.js / Python 依赖未随文件迁移，保持停止状态待人工处理
	switch project.Type {
	case types.ProjectTypeNodejs:
		return append(warnings, uc.t.Get("Node.js dependencies were not installed; install dependencies and start the project manually")), nil
	case types.ProjectTypePython:
		return append(warnings, uc.t.Get(
			"the source Python virtual environment was not migrated; create an environment, install dependencies, and start the project manually",
		)), nil
	case types.ProjectTypeGeneral:
		return append(warnings, uc.t.Get("the project was not started; start it manually")), nil
	default:
		// 其余类型依赖随文件一起迁移，可直接启动
	}
	if project.Enabled {
		_ = systemctl.Enable(ctx, detail.Item.TargetName)
	}
	if project.Running {
		_ = systemctl.Start(ctx, detail.Item.TargetName)
	}
	return warnings, nil
}

// projectWebsite 为带域名的项目创建反向代理网站
func (uc *ToolboxMigrationUsecase) projectWebsite(ctx context.Context, detail *types.MigrationDetail, project *types.MigrationProject) []string {
	if project.Port == 0 || len(project.Domains) == 0 {
		return nil
	}
	if _, err := uc.website.GetByName(detail.Item.TargetName); err == nil {
		return []string{uc.t.Get("the project was migrated, but its reverse proxy website already exists on the target")}
	}
	root, _ := uc.setting.Get(SettingKeyWebsitePath, filepath.Join(app.Root, "sites"))
	website, warnings, err := uc.website.Create(ctx, &request.WebsiteCreate{
		Type: "proxy", Name: detail.Item.TargetName, Domains: project.Domains,
		Listens: lo.Ternary(len(project.Listens) > 0, project.Listens, []string{"80"}),
		Path:    filepath.Join(root, detail.Item.TargetName, "public"),
		Proxy:   "http://127.0.0.1:" + strconv.FormatUint(uint64(project.Port), 10),
	})
	if err != nil {
		return []string{uc.t.Get("the project was migrated, but its reverse proxy website could not be created: %v", err)}
	}
	if !project.Running {
		_ = uc.website.UpdateStatus(ctx, website.ID, false)
	}
	return warnings
}

// rewriteExecStart 将启动命令中的来源路径与运行时可执行文件替换为目标侧
func (uc *ToolboxMigrationUsecase) rewriteExecStart(project *types.MigrationProject, targetPath, slug string) string {
	execStart := strings.ReplaceAll(project.ExecStart, project.Path, targetPath)
	if slug == "" {
		return execStart
	}
	command, arguments, _ := strings.Cut(strings.TrimLeft(execStart, " \t"), " ")
	executable := ""
	switch base := filepath.Base(strings.Trim(command, `"'`)); project.Type {
	case types.ProjectTypeGo, types.ProjectTypeJava, types.ProjectTypeDotnet, types.ProjectTypePHP:
		if base == string(project.Type) || base == "java" && project.Type == types.ProjectTypeJava {
			executable = base + slug
		}
	case types.ProjectTypePython:
		if base == "python" || base == "python3" {
			executable = "python" + slug
		}
	case types.ProjectTypeNodejs:
		switch base {
		case "node":
			executable = "node" + slug
		case "npm", "npx", "corepack":
			executable = filepath.Join(app.Root, "server", "nodejs", slug, "bin", base)
		}
	default:
		// 通用类型没有面板托管的运行时，不改写启动命令
	}
	if executable == "" {
		return execStart
	}
	return strings.TrimSpace(executable + " " + arguments)
}

// runtimeSlug 匹配目标侧兼容的运行时，返回其 slug
func (uc *ToolboxMigrationUsecase) runtimeSlug(ctx context.Context, typ types.ProjectType, version string) (string, bool) {
	if typ == types.ProjectTypeGeneral || typ == "" {
		return "", true
	}
	installed := uc.environment.InstalledSlugs(string(typ))
	if len(installed) == 0 {
		return "", false
	}
	if strings.TrimSpace(version) == "" {
		return installed[0], true
	}

	// Node.js 与 Java 只比较主版本，其余比较主次版本
	parts := 2
	if typ == types.ProjectTypeNodejs || typ == types.ProjectTypeJava {
		parts = 1
	}
	source := uc.javaVersion(typ, uc.versionParts(version))
	if len(source) < parts {
		return "", false
	}
	for _, slug := range installed {
		target := uc.javaVersion(typ, uc.versionParts(uc.environment.InstalledVersion(ctx, string(typ), slug)))
		if len(target) >= parts && slices.Equal(source[:parts], target[:parts]) {
			return slug, true
		}
	}
	return "", false
}

// projectUser 校验并映射项目运行用户
func (uc *ToolboxMigrationUsecase) projectUser(name string) (string, error) {
	switch name {
	case "", "www-data", "nginx", "apache":
		return "www", nil
	case "www", "root":
		return name, nil
	}
	if _, err := user.Lookup(name); err != nil {
		return "", errors.New(uc.t.Get("the target server does not have user %s", name))
	}
	return name, nil
}

// restoreFiles 解包备份并复制到目标目录
func (uc *ToolboxMigrationUsecase) restoreFiles(ctx context.Context, archive, target string) error {
	staging, err := uc.archive.TempDir()
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(staging) }()
	content, err := uc.archive.Extract(ctx, archive, staging)
	if err != nil {
		return err
	}
	return uc.archive.CopyTree(ctx, content, target)
}

// removeDependencies 删除来源侧的依赖目录
func (uc *ToolboxMigrationUsecase) removeDependencies(root string, typ types.ProjectType) {
	names := map[types.ProjectType][]string{
		types.ProjectTypeNodejs: {"node_modules"},
		types.ProjectTypePython: {".venv", "venv", "virtualenv"},
	}[typ]
	if len(names) == 0 {
		return
	}
	_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || !entry.IsDir() || path == root || !slices.Contains(names, entry.Name()) {
			return nil //nolint:nilerr
		}
		_ = os.RemoveAll(path) //nolint:gosec
		return filepath.SkipDir
	})
}

// relocate 将来源路径按根目录映射到目标路径
func (uc *ToolboxMigrationUsecase) relocate(value, sourceRoot, targetRoot string) string {
	if value == "" || sourceRoot == "" {
		return targetRoot
	}
	value, sourceRoot = filepath.Clean(value), filepath.Clean(sourceRoot)
	if value == sourceRoot {
		return targetRoot
	}
	if relative, ok := strings.CutPrefix(value, sourceRoot+string(os.PathSeparator)); ok {
		return filepath.Join(targetRoot, relative)
	}
	return targetRoot
}

// versionParts 提取版本号中的数字段
func (uc *ToolboxMigrationUsecase) versionParts(version string) []int {
	return lo.Map(uc.version.FindAllString(version, 3), func(part string, _ int) int { return cast.ToInt(part) })
}

// javaVersion 归一化 Java 的 1.8 式版本号
func (uc *ToolboxMigrationUsecase) javaVersion(typ types.ProjectType, parts []int) []int {
	if typ == types.ProjectTypeJava && len(parts) > 1 && parts[0] == 1 {
		return parts[1:]
	}
	return parts
}
