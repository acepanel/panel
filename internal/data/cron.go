package data

import (
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/utils/str"
	"gorm.io/gorm"

	"github.com/acepanel/panel/internal/app"
	"github.com/acepanel/panel/internal/biz"
	"github.com/acepanel/panel/internal/http/request"
	"github.com/acepanel/panel/pkg/io"
	"github.com/acepanel/panel/pkg/os"
	"github.com/acepanel/panel/pkg/shell"
	"github.com/acepanel/panel/pkg/systemctl"
)

type cronRepo struct {
	t   *gotext.Locale
	db  *gorm.DB
	log *slog.Logger
}

func NewCronRepo(t *gotext.Locale, db *gorm.DB, log *slog.Logger) biz.CronRepo {
	return &cronRepo{
		t:   t,
		db:  db,
		log: log,
	}
}

func (r *cronRepo) Count() (int64, error) {
	var count int64
	if err := r.db.Model(&biz.Cron{}).Count(&count).Error; err != nil {
		return 0, err
	}

	return count, nil
}

func (r *cronRepo) List(page, limit uint) ([]*biz.Cron, int64, error) {
	cron := make([]*biz.Cron, 0)
	var total int64
	err := r.db.Model(&biz.Cron{}).Order("id desc").Count(&total).Offset(int((page - 1) * limit)).Limit(int(limit)).Find(&cron).Error
	return cron, total, err
}

func (r *cronRepo) Get(id uint) (*biz.Cron, error) {
	cron := new(biz.Cron)
	if err := r.db.Where("id = ?", id).First(cron).Error; err != nil {
		return nil, err
	}

	return cron, nil
}

func (r *cronRepo) Create(req *request.CronCreate) error {
	var script string
	if req.Type == "backup" {
		if req.BackupType == "website" {
			script = fmt.Sprintf(`#!/bin/bash
export PATH=/bin:/sbin:/usr/bin:/usr/sbin:/usr/local/bin:/usr/local/sbin:$PATH

acepanel backup website -n '%s' -p '%s'
acepanel backup clear -t website -f '%s' -s '%d' -p '%s'
`, req.Target, req.BackupPath, req.Target, req.Save, req.BackupPath)
		}
		if req.BackupType == "mysql" || req.BackupType == "postgres" {
			script = fmt.Sprintf(`#!/bin/bash
export PATH=/bin:/sbin:/usr/bin:/usr/sbin:/usr/local/bin:/usr/local/sbin:$PATH

acepanel backup database -t '%s' -n '%s' -p '%s'
acepanel backup clear -t '%s' -f '%s' -s '%d' -p '%s'
`, req.BackupType, req.Target, req.BackupPath, req.BackupType, req.Target, req.Save, req.BackupPath)
		}
	}
	if req.Type == "cutoff" {
		script = fmt.Sprintf(`#!/bin/bash
export PATH=/bin:/sbin:/usr/bin:/usr/sbin:/usr/local/bin:/usr/local/sbin:$PATH

acepanel cutoff website -n '%s' -p '%s'
acepanel cutoff clear -t website -f '%s' -s '%d' -p '%s'
`, req.Target, req.BackupPath, req.Target, req.Save, req.BackupPath)
	}
	if req.Type == "shell" {
		script = req.Script
	}

	shellDir := fmt.Sprintf("%s/server/cron/", app.Root)
	shellLogDir := fmt.Sprintf("%s/server/cron/logs/", app.Root)
	shellFile := str.Random(16)
	if err := io.Write(filepath.Join(shellDir, shellFile+".sh"), script, 0700); err != nil {
		return errors.New(err.Error())
	}
	// 编码转换
	_, _ = shell.Execf("dos2unix %s%s.sh", shellDir, shellFile)

	cron := new(biz.Cron)
	cron.Name = req.Name
	cron.Type = req.Type
	cron.Status = true
	cron.Time = req.Time
	cron.Shell = shellDir + shellFile + ".sh"
	cron.Log = shellLogDir + shellFile + ".log"

	if err := r.db.Create(cron).Error; err != nil {
		return err
	}
	if err := r.addToSystem(cron); err != nil {
		return err
	}

	// 记录日志
	r.log.Info("cron created", slog.String("type", biz.OperationTypeCron), slog.Uint64("operator_id", 0), slog.String("name", req.Name), slog.String("cron_type", req.Type))

	return nil
}

func (r *cronRepo) Update(req *request.CronUpdate) error {
	cron, err := r.Get(req.ID)
	if err != nil {
		return err
	}

	cron.Time = req.Time
	cron.Name = req.Name
	if err = r.db.Save(cron).Error; err != nil {
		return err
	}

	if err = io.Write(cron.Shell, req.Script, 0700); err != nil {
		return err
	}
	if out, err := shell.Execf("dos2unix %s", cron.Shell); err != nil {
		return errors.New(out)
	}

	if err = r.deleteFromSystem(cron); err != nil {
		return err
	}
	if cron.Status {
		if err = r.addToSystem(cron); err != nil {
			return err
		}
	}

	// 记录日志
	r.log.Info("cron updated", slog.String("type", biz.OperationTypeCron), slog.Uint64("operator_id", 0), slog.Uint64("id", uint64(req.ID)), slog.String("name", cron.Name))

	return nil
}

func (r *cronRepo) Delete(id uint) error {
	cron, err := r.Get(id)
	if err != nil {
		return err
	}

	if err = r.deleteFromSystem(cron); err != nil {
		return err
	}
	if err = io.Remove(cron.Shell); err != nil {
		return err
	}

	if err = r.db.Delete(cron).Error; err != nil {
		return err
	}

	// 记录日志
	r.log.Info("cron deleted", slog.String("type", biz.OperationTypeCron), slog.Uint64("operator_id", 0), slog.Uint64("id", uint64(id)), slog.String("name", cron.Name))

	return nil
}

func (r *cronRepo) Status(id uint, status bool) error {
	cron, err := r.Get(id)
	if err != nil {
		return err
	}

	if err = r.deleteFromSystem(cron); err != nil {
		return err
	}
	if status {
		if err = r.addToSystem(cron); err != nil {
			return err
		}
	}

	cron.Status = status

	return r.db.Save(cron).Error
}

// addToSystem 添加到系统
func (r *cronRepo) addToSystem(cron *biz.Cron) error {
	if _, err := shell.Execf(`( crontab -l; echo "%s %s >> %s 2>&1" ) | sort - | uniq - | crontab -`, cron.Time, cron.Shell, cron.Log); err != nil {
		return err
	}

	return r.restartCron()
}

// deleteFromSystem 从系统中删除
func (r *cronRepo) deleteFromSystem(cron *biz.Cron) error {
	if _, err := shell.Execf(`( crontab -l | grep -v -F "%s %s >> %s 2>&1" ) | crontab -`, cron.Time, cron.Shell, cron.Log); err != nil {
		return err
	}

	return r.restartCron()
}

// restartCron 重启 cron 服务
func (r *cronRepo) restartCron() error {
	if os.IsRHEL() {
		return systemctl.Restart("crond")
	}

	if os.IsDebian() || os.IsUbuntu() {
		return systemctl.Restart("cron")
	}

	return errors.New(r.t.Get("unsupported system"))
}
