package biz

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
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

// remoteSetting 目标面板的路径配置
type remoteSetting struct {
	WebsitePath string `json:"website_path"`
	ProjectPath string `json:"project_path"`
}

// probeRemote 校验目标面板连通性并读取其版本
func (uc *ToolboxMigrationUsecase) probeRemote(
	ctx context.Context,
	conn *request.ToolboxMigrationConnection,
) (*types.MigrationSource, error) {
	body, err := uc.remote.Request(ctx, conn, "GET", "/api/home/system_info", nil)
	if err != nil {
		return nil, errors.New(uc.t.Get("failed to connect target server: %v", err))
	}
	var response struct {
		Data struct {
			PanelVersion string `json:"panel_version"`
		} `json:"data"`
	}
	if err = json.Unmarshal(body, &response); err != nil {
		return nil, errors.New(uc.t.Get("failed to connect target server: %v", err))
	}
	return &types.MigrationSource{Panel: "acepanel", Version: response.Data.PanelVersion}, nil
}

// localItems 列出本地可推送到目标面板的资源
func (uc *ToolboxMigrationUsecase) localItems(ctx context.Context) ([]types.MigrationItem, error) {
	websites, err := uc.website.All()
	if err != nil {
		return nil, err
	}
	databases, _, err := uc.database.List(ctx, 1, 10000, "", "")
	if err != nil {
		return nil, err
	}
	users, _, err := uc.databaseUser.List(ctx, 1, 10000, "")
	if err != nil {
		return nil, err
	}
	projects, _, err := uc.project.List(ctx, "", "", 1, 10000)
	if err != nil {
		return nil, err
	}

	items := make([]types.MigrationItem, 0, len(websites)+len(databases)+len(users)+len(projects))
	for _, website := range websites {
		items = append(items, types.MigrationItem{
			Key: MigrationItemKey("website", strconv.FormatUint(uint64(website.ID), 10)), Type: "website", Subtype: string(website.Type),
			Name: website.Name, Status: lo.Ternary(website.Status, "running", "stopped"),
			TargetName: website.Name, SourceID: strconv.FormatUint(uint64(website.ID), 10), SourcePath: website.Path,
		})
	}
	for _, database := range databases {
		items = append(items, types.MigrationItem{
			Key: MigrationItemKey("database", database.Server+":"+database.Name), Type: "database", Subtype: string(database.Type),
			Name: database.Name, Status: "running", TargetName: database.Name,
			SourceID: strconv.FormatUint(uint64(database.ServerID), 10), SourceGroup: database.Server,
		})
	}
	for _, user := range users {
		if user.Server == nil {
			continue
		}
		items = append(items, types.MigrationItem{
			Key: MigrationItemKey("database_user", strconv.FormatUint(uint64(user.ID), 10)), Type: "database_user", Subtype: string(user.Server.Type),
			Name: user.Username + "@" + user.Host, Status: "running", TargetName: user.Username,
			SourceID: strconv.FormatUint(uint64(user.ID), 10), SourceGroup: user.Server.Name,
		})
	}
	for _, project := range projects {
		items = append(items, types.MigrationItem{
			Key: MigrationItemKey("project", strconv.FormatUint(uint64(project.ID), 10)), Type: "project", Subtype: string(project.Type),
			Name: project.Name, Status: lo.Ternary(project.Status == "active", "running", "stopped"),
			TargetName: project.Name, SourceID: strconv.FormatUint(uint64(project.ID), 10), SourcePath: project.RootDir,
		})
	}
	return items, nil
}

// push 将本地资源推送到目标面板
func (uc *ToolboxMigrationUsecase) push(
	ctx context.Context,
	conn *request.ToolboxMigrationConnection,
	item types.MigrationItem,
	req *request.ToolboxMigrationStart,
) ([]string, error) {
	switch item.Type {
	case "database":
		return uc.pushDatabase(ctx, conn, item)
	case "database_user":
		return uc.pushDatabaseUser(ctx, conn, item)
	case "website":
		return uc.pushWebsite(ctx, conn, item, req.StopSource)
	case "project":
		return uc.pushProject(ctx, conn, item, req.StopSource)
	default:
		return nil, errors.New(uc.t.Get("unsupported migration resource type: %s", item.Type))
	}
}

// pushDatabase 备份本地数据库并在目标面板恢复
func (uc *ToolboxMigrationUsecase) pushDatabase(
	ctx context.Context,
	conn *request.ToolboxMigrationConnection,
	item types.MigrationItem,
) ([]string, error) {
	backupType := BackupType(item.Subtype)
	if !slices.Contains([]BackupType{BackupTypeMySQL, BackupTypePostgres, BackupTypeClickHouse}, backupType) {
		return nil, errors.New(uc.t.Get("unsupported database type: %s", item.Subtype))
	}
	server, err := uc.databaseServer.FindByName(item.SourceGroup)
	if err != nil {
		return nil, err
	}
	remoteServer, err := uc.remoteDatabaseServer(ctx, conn, server)
	if err != nil {
		return nil, err
	}

	backup, err := uc.createBackup(ctx, backupType, item.Name)
	if err != nil {
		return nil, errors.New(uc.t.Get("database export failed: %v", err))
	}

	if _, err = uc.remote.Request(ctx, conn, "POST", "/api/database", &request.DatabaseCreate{
		ServerID: remoteServer, Name: item.TargetName,
	}); err != nil {
		return nil, errors.New(uc.t.Get("failed to create database on target: %v", err))
	}
	uc.setStage(item.Key, types.MigrationStageTransfer)
	if err = uc.remote.Upload(ctx, conn, backup, backup, uc.transferProgress(item.Name, uc.t.Get("uploading"))); err != nil {
		return nil, errors.New(uc.t.Get("backup transfer failed: %v", err))
	}

	uc.setStage(item.Key, types.MigrationStageImport)
	command := fmt.Sprintf("acepanel restore database -t %s -n %s -f %s",
		strconv.Quote(item.Subtype), strconv.Quote(item.TargetName), strconv.Quote(backup))
	if err = uc.remote.Exec(ctx, conn, command); err != nil {
		return nil, errors.New(uc.t.Get("target import failed: %v", err))
	}
	return nil, nil
}

// pushDatabaseUser 在目标面板重建数据库用户及授权
func (uc *ToolboxMigrationUsecase) pushDatabaseUser(
	ctx context.Context,
	conn *request.ToolboxMigrationConnection,
	item types.MigrationItem,
) ([]string, error) {
	user, err := uc.databaseUser.Get(ctx, cast.ToUint(item.SourceID))
	if err != nil || user.Server == nil {
		return nil, errors.New(uc.t.Get("failed to read database user detail: %v", err))
	}
	remoteServer, err := uc.remoteDatabaseServer(ctx, conn, user.Server)
	if err != nil {
		return nil, err
	}

	uc.setStage(item.Key, types.MigrationStageImport)
	if _, err = uc.remote.Request(ctx, conn, "POST", "/api/database_user", &request.DatabaseUserCreate{
		ServerID: remoteServer, Username: user.Username, Password: user.Password,
		Host: user.Host, Privileges: user.Privileges,
	}); err != nil {
		return nil, errors.New(uc.t.Get("failed to create database user on target: %v", err))
	}
	return nil, nil
}

// pushWebsite 备份本地网站并在目标面板重建
func (uc *ToolboxMigrationUsecase) pushWebsite(
	ctx context.Context,
	conn *request.ToolboxMigrationConnection,
	item types.MigrationItem,
	stopSource bool,
) ([]string, error) {
	id := cast.ToUint(item.SourceID)
	website, err := uc.website.Get(id)
	if err != nil {
		return nil, errors.New(uc.t.Get("failed to read website detail: %v", err))
	}
	// 备份期间停站避免文件不一致，备份落盘后立即恢复
	// 停站与恢复必须成对完成，中途取消会把源站留在停止状态
	statusCtx := context.WithoutCancel(ctx)
	stopped := stopSource && item.Status == "running"
	if stopped {
		_ = uc.website.UpdateStatus(statusCtx, id, false)
	}
	backup, err := uc.createBackup(ctx, BackupTypeWebsite, item.Name)
	if stopped {
		_ = uc.website.UpdateStatus(statusCtx, id, true)
	}
	if err != nil {
		return nil, errors.New(uc.t.Get("website backup failed: %v", err))
	}

	setting, err := uc.remoteSetting(ctx, conn)
	if err != nil {
		return nil, err
	}
	targetPath := filepath.Join(setting.WebsitePath, item.TargetName, "public")

	// 目标尚无证书，建站时只带非 SSL 监听，配置随后整体覆盖
	listens := lo.FilterMap(website.Listens, func(listen webtypes.Listen, _ int) (string, bool) {
		return listen.Address, !slices.Contains(listen.Args, "ssl")
	})
	create := &request.WebsiteCreate{
		Type: website.Type, Name: item.TargetName, Domains: website.Domains, Path: targetPath, PHP: website.PHP,
		Listens: lo.Ternary(len(listens) > 0, listens, []string{"80"}),
	}
	if website.Type == "proxy" && len(website.Proxies) > 0 {
		create.Proxy = website.Proxies[0].Pass
	}
	if _, err = uc.remote.Request(ctx, conn, "POST", "/api/website", create); err != nil {
		return nil, errors.New(uc.t.Get("failed to create website on target: %v", err))
	}
	uc.setStage(item.Key, types.MigrationStageTransfer)
	if err = uc.remote.Upload(ctx, conn, backup, backup, uc.transferProgress(item.Name, uc.t.Get("uploading"))); err != nil {
		return nil, errors.New(uc.t.Get("backup transfer failed: %v", err))
	}

	uc.setStage(item.Key, types.MigrationStageImport)
	command := fmt.Sprintf("acepanel restore website -n %s -f %s", strconv.Quote(item.TargetName), strconv.Quote(backup))
	if err = uc.remote.Exec(ctx, conn, command); err != nil {
		return nil, errors.New(uc.t.Get("target import failed: %v", err))
	}

	remoteID, err := uc.remoteWebsiteID(ctx, conn, item.TargetName)
	if err != nil {
		return nil, err
	}
	update := &request.WebsiteUpdate{
		ID: remoteID, Listens: website.Listens, Domains: website.Domains,
		Path: targetPath, Root: uc.relocate(website.Root, website.Path, targetPath), Index: website.Index,
		SSL: website.SSL, SSLCert: website.SSLCert, SSLKey: website.SSLKey, SSLProtocols: website.SSLProtocols,
		HSTS: website.HSTS, OCSP: website.OCSP, HTTPRedirect: website.HTTPRedirect,
		PHP: website.PHP, Rewrite: website.Rewrite, OpenBasedir: website.OpenBasedir,
		Upstreams: website.Upstreams, Proxies: website.Proxies, Redirects: website.Redirects,
		StatEnabled: website.StatEnabled, RateLimit: website.RateLimit, RealIP: website.RealIP, BasicAuth: website.BasicAuth,
	}
	if _, err = uc.remote.Request(ctx, conn, "PUT", fmt.Sprintf("/api/website/%d", remoteID), update); err != nil {
		return nil, errors.New(uc.t.Get("failed to rebuild website configuration on target: %v", err))
	}
	if item.Status != "running" {
		_, _ = uc.remote.Request(ctx, conn, "POST", fmt.Sprintf("/api/website/%d/status", remoteID),
			&request.WebsiteUpdateStatus{ID: remoteID, Status: false})
	}
	return nil, nil
}

// pushProject 打包本地项目目录并在目标面板重建
func (uc *ToolboxMigrationUsecase) pushProject(
	ctx context.Context,
	conn *request.ToolboxMigrationConnection,
	item types.MigrationItem,
	stopSource bool,
) ([]string, error) {
	project, err := uc.project.Get(ctx, cast.ToUint(item.SourceID))
	if err != nil {
		return nil, errors.New(uc.t.Get("failed to read project detail: %v", err))
	}
	setting, err := uc.remoteSetting(ctx, conn)
	if err != nil {
		return nil, err
	}
	targetPath := lo.CoalesceOrEmpty(item.TargetPath, filepath.Join(setting.ProjectPath, item.TargetName))

	tmpDir, err := uc.archive.TempDir()
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// 打包期间停服避免文件不一致，打包完成后立即恢复
	stopped := stopSource && project.Status == "active"
	if stopped {
		_ = systemctl.Stop(ctx, item.Name)
	}
	archive := filepath.Join(tmpDir, item.TargetName+".tar.gz")
	err = uc.archive.Compress(ctx, project.RootDir, archive)
	if stopped {
		_ = systemctl.Start(ctx, item.Name)
	}
	if err != nil {
		return nil, errors.New(uc.t.Get("project backup failed: %v", err))
	}

	create := &request.ProjectCreate{
		Name: item.TargetName, Type: project.Type, Description: project.Description,
		RootDir: targetPath, WorkingDir: uc.relocate(project.WorkingDir, project.RootDir, targetPath),
		ExecStart: strings.ReplaceAll(project.ExecStart, project.RootDir, targetPath),
		User:      lo.CoalesceOrEmpty(item.TargetUser, project.User), Restart: project.Restart,
		Environments: project.Environments,
	}
	if _, err = uc.remote.Request(ctx, conn, "POST", "/api/project", create); err != nil {
		return nil, errors.New(uc.t.Get("failed to create project on target: %v", err))
	}
	uc.setStage(item.Key, types.MigrationStageTransfer)
	if err = uc.remote.Upload(ctx, conn, archive, archive, uc.transferProgress(item.Name, uc.t.Get("uploading"))); err != nil {
		return nil, errors.New(uc.t.Get("backup transfer failed: %v", err))
	}

	uc.setStage(item.Key, types.MigrationStageImport)
	command := fmt.Sprintf("mkdir -p %s && tar xf %s --overwrite -C %s && rm -f %s",
		strconv.Quote(targetPath), strconv.Quote(archive), strconv.Quote(targetPath), strconv.Quote(archive))
	if err = uc.remote.Exec(ctx, conn, command); err != nil {
		return nil, errors.New(uc.t.Get("target import failed: %v", err))
	}
	return nil, nil
}

// createBackup 在本地生成备份并返回文件路径
func (uc *ToolboxMigrationUsecase) createBackup(ctx context.Context, typ BackupType, target string) (string, error) {
	if err := uc.backup.Create(ctx, typ, target, 0); err != nil {
		return "", err
	}
	files, err := uc.backup.List(typ)
	if err != nil {
		return "", err
	}
	// 备份文件名为 {target}_{时间戳}，取最新的一个
	files = lo.Filter(files, func(file *types.BackupFile, _ int) bool { return strings.HasPrefix(file.Name, target+"_") })
	if len(files) == 0 {
		return "", errors.New(uc.t.Get("backup file not exists"))
	}
	newest := lo.MaxBy(files, func(a, b *types.BackupFile) bool { return a.Time.After(b.Time) })
	return newest.Path, nil
}

// remoteSetting 读取目标面板的路径配置
func (uc *ToolboxMigrationUsecase) remoteSetting(
	ctx context.Context,
	conn *request.ToolboxMigrationConnection,
) (*remoteSetting, error) {
	body, err := uc.remote.Request(ctx, conn, "GET", "/api/setting", nil)
	if err != nil {
		return nil, errors.New(uc.t.Get("failed to read target settings: %v", err))
	}
	var response struct {
		Data remoteSetting `json:"data"`
	}
	if err = json.Unmarshal(body, &response); err != nil {
		return nil, errors.New(uc.t.Get("failed to read target settings: %v", err))
	}
	setting := response.Data
	if setting.WebsitePath == "" {
		setting.WebsitePath = filepath.Join(app.Root, "sites")
	}
	if setting.ProjectPath == "" {
		setting.ProjectPath = filepath.Join(app.Root, "projects")
	}
	return &setting, nil
}

// remoteDatabaseServer 查找目标面板上同名同类型的数据库服务器
func (uc *ToolboxMigrationUsecase) remoteDatabaseServer(
	ctx context.Context,
	conn *request.ToolboxMigrationConnection,
	server *DatabaseServer,
) (uint, error) {
	body, err := uc.remote.Request(ctx, conn, "GET", "/api/database_server", map[string]any{"page": 1, "limit": 10000})
	if err != nil {
		return 0, errors.New(uc.t.Get("failed to read target database servers: %v", err))
	}
	var response struct {
		Data struct {
			Items []struct {
				ID   uint         `json:"id"`
				Name string       `json:"name"`
				Type DatabaseType `json:"type"`
			} `json:"items"`
		} `json:"data"`
	}
	if err = json.Unmarshal(body, &response); err != nil {
		return 0, errors.New(uc.t.Get("failed to read target database servers: %v", err))
	}
	index := slices.IndexFunc(response.Data.Items, func(item struct {
		ID   uint         `json:"id"`
		Name string       `json:"name"`
		Type DatabaseType `json:"type"`
	}) bool {
		return item.Name == server.Name && item.Type == server.Type
	})
	if index < 0 {
		return 0, errors.New(uc.t.Get("no matching database server found on target"))
	}
	return response.Data.Items[index].ID, nil
}

// remoteWebsiteID 查找目标面板上新建网站的 ID
func (uc *ToolboxMigrationUsecase) remoteWebsiteID(
	ctx context.Context,
	conn *request.ToolboxMigrationConnection,
	name string,
) (uint, error) {
	body, err := uc.remote.Request(ctx, conn, "GET", "/api/website", map[string]any{"type": "all", "page": 1, "limit": 10000})
	if err != nil {
		return 0, errors.New(uc.t.Get("failed to read the created website on target: %v", err))
	}
	var response struct {
		Data struct {
			Items []struct {
				ID   uint   `json:"id"`
				Name string `json:"name"`
			} `json:"items"`
		} `json:"data"`
	}
	if err = json.Unmarshal(body, &response); err != nil {
		return 0, errors.New(uc.t.Get("failed to read the created website on target: %v", err))
	}
	for _, item := range response.Data.Items {
		if item.Name == name {
			return item.ID, nil
		}
	}
	return 0, errors.New(uc.t.Get("the created website could not be found on target"))
}

// MigrationItemKey 生成前端选择用的资源标识
func MigrationItemKey(typ, id string) string {
	return typ + ":" + base64.RawURLEncoding.EncodeToString([]byte(id))
}
