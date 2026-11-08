package biz

import (
	"context"
	"log/slog"
	"time"

	"github.com/leonelquinteros/gotext"

	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/pkg/types"
)

type Cron struct {
	ID        uint             `gorm:"primaryKey" json:"id"`
	Name      string           `gorm:"not null;default:'';unique" json:"name"`
	Status    bool             `gorm:"not null;default:false" json:"status"`
	Type      string           `gorm:"not null;default:''" json:"type"`
	Time      string           `gorm:"not null;default:''" json:"time"`
	Config    types.CronConfig `gorm:"serializer:json;not null;default:'{}'" json:"config"`
	Shell     string           `gorm:"not null;default:''" json:"shell"`
	Log       string           `gorm:"not null;default:''" json:"log"`
	CreatedAt time.Time        `json:"created_at"`
	UpdatedAt time.Time        `json:"updated_at"`
}

type CronRepo interface {
	Count() (int64, error)
	List(page, limit uint) ([]*Cron, int64, error)
	Get(id uint) (*Cron, error)
	Create(cron *Cron) error
	Save(cron *Cron) error
	Delete(cron *Cron) error
	GenerateScript(typ string, config types.CronConfig, rawScript string) string
	WriteNewScript(ctx context.Context, script string) (string, string, error)
	WriteScript(path, script string) error
	Dos2Unix(ctx context.Context, path string) error
	AddToSystem(ctx context.Context, cron *Cron) error
	DeleteFromSystem(ctx context.Context, cron *Cron) error
	RemoveFiles(ctx context.Context, cron *Cron) error
}

// CronUsecase 计划任务业务逻辑
type CronUsecase struct {
	repo CronRepo
	t    *gotext.Locale
	log  *slog.Logger
}

func NewCronUsecase(repo CronRepo, t *gotext.Locale, log *slog.Logger) *CronUsecase {
	return &CronUsecase{repo: repo, t: t, log: log}
}

func (uc *CronUsecase) Count() (int64, error) {
	return uc.repo.Count()
}

func (uc *CronUsecase) List(page, limit uint) ([]*Cron, int64, error) {
	return uc.repo.List(page, limit)
}

func (uc *CronUsecase) Get(id uint) (*Cron, error) {
	return uc.repo.Get(id)
}

func (uc *CronUsecase) Create(ctx context.Context, req *request.CronCreate) error {
	config := types.CronConfig{
		Type:     req.SubType,
		Flock:    req.Flock,
		Targets:  req.Targets,
		Storage:  req.Storage,
		Keep:     req.Keep,
		URL:      req.URL,
		Method:   req.Method,
		Headers:  req.Headers,
		Body:     req.Body,
		Timeout:  req.Timeout,
		Insecure: req.Insecure,
		Retries:  req.Retries,
	}
	script := uc.repo.GenerateScript(req.Type, config, req.Script)

	shellPath, logPath, err := uc.repo.WriteNewScript(ctx, script)
	if err != nil {
		return err
	}

	cron := new(Cron)
	cron.Name = req.Name
	cron.Type = req.Type
	cron.Status = true
	cron.Time = req.Time
	cron.Shell = shellPath
	cron.Log = logPath
	cron.Config = config

	if err := uc.repo.Create(cron); err != nil {
		return err
	}
	if err := uc.repo.AddToSystem(ctx, cron); err != nil {
		return err
	}

	// 记录日志
	uc.log.Info(uc.t.Get("cron created"), slog.String("type", OperationTypeCron), slog.Uint64("operator_id", OperatorID(ctx)), slog.Uint64("id", uint64(cron.ID)), slog.String("name", req.Name), slog.String("cron_type", req.Type), slog.String("sub_type", req.SubType), slog.String("schedule", req.Time))

	return nil
}

func (uc *CronUsecase) Update(ctx context.Context, req *request.CronUpdate) error {
	cron, err := uc.repo.Get(req.ID)
	if err != nil {
		return err
	}

	cron.Time = req.Time
	cron.Name = req.Name

	// 根据类型重新生成脚本
	if req.Type != "shell" {
		config := types.CronConfig{
			Type:     req.SubType,
			Flock:    req.Flock,
			Targets:  req.Targets,
			Storage:  req.Storage,
			Keep:     req.Keep,
			URL:      req.URL,
			Method:   req.Method,
			Headers:  req.Headers,
			Body:     req.Body,
			Timeout:  req.Timeout,
			Insecure: req.Insecure,
			Retries:  req.Retries,
		}
		cron.Config = config
		script := uc.repo.GenerateScript(req.Type, config, "")
		if err = uc.repo.WriteScript(cron.Shell, script); err != nil {
			return err
		}
	} else {
		cron.Config.Flock = req.Flock
		if err = uc.repo.WriteScript(cron.Shell, req.Script); err != nil {
			return err
		}
	}

	if err = uc.repo.Save(cron); err != nil {
		return err
	}

	if err = uc.repo.Dos2Unix(ctx, cron.Shell); err != nil {
		return err
	}

	if err = uc.repo.DeleteFromSystem(ctx, cron); err != nil {
		return err
	}
	if cron.Status {
		if err = uc.repo.AddToSystem(ctx, cron); err != nil {
			return err
		}
	}

	// 记录日志
	uc.log.Info(uc.t.Get("cron updated"), slog.String("type", OperationTypeCron), slog.Uint64("operator_id", OperatorID(ctx)), slog.Uint64("id", uint64(req.ID)), slog.String("name", cron.Name), slog.String("cron_type", cron.Type), slog.String("schedule", req.Time))

	return nil
}

func (uc *CronUsecase) Delete(ctx context.Context, id uint) error {
	cron, err := uc.repo.Get(id)
	if err != nil {
		return err
	}

	if err = uc.repo.DeleteFromSystem(ctx, cron); err != nil {
		return err
	}
	if err = uc.repo.RemoveFiles(ctx, cron); err != nil {
		return err
	}

	if err = uc.repo.Delete(cron); err != nil {
		return err
	}

	// 记录日志
	uc.log.Warn(uc.t.Get("cron deleted"), slog.String("type", OperationTypeCron), slog.Uint64("operator_id", OperatorID(ctx)), slog.Uint64("id", uint64(id)), slog.String("name", cron.Name))

	return nil
}

func (uc *CronUsecase) Status(ctx context.Context, id uint, status bool) error {
	cron, err := uc.repo.Get(id)
	if err != nil {
		return err
	}

	if err = uc.repo.DeleteFromSystem(ctx, cron); err != nil {
		return err
	}
	if status {
		if err = uc.repo.AddToSystem(ctx, cron); err != nil {
			return err
		}
	}

	cron.Status = status

	if err = uc.repo.Save(cron); err != nil {
		return err
	}

	uc.log.Info(uc.t.Get("cron status updated"), slog.String("type", OperationTypeCron), slog.Uint64("operator_id", OperatorID(ctx)), slog.Uint64("id", uint64(id)), slog.String("name", cron.Name), slog.Bool("status", status))

	return nil
}
