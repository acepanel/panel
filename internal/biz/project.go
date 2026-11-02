package biz

import (
	"context"
	"errors"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"github.com/leonelquinteros/gotext"
	"github.com/samber/lo"

	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/pkg/systemctl"
	"github.com/acepanel/panel/v3/pkg/types"
)

type Project struct {
	ID        uint              `gorm:"primaryKey" json:"id"`
	Name      string            `gorm:"not null;unique" json:"name"`                  // 项目名称
	Type      types.ProjectType `gorm:"not null;index;default:'general'" json:"type"` // 项目类型
	Path      string            `gorm:"not null;default:''" json:"path"`              // 项目路径
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}

type ProjectRepo interface {
	Count() (int64, error)
	List(typ types.ProjectType, keyword string, page, limit uint) ([]*Project, int64, error)
	GetEntity(id uint) (*Project, error)
	ParseDetail(ctx context.Context, project *Project) (*types.ProjectDetail, error)
	NameExists(name string) (bool, error)
	Create(ctx context.Context, project *Project, req *request.ProjectCreate) error
	Save(project *Project) error
	Delete(project *Project) error
	RenameUnitFile(oldName, newName string) error
	RemoveUnitFile(name string) error
	UpdateUnitFile(ctx context.Context, name string, req *request.ProjectUpdate) error
}

type ProjectUsecase struct {
	repo    ProjectRepo
	website *WebsiteUsecase
	log     *slog.Logger
	t       *gotext.Locale
}

func NewProjectUsecase(t *gotext.Locale, log *slog.Logger, projectRepo ProjectRepo, websiteUsecase *WebsiteUsecase) *ProjectUsecase {
	return &ProjectUsecase{
		repo:    projectRepo,
		website: websiteUsecase,
		log:     log,
		t:       t,
	}
}

func (uc *ProjectUsecase) Count() (int64, error) {
	return uc.repo.Count()
}

func (uc *ProjectUsecase) List(ctx context.Context, typ types.ProjectType, keyword string, page, limit uint) ([]*types.ProjectDetail, int64, error) {
	projects, total, err := uc.repo.List(typ, keyword, page, limit)
	if err != nil {
		return nil, 0, err
	}

	details := lo.Map(projects, func(p *Project, _ int) *types.ProjectDetail {
		detail, err := uc.repo.ParseDetail(ctx, p)
		if err != nil {
			// 如果解析失败，返回基本信息
			return &types.ProjectDetail{
				ID:   p.ID,
				Name: p.Name,
				Type: p.Type,
			}
		}
		return detail
	})

	return details, total, nil
}

func (uc *ProjectUsecase) Get(ctx context.Context, id uint) (*types.ProjectDetail, error) {
	project, err := uc.repo.GetEntity(id)
	if err != nil {
		return nil, err
	}
	return uc.repo.ParseDetail(ctx, project)
}

func (uc *ProjectUsecase) Create(ctx context.Context, req *request.ProjectCreate) (*types.ProjectDetail, error) {
	// 检查项目名是否已存在
	exists, err := uc.repo.NameExists(req.Name)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, errors.New(uc.t.Get("project name already exists"))
	}

	project := &Project{
		Name: req.Name,
		Type: req.Type,
		Path: lo.If(!strings.HasPrefix(req.RootDir, "/"), filepath.Join("/", req.RootDir)).Else(req.RootDir),
	}

	if err := uc.repo.Create(ctx, project, req); err != nil {
		return nil, err
	}

	// 记录日志
	uc.log.Info("project created", slog.String("type", OperationTypeProject), slog.Uint64("operator_id", operatorID(ctx)), slog.String("name", req.Name), slog.String("project_type", string(req.Type)))

	return uc.repo.ParseDetail(ctx, project)
}

func (uc *ProjectUsecase) Update(ctx context.Context, req *request.ProjectUpdate) error {
	project, err := uc.repo.GetEntity(req.ID)
	if err != nil {
		return err
	}

	// systemd 按名字跟踪服务，只改 unit 文件名会让旧单元带着进程继续跑而新单元显示停止
	// 改名需先停掉并取消自启旧单元，写完新配置后再按原状态恢复新单元
	renamed := req.Name != project.Name
	var running, enabled bool
	if renamed {
		running, _ = systemctl.Status(ctx, project.Name)
		enabled, _ = systemctl.IsEnabled(ctx, project.Name)
		if running {
			if err := systemctl.Stop(ctx, project.Name); err != nil {
				return err
			}
		}
		if enabled {
			if err := systemctl.Disable(ctx, project.Name); err != nil {
				return err
			}
		}
		if err := uc.repo.RenameUnitFile(project.Name, req.Name); err != nil {
			return err
		}
		project.Name = req.Name
	}

	project.Path = lo.If(!strings.HasPrefix(req.RootDir, "/"), filepath.Join("/", req.RootDir)).Else(req.RootDir)
	if err := uc.repo.Save(project); err != nil {
		return err
	}

	// 记录日志
	uc.log.Info("project updated", slog.String("type", OperationTypeProject), slog.Uint64("operator_id", operatorID(ctx)), slog.Uint64("id", uint64(req.ID)), slog.String("name", project.Name))

	// 更新 systemd unit 文件
	if err := uc.repo.UpdateUnitFile(ctx, project.Name, req); err != nil {
		return err
	}

	if renamed && enabled {
		if err := systemctl.Enable(ctx, project.Name); err != nil {
			return err
		}
	}
	if renamed && running {
		return systemctl.Start(ctx, project.Name)
	}

	return nil
}

func (uc *ProjectUsecase) Delete(ctx context.Context, id uint, websites bool) error {
	project, err := uc.repo.GetEntity(id)
	if err != nil {
		return err
	}

	if err = uc.website.ReleaseProject(ctx, id, websites); err != nil {
		return err
	}

	// 删除 systemd unit 文件
	if err := uc.repo.RemoveUnitFile(project.Name); err != nil {
		return err
	}

	if err := uc.repo.Delete(project); err != nil {
		return err
	}

	// 记录日志
	uc.log.Info("project deleted", slog.String("type", OperationTypeProject), slog.Uint64("operator_id", operatorID(ctx)), slog.Uint64("id", uint64(id)), slog.String("name", project.Name))

	return nil
}
