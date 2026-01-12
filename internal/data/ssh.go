package data

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/leonelquinteros/gotext"
	cryptossh "golang.org/x/crypto/ssh"
	"gorm.io/gorm"

	"github.com/acepanel/panel/internal/biz"
	"github.com/acepanel/panel/internal/http/request"
	pkgssh "github.com/acepanel/panel/pkg/ssh"
)

type sshRepo struct {
	t   *gotext.Locale
	db  *gorm.DB
	log *slog.Logger
}

func NewSSHRepo(t *gotext.Locale, db *gorm.DB, log *slog.Logger) biz.SSHRepo {
	return &sshRepo{
		t:   t,
		db:  db,
		log: log,
	}
}

func (r *sshRepo) List(page, limit uint) ([]*biz.SSH, int64, error) {
	ssh := make([]*biz.SSH, 0)
	var total int64
	err := r.db.Model(&biz.SSH{}).Omit("Hosts").Order("id desc").Count(&total).Offset(int((page - 1) * limit)).Limit(int(limit)).Find(&ssh).Error
	return ssh, total, err
}

func (r *sshRepo) Get(id uint) (*biz.SSH, error) {
	ssh := new(biz.SSH)
	if err := r.db.Where("id = ?", id).First(ssh).Error; err != nil {
		return nil, err
	}

	return ssh, nil
}

func (r *sshRepo) Create(ctx context.Context, req *request.SSHCreate) error {
	conf := pkgssh.ClientConfig{
		AuthMethod: pkgssh.AuthMethod(req.AuthMethod),
		Host:       fmt.Sprintf("%s:%d", req.Host, req.Port),
		User:       req.User,
		Password:   req.Password,
		Key:        req.Key,
		Passphrase: req.Passphrase,
	}
	client, err := pkgssh.NewSSHClient(conf)
	if err != nil {
		return errors.New(r.t.Get("failed to check ssh connection: %v", err))
	}
	defer func(client *cryptossh.Client) { _ = client.Close() }(client)

	ssh := &biz.SSH{
		Name:   req.Name,
		Host:   req.Host,
		Port:   req.Port,
		Config: conf,
		Remark: req.Remark,
	}

	if err = r.db.Create(ssh).Error; err != nil {
		return err
	}

	// 记录日志
	r.log.Info("ssh created", slog.String("type", biz.OperationTypeSSH), slog.Uint64("operator_id", getOperatorID(ctx)), slog.String("name", req.Name), slog.String("host", req.Host))

	return nil
}

func (r *sshRepo) Update(ctx context.Context, req *request.SSHUpdate) error {
	conf := pkgssh.ClientConfig{
		AuthMethod: pkgssh.AuthMethod(req.AuthMethod),
		Host:       fmt.Sprintf("%s:%d", req.Host, req.Port),
		User:       req.User,
		Password:   req.Password,
		Key:        req.Key,
		Passphrase: req.Passphrase,
	}
	client, err := pkgssh.NewSSHClient(conf)
	if err != nil {
		return errors.New(r.t.Get("failed to check ssh connection: %v", err))
	}
	defer func(client *cryptossh.Client) { _ = client.Close() }(client)

	ssh := &biz.SSH{
		ID:     req.ID,
		Name:   req.Name,
		Host:   req.Host,
		Port:   req.Port,
		Config: conf,
		Remark: req.Remark,
	}

	if err = r.db.Model(ssh).Where("id = ?", req.ID).Select("*").Updates(ssh).Error; err != nil {
		return err
	}

	// 记录日志
	r.log.Info("ssh updated", slog.String("type", biz.OperationTypeSSH), slog.Uint64("operator_id", getOperatorID(ctx)), slog.Uint64("id", uint64(req.ID)), slog.String("name", req.Name))

	return nil
}

func (r *sshRepo) Delete(ctx context.Context, id uint) error {
	ssh, err := r.Get(id)
	if err != nil {
		return err
	}

	if err = r.db.Delete(&biz.SSH{}, id).Error; err != nil {
		return err
	}

	// 记录日志
	r.log.Info("ssh deleted", slog.String("type", biz.OperationTypeSSH), slog.Uint64("operator_id", getOperatorID(ctx)), slog.Uint64("id", uint64(id)), slog.String("name", ssh.Name))

	return nil
}
