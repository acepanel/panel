package data

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/coreos/go-systemd/v22/unit"
	"github.com/leonelquinteros/gotext"
	"github.com/samber/lo"
	"github.com/spf13/cast"
	"gorm.io/gorm"

	"github.com/acepanel/panel/internal/biz"
	"github.com/acepanel/panel/internal/http/request"
	"github.com/acepanel/panel/pkg/types"
)

type projectRepo struct {
	t   *gotext.Locale
	db  *gorm.DB
	log *slog.Logger
}

func NewProjectRepo(t *gotext.Locale, db *gorm.DB, log *slog.Logger) biz.ProjectRepo {
	return &projectRepo{
		t:   t,
		db:  db,
		log: log,
	}
}

func (r *projectRepo) List(typ types.ProjectType, page, limit uint) ([]*types.ProjectDetail, int64, error) {
	var projects []*biz.Project
	var total int64

	query := r.db.Model(&biz.Project{})
	if typ != "" && typ != "all" {
		query = query.Where("type = ?", typ)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := query.Offset(int((page - 1) * limit)).Limit(int(limit)).Order("id desc").Find(&projects).Error; err != nil {
		return nil, 0, err
	}

	details := make([]*types.ProjectDetail, 0, len(projects))
	for _, p := range projects {
		detail, err := r.parseProjectDetail(p)
		if err != nil {
			// 如果解析失败，返回基本信息
			detail = &types.ProjectDetail{
				ID:   p.ID,
				Name: p.Name,
				Type: p.Type,
			}
		}
		details = append(details, detail)
	}

	return details, total, nil
}

func (r *projectRepo) Get(id uint) (*types.ProjectDetail, error) {
	project := new(biz.Project)
	if err := r.db.First(project, id).Error; err != nil {
		return nil, err
	}
	return r.parseProjectDetail(project)
}

func (r *projectRepo) Create(ctx context.Context, req *request.ProjectCreate) (*types.ProjectDetail, error) {
	// 检查项目名是否已存在
	var count int64
	if err := r.db.Model(&biz.Project{}).Where("name = ?", req.Name).Count(&count).Error; err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, errors.New(r.t.Get("project name already exists"))
	}

	project := &biz.Project{
		Name: req.Name,
		Type: req.Type,
		Path: req.RootDir,
	}

	err := r.db.Transaction(func(tx *gorm.DB) error {
		// 创建数据库记录
		if err := tx.Create(project).Error; err != nil {
			return err
		}

		// 生成 systemd unit 文件
		if err := r.generateUnitFile(project.ID, req); err != nil {
			return fmt.Errorf("%s: %w", r.t.Get("failed to generate systemd config"), err)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	// 记录日志
	r.log.Info("project created", slog.String("type", biz.OperationTypeProject), slog.Uint64("operator_id", getOperatorID(ctx)), slog.String("name", req.Name), slog.String("project_type", string(req.Type)))

	return r.parseProjectDetail(project)
}

func (r *projectRepo) Update(ctx context.Context, req *request.ProjectUpdate) error {
	project := new(biz.Project)
	if err := r.db.First(project, req.ID).Error; err != nil {
		return err
	}

	// 如果名称变更，需要重命名 unit 文件
	if req.Name != project.Name {
		oldPath := r.unitFilePath(project.Name)
		newPath := r.unitFilePath(req.Name)
		if err := os.Rename(oldPath, newPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("%s: %w", r.t.Get("failed to rename systemd config"), err)
		}
		project.Name = req.Name
	}

	project.Path = req.RootDir
	if err := r.db.Save(project).Error; err != nil {
		return err
	}

	// 记录日志
	r.log.Info("project updated", slog.String("type", biz.OperationTypeProject), slog.Uint64("operator_id", getOperatorID(ctx)), slog.Uint64("id", uint64(req.ID)), slog.String("name", project.Name))

	// 更新 systemd unit 文件
	return r.updateUnitFile(project.Name, req)
}

func (r *projectRepo) Delete(ctx context.Context, id uint) error {
	project := new(biz.Project)
	if err := r.db.First(project, id).Error; err != nil {
		return err
	}

	// 删除 systemd unit 文件
	unitPath := r.unitFilePath(project.Name)
	if err := os.Remove(unitPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("%s: %w", r.t.Get("failed to delete systemd config"), err)
	}

	if err := r.db.Delete(project).Error; err != nil {
		return err
	}

	// 记录日志
	r.log.Info("project deleted", slog.String("type", biz.OperationTypeProject), slog.Uint64("operator_id", getOperatorID(ctx)), slog.Uint64("id", uint64(id)), slog.String("name", project.Name))

	return nil
}

// unitFilePath 返回 systemd unit 文件路径
func (r *projectRepo) unitFilePath(name string) string {
	return filepath.Join("/etc/systemd/system", fmt.Sprintf("%s.service", name))
}

// parseProjectDetail 从数据库记录和 systemd unit 文件解析项目详情
func (r *projectRepo) parseProjectDetail(project *biz.Project) (*types.ProjectDetail, error) {
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

	return detail, nil
}

// parseUnitSection 解析 [Unit] 部分
func (r *projectRepo) parseUnitSection(detail *types.ProjectDetail, opt *unit.UnitOption) {
	switch opt.Name {
	case "Description":
		detail.Description = opt.Value
	case "Requires":
		detail.Requires = append(detail.Requires, opt.Value)
	case "Wants":
		detail.Wants = append(detail.Wants, opt.Value)
	case "After":
		detail.After = append(detail.After, opt.Value)
	case "Before":
		detail.Before = append(detail.Before, opt.Value)
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
	case "ProtectTmp":
		detail.ProtectTmp = opt.Value == "true" || opt.Value == "yes"
	case "ProtectHome":
		detail.ProtectHome = opt.Value == "true" || opt.Value == "yes"
	case "ProtectSystem":
		detail.ProtectSystem = opt.Value
	case "ReadWritePaths":
		detail.ReadWritePaths = append(detail.ReadWritePaths, opt.Value)
	case "ReadOnlyPaths":
		detail.ReadOnlyPaths = append(detail.ReadOnlyPaths, opt.Value)
	}
}

// parseEnvironment 解析环境变量
func (r *projectRepo) parseEnvironment(value string) *types.KV {
	parts := strings.SplitN(value, "=", 2)
	if len(parts) != 2 {
		return nil
	}
	return &types.KV{Key: parts[0], Value: parts[1]}
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

// parsePercent 解析百分比 (如 50%)
func (r *projectRepo) parsePercent(value string) (float64, error) {
	value = strings.TrimSuffix(value, "%")
	return strconv.ParseFloat(value, 64)
}

// generateUnitFile 生成 systemd unit 文件
func (r *projectRepo) generateUnitFile(id uint, req *request.ProjectCreate) error {
	options := []*unit.UnitOption{
		// [Unit] section
		unit.NewUnitOption("Unit", "Description", req.Description),
		unit.NewUnitOption("Unit", "After", "network.target"),

		// [Service] section
		unit.NewUnitOption("Service", "Type", "simple"),
		unit.NewUnitOption("Service", "WorkingDirectory", lo.If(req.WorkingDir != "", req.WorkingDir).Else(req.RootDir)),
	}

	if req.ExecStart != "" {
		options = append(options, unit.NewUnitOption("Service", "ExecStart", req.ExecStart))
	}
	if req.User != "" {
		options = append(options, unit.NewUnitOption("Service", "User", req.User))
	}
	if req.Restart != "" {
		options = append(options, unit.NewUnitOption("Service", "Restart", req.Restart))
	} else {
		options = append(options, unit.NewUnitOption("Service", "Restart", "on-failure"))
	}

	// 环境变量
	for _, env := range req.Environments {
		options = append(options, unit.NewUnitOption("Service", "Environment", fmt.Sprintf("%s=%s", env.Key, env.Value)))
	}

	// [Install] section
	options = append(options, unit.NewUnitOption("Install", "WantedBy", "multi-user.target"))

	// 写入文件
	unitPath := r.unitFilePath(req.Name)
	reader := unit.Serialize(options)
	content, err := io.ReadAll(reader)
	if err != nil {
		return err
	}

	return os.WriteFile(unitPath, content, 0644)
}

// updateUnitFile 更新 systemd unit 文件
func (r *projectRepo) updateUnitFile(name string, req *request.ProjectUpdate) error {
	options := []*unit.UnitOption{
		// [Unit] section
		unit.NewUnitOption("Unit", "Description", req.Description),
	}

	// Unit 依赖
	for _, v := range req.Requires {
		options = append(options, unit.NewUnitOption("Unit", "Requires", v))
	}
	for _, v := range req.Wants {
		options = append(options, unit.NewUnitOption("Unit", "Wants", v))
	}
	for _, v := range req.After {
		options = append(options, unit.NewUnitOption("Unit", "After", v))
	}
	if len(req.After) == 0 {
		options = append(options, unit.NewUnitOption("Unit", "After", "network.target"))
	}
	for _, v := range req.Before {
		options = append(options, unit.NewUnitOption("Unit", "Before", v))
	}

	// [Service] section
	options = append(options, unit.NewUnitOption("Service", "Type", "simple"))
	options = append(options, unit.NewUnitOption("Service", "WorkingDirectory", lo.If(req.WorkingDir != "", req.WorkingDir).Else(req.RootDir)))

	if req.ExecStartPre != "" {
		options = append(options, unit.NewUnitOption("Service", "ExecStartPre", req.ExecStartPre))
	}
	if req.ExecStart != "" {
		options = append(options, unit.NewUnitOption("Service", "ExecStart", req.ExecStart))
	}
	if req.ExecStartPost != "" {
		options = append(options, unit.NewUnitOption("Service", "ExecStartPost", req.ExecStartPost))
	}
	if req.ExecStop != "" {
		options = append(options, unit.NewUnitOption("Service", "ExecStop", req.ExecStop))
	}
	if req.ExecReload != "" {
		options = append(options, unit.NewUnitOption("Service", "ExecReload", req.ExecReload))
	}
	if req.User != "" {
		options = append(options, unit.NewUnitOption("Service", "User", req.User))
	}
	if req.Restart != "" {
		options = append(options, unit.NewUnitOption("Service", "Restart", req.Restart))
	} else {
		options = append(options, unit.NewUnitOption("Service", "Restart", "on-failure"))
	}
	if req.RestartSec != "" {
		options = append(options, unit.NewUnitOption("Service", "RestartSec", req.RestartSec))
	}
	if req.RestartMax > 0 {
		options = append(options, unit.NewUnitOption("Service", "StartLimitBurst", strconv.Itoa(req.RestartMax)))
	}
	if req.TimeoutStartSec > 0 {
		options = append(options, unit.NewUnitOption("Service", "TimeoutStartSec", strconv.Itoa(req.TimeoutStartSec)))
	}
	if req.TimeoutStopSec > 0 {
		options = append(options, unit.NewUnitOption("Service", "TimeoutStopSec", strconv.Itoa(req.TimeoutStopSec)))
	}

	// 环境变量
	for _, env := range req.Environments {
		options = append(options, unit.NewUnitOption("Service", "Environment", fmt.Sprintf("%s=%s", env.Key, env.Value)))
	}

	// 输出
	if req.StandardOutput != "" {
		options = append(options, unit.NewUnitOption("Service", "StandardOutput", req.StandardOutput))
	}
	if req.StandardError != "" {
		options = append(options, unit.NewUnitOption("Service", "StandardError", req.StandardError))
	}

	// 资源限制
	if req.MemoryLimit > 0 {
		options = append(options, unit.NewUnitOption("Service", "MemoryLimit", r.formatBytes(req.MemoryLimit)))
	}
	if req.CPUQuota != "" {
		options = append(options, unit.NewUnitOption("Service", "CPUQuota", req.CPUQuota))
	}

	// 安全选项
	if req.NoNewPrivileges {
		options = append(options, unit.NewUnitOption("Service", "NoNewPrivileges", "true"))
	}
	if req.ProtectTmp {
		options = append(options, unit.NewUnitOption("Service", "ProtectTmp", "true"))
	}
	if req.ProtectHome {
		options = append(options, unit.NewUnitOption("Service", "ProtectHome", "true"))
	}
	if req.ProtectSystem != "" {
		options = append(options, unit.NewUnitOption("Service", "ProtectSystem", req.ProtectSystem))
	}
	for _, v := range req.ReadWritePaths {
		options = append(options, unit.NewUnitOption("Service", "ReadWritePaths", v))
	}
	for _, v := range req.ReadOnlyPaths {
		options = append(options, unit.NewUnitOption("Service", "ReadOnlyPaths", v))
	}

	// [Install] section
	options = append(options, unit.NewUnitOption("Install", "WantedBy", "multi-user.target"))

	// 写入文件
	unitPath := r.unitFilePath(name)
	reader := unit.Serialize(options)
	content, err := io.ReadAll(reader)
	if err != nil {
		return err
	}

	return os.WriteFile(unitPath, content, 0644)
}
