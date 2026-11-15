package data

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/coreos/go-systemd/v22/unit"
	"github.com/leonelquinteros/gotext"
	"github.com/samber/lo"
	"github.com/spf13/cast"
	"gorm.io/gorm"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/pkg/systemctl"
	"github.com/acepanel/panel/v3/pkg/types"
)

type projectRepo struct {
	t  *gotext.Locale
	db *gorm.DB
}

func NewProjectRepo(db *gorm.DB, t *gotext.Locale) biz.ProjectRepo {
	return &projectRepo{
		t:  t,
		db: db,
	}
}

func (r *projectRepo) Count() (int64, error) {
	var count int64
	if err := r.db.Model(&biz.Project{}).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (r *projectRepo) List(typ types.ProjectType, keyword string, page, limit uint) ([]*biz.Project, int64, error) {
	var projects []*biz.Project
	var total int64

	query := r.db.Model(&biz.Project{})
	if typ != "" && typ != "all" {
		query = query.Where("type = ?", typ)
	}
	if keyword != "" {
		query = query.Where("name LIKE ?", "%"+keyword+"%")
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := query.Offset(int((page - 1) * limit)).Limit(int(limit)).Order("id desc").Find(&projects).Error; err != nil {
		return nil, 0, err
	}

	return projects, total, nil
}

func (r *projectRepo) GetEntity(id uint) (*biz.Project, error) {
	project := new(biz.Project)
	if err := r.db.First(project, id).Error; err != nil {
		return nil, err
	}
	return project, nil
}

// ParseDetail 从数据库记录和 systemd unit 文件解析项目详情
func (r *projectRepo) ParseDetail(ctx context.Context, project *biz.Project) (*types.ProjectDetail, error) {
	detail := &types.ProjectDetail{
		ID:      project.ID,
		Name:    project.Name,
		Type:    project.Type,
		RootDir: project.Path,
	}

	// 读取并解析 systemd unit 文件
	unitPath := r.unitFilePath(project.Name)
	file, err := os.Open(unitPath)
	if err != nil {
		if os.IsNotExist(err) {
			return detail, nil
		}
		return nil, err
	}
	defer func(file *os.File) { _ = file.Close() }(file)

	options, err := unit.DeserializeOptions(file)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", r.t.Get("failed to parse systemd config"), err)
	}

	// 解析各个字段
	for _, opt := range options {
		switch opt.Section {
		case "Unit":
			r.parseUnitSection(detail, opt)
		case "Service":
			r.parseServiceSection(detail, opt)
		}
	}

	// 获取运行状态
	if info, err := systemctl.GetServiceInfo(ctx, project.Name); err == nil {
		detail.Status = info.Status
		detail.PID = info.PID
		detail.Memory = info.Memory
		detail.CPU = info.CPU
		detail.Uptime = info.Uptime
	}

	// 获取是否自启动
	if enabled, err := systemctl.IsEnabled(ctx, project.Name); err == nil {
		detail.Enabled = enabled
	}

	return detail, nil
}

// parseUnitSection 解析 [Unit] 部分
func (r *projectRepo) parseUnitSection(detail *types.ProjectDetail, opt *unit.UnitOption) {
	switch opt.Name {
	case "Description":
		detail.Description = opt.Value
	case "Requires":
		detail.Requires = append(detail.Requires, strings.Fields(opt.Value)...)
	case "Wants":
		detail.Wants = append(detail.Wants, strings.Fields(opt.Value)...)
	case "After":
		detail.After = append(detail.After, strings.Fields(opt.Value)...)
	case "Before":
		detail.Before = append(detail.Before, strings.Fields(opt.Value)...)
	}
}

// parseServiceSection 解析 [Service] 部分
func (r *projectRepo) parseServiceSection(detail *types.ProjectDetail, opt *unit.UnitOption) {
	switch opt.Name {
	case "WorkingDirectory":
		detail.WorkingDir = opt.Value
	case "ExecStartPre":
		detail.ExecStartPre = opt.Value
	case "ExecStartPost":
		detail.ExecStartPost = opt.Value
	case "ExecStart":
		detail.ExecStart = opt.Value
	case "ExecStop":
		detail.ExecStop = opt.Value
	case "ExecReload":
		detail.ExecReload = opt.Value
	case "User":
		detail.User = opt.Value
	case "Restart":
		detail.Restart = opt.Value
	case "RestartSec":
		detail.RestartSec = opt.Value
	case "StartLimitBurst":
		if v, err := strconv.Atoi(opt.Value); err == nil {
			detail.RestartMax = v
		}
	case "TimeoutStartSec":
		if v, err := strconv.Atoi(opt.Value); err == nil {
			detail.TimeoutStartSec = v
		}
	case "TimeoutStopSec":
		if v, err := strconv.Atoi(opt.Value); err == nil {
			detail.TimeoutStopSec = v
		}
	case "Environment":
		// 格式: KEY=VALUE
		if kv := r.parseEnvironment(opt.Value); kv != nil {
			detail.Environments = append(detail.Environments, *kv)
		}
	case "StandardOutput":
		detail.StandardOutput = opt.Value
	case "StandardError":
		detail.StandardError = opt.Value
	case "MemoryLimit":
		if v, err := r.parseBytes(opt.Value); err == nil {
			detail.MemoryLimit = v
		}
	case "CPUQuota":
		if v, err := r.parsePercent(opt.Value); err == nil {
			detail.CPUQuota = v
		}
	case "NoNewPrivileges":
		detail.NoNewPrivileges = opt.Value == "true" || opt.Value == "yes"
	case "PrivateTmp", "ProtectTmp":
		detail.ProtectTmp = opt.Value == "true" || opt.Value == "yes"
	case "ProtectHome":
		detail.ProtectHome = opt.Value == "true" || opt.Value == "yes"
	case "ProtectSystem":
		detail.ProtectSystem = opt.Value
	case "ReadWritePaths":
		detail.ReadWritePaths = append(detail.ReadWritePaths, strings.Fields(opt.Value)...)
	case "ReadOnlyPaths":
		detail.ReadOnlyPaths = append(detail.ReadOnlyPaths, strings.Fields(opt.Value)...)
	}
}

// parseEnvironment 解析环境变量，兼容 "KEY=VALUE" 和 KEY="VALUE" 两种引号格式
func (r *projectRepo) parseEnvironment(value string) *types.KV {
	parts := strings.SplitN(r.unquoteEnvironment(value), "=", 2)
	if len(parts) != 2 {
		return nil
	}
	return &types.KV{Key: parts[0], Value: r.unquoteEnvironment(parts[1])}
}

// unquoteEnvironment 去除首尾双引号并反转义
func (r *projectRepo) unquoteEnvironment(s string) string {
	if len(s) >= 2 && strings.HasPrefix(s, `"`) && strings.HasSuffix(s, `"`) {
		s = strings.NewReplacer(`\\`, `\`, `\"`, `"`).Replace(s[1 : len(s)-1])
	}
	return s
}

// parseBytes 解析字节大小 (如 512M, 1G)
func (r *projectRepo) parseBytes(value string) (float64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, errors.New("empty value")
	}

	multiplier := float64(1)
	suffix := value[len(value)-1]
	switch suffix {
	case 'K', 'k':
		multiplier = 1024
		value = value[:len(value)-1]
	case 'M', 'm':
		multiplier = 1024 * 1024
		value = value[:len(value)-1]
	case 'G', 'g':
		multiplier = 1024 * 1024 * 1024
		value = value[:len(value)-1]
	}

	return cast.ToFloat64(value) * multiplier, nil
}

// parsePercent 解析百分比 (如 50%)
func (r *projectRepo) parsePercent(value string) (float64, error) {
	value = strings.TrimSuffix(value, "%")
	return strconv.ParseFloat(value, 64)
}

// NameExists 检查项目名是否已存在
func (r *projectRepo) NameExists(name string) (bool, error) {
	var count int64
	if err := r.db.Model(&biz.Project{}).Where("name = ?", name).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *projectRepo) Create(ctx context.Context, project *biz.Project, req *request.ProjectCreate) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		// 创建数据库记录
		if err := tx.Create(project).Error; err != nil {
			return err
		}

		// 创建项目目录
		if err := os.MkdirAll(project.Path, 0755); err != nil {
			return fmt.Errorf("%s: %w", r.t.Get("failed to create project directory"), err)
		}

		// 生成 systemd unit 文件
		if err := r.generateUnitFile(ctx, req); err != nil {
			return fmt.Errorf("%s: %w", r.t.Get("failed to generate systemd config"), err)
		}

		return nil
	})
}

// generateUnitFile 生成 systemd unit 文件
func (r *projectRepo) generateUnitFile(ctx context.Context, req *request.ProjectCreate) error {
	return r.writeUnit(ctx, req.Name, r.createUnitOptions(req))
}

func (r *projectRepo) createUnitOptions(req *request.ProjectCreate) []*unit.UnitOption {
	options := r.unitOptions(&request.ProjectUpdate{
		Description:  req.Description,
		RootDir:      req.RootDir,
		WorkingDir:   req.WorkingDir,
		ExecStart:    req.ExecStart,
		User:         req.User,
		Restart:      req.Restart,
		Environments: req.Environments,
	}, nil)
	if req.Type == types.ProjectTypeJava {
		// JVM 捕获 SIGTERM 后主动以 143 (128+15) 退出，需视为正常退出
		i := slices.IndexFunc(options, func(opt *unit.UnitOption) bool { return opt.Name == "Restart" })
		options = slices.Insert(options, i+1, unit.NewUnitOption("Service", "SuccessExitStatus", "143"))
	}
	return options
}

func (r *projectRepo) Save(project *biz.Project) error {
	return r.db.Save(project).Error
}

func (r *projectRepo) Delete(project *biz.Project) error {
	return r.db.Delete(project).Error
}

// RenameUnitFile 重命名 systemd unit 文件
func (r *projectRepo) RenameUnitFile(oldName, newName string) error {
	oldPath := r.unitFilePath(oldName)
	newPath := r.unitFilePath(newName)
	if err := os.Rename(oldPath, newPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("%s: %w", r.t.Get("failed to rename systemd config"), err)
	}
	return nil
}

// RemoveUnitFile 删除 systemd unit 文件
func (r *projectRepo) RemoveUnitFile(name string) error {
	unitPath := r.unitFilePath(name)
	if err := os.Remove(unitPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("%s: %w", r.t.Get("failed to delete systemd config"), err)
	}
	return nil
}

// UpdateUnitFile 更新 systemd unit 文件
func (r *projectRepo) UpdateUnitFile(ctx context.Context, name string, req *request.ProjectUpdate) error {
	// 解析失败时直接重写
	var existing []*unit.UnitOption
	if file, err := os.Open(r.unitFilePath(name)); err == nil {
		if parsed, errParse := unit.DeserializeOptions(file); errParse == nil {
			existing = parsed
		}
		_ = file.Close()
	}

	return r.writeUnit(ctx, name, r.unitOptions(req, existing))
}

// unitOptions 生成 unit 配置项，existing 中面板未托管的配置项（如 SuccessExitStatus、LimitNOFILE 等）原样保留
func (r *projectRepo) unitOptions(req *request.ProjectUpdate, existing []*unit.UnitOption) []*unit.UnitOption {
	rootDir := lo.If(!strings.HasPrefix(req.RootDir, "/"), filepath.Join("/", req.RootDir)).Else(req.RootDir)
	workingDir := cmp.Or(req.WorkingDir, rootDir)
	workingDir = lo.If(!strings.HasPrefix(workingDir, "/"), filepath.Join("/", workingDir)).Else(workingDir)

	var options []*unit.UnitOption
	add := func(section, name string, values ...string) {
		for _, value := range values {
			options = append(options, unit.NewUnitOption(section, name, value))
		}
	}
	// 空值不写入
	set := func(name, value string) {
		if value != "" {
			add("Service", name, value)
		}
	}

	add("Unit", "Description", req.Description)
	add("Unit", "Requires", req.Requires...)
	add("Unit", "Wants", req.Wants...)
	add("Unit", "After", req.After...)
	if len(req.After) == 0 {
		add("Unit", "After", "network.target")
	}
	add("Unit", "Before", req.Before...)

	add("Service", "Type", "simple")
	add("Service", "WorkingDirectory", workingDir)
	set("ExecStartPre", req.ExecStartPre)
	set("ExecStart", req.ExecStart)
	set("ExecStartPost", req.ExecStartPost)
	set("ExecStop", req.ExecStop)
	set("ExecReload", req.ExecReload)
	set("User", req.User)
	add("Service", "Restart", cmp.Or(req.Restart, "on-failure"))
	set("RestartSec", req.RestartSec)
	if req.RestartMax > 0 {
		add("Service", "StartLimitBurst", strconv.Itoa(req.RestartMax))
	}
	if req.TimeoutStartSec > 0 {
		add("Service", "TimeoutStartSec", strconv.Itoa(req.TimeoutStartSec))
	}
	if req.TimeoutStopSec > 0 {
		add("Service", "TimeoutStopSec", strconv.Itoa(req.TimeoutStopSec))
	}
	for _, env := range req.Environments {
		add("Service", "Environment", r.quoteEnvironment(env))
	}
	set("StandardOutput", req.StandardOutput)
	set("StandardError", req.StandardError)

	// 资源限制
	if req.MemoryLimit > 0 {
		add("Service", "MemoryLimit", r.formatBytes(req.MemoryLimit))
	}
	set("CPUQuota", req.CPUQuota)

	// 安全选项
	if req.NoNewPrivileges {
		add("Service", "NoNewPrivileges", "true")
	}
	if req.ProtectTmp {
		add("Service", "PrivateTmp", "true")
	}
	if req.ProtectHome {
		add("Service", "ProtectHome", "true")
	}
	set("ProtectSystem", req.ProtectSystem)
	add("Service", "ReadWritePaths", req.ReadWritePaths...)
	add("Service", "ReadOnlyPaths", req.ReadOnlyPaths...)

	add("Install", "WantedBy", "multi-user.target")

	// managedUnitKeys 面板托管的 unit 配置项，更新时整体重写，文件中的其余配置项原样保留
	managedUnitKeys := map[string]map[string]bool{
		"Unit": {
			"Description": true,
			"Requires":    true,
			"Wants":       true,
			"After":       true,
			"Before":      true,
		},
		"Service": {
			"Type":             true,
			"WorkingDirectory": true,
			"ExecStartPre":     true,
			"ExecStart":        true,
			"ExecStartPost":    true,
			"ExecStop":         true,
			"ExecReload":       true,
			"User":             true,
			"Restart":          true,
			"RestartSec":       true,
			"StartLimitBurst":  true,
			"TimeoutStartSec":  true,
			"TimeoutStopSec":   true,
			"Environment":      true,
			"StandardOutput":   true,
			"StandardError":    true,
			"MemoryLimit":      true,
			"CPUQuota":         true,
			"NoNewPrivileges":  true,
			"PrivateTmp":       true,
			"ProtectTmp":       true, // 旧版本写入的无效项，保存时清掉
			"ProtectHome":      true,
			"ProtectSystem":    true,
			"ReadWritePaths":   true,
			"ReadOnlyPaths":    true,
		},
		"Install": {
			"WantedBy": true,
		},
	}
	for _, opt := range existing {
		if !managedUnitKeys[opt.Section][opt.Name] {
			options = append(options, opt)
		}
	}

	// 按段落分组排序，避免序列化时段落交错
	slices.SortStableFunc(options, func(a, b *unit.UnitOption) int {
		return cmp.Compare(r.sectionOrder(a.Section), r.sectionOrder(b.Section))
	})

	return options
}

// quoteEnvironment 序列化环境变量，加双引号以支持值中包含空格
func (r *projectRepo) quoteEnvironment(env types.KV) string {
	value := strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(env.Value)
	return fmt.Sprintf(`"%s=%s"`, env.Key, value)
}

// formatBytes 格式化字节大小
func (r *projectRepo) formatBytes(bytes float64) string {
	b := int64(bytes)
	if b >= 1024*1024*1024 && b%(1024*1024*1024) == 0 {
		return fmt.Sprintf("%dG", b/(1024*1024*1024))
	}
	if b >= 1024*1024 && b%(1024*1024) == 0 {
		return fmt.Sprintf("%dM", b/(1024*1024))
	}
	if b >= 1024 && b%1024 == 0 {
		return fmt.Sprintf("%dK", b/1024)
	}
	return strconv.FormatInt(b, 10)
}

// sectionOrder 返回 unit 文件段落的排序权重
func (r *projectRepo) sectionOrder(section string) int {
	switch section {
	case "Unit":
		return 0
	case "Service":
		return 1
	case "Install":
		return 2
	default:
		return 3
	}
}

func (r *projectRepo) writeUnit(ctx context.Context, name string, options []*unit.UnitOption) error {
	content, err := io.ReadAll(unit.Serialize(options))
	if err != nil {
		return err
	}

	if err = os.WriteFile(r.unitFilePath(name), content, 0644); err != nil {
		return err
	}

	return systemctl.DaemonReload(ctx)
}

// unitFilePath 返回 systemd unit 文件路径
func (r *projectRepo) unitFilePath(name string) string {
	return filepath.Join("/etc/systemd/system", name+".service")
}
