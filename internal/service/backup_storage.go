package service

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/chix/v2"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/pkg/storage"
	"github.com/acepanel/panel/v3/pkg/types"
)

type BackupStorageService struct {
	t                 *gotext.Locale
	log               *slog.Logger
	backupAccountRepo *biz.BackupAccountUsecase
}

func NewBackupStorageService(backupAccountUsecase *biz.BackupAccountUsecase, t *gotext.Locale, log *slog.Logger) *BackupStorageService {
	return &BackupStorageService{
		t:                 t,
		log:               log,
		backupAccountRepo: backupAccountUsecase,
	}
}

func (s *BackupStorageService) List(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.Paginate](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	accounts, total, err := s.backupAccountRepo.List(req.Page, req.Limit)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("backup storage list viewed"), slog.String("type", biz.OperationTypeBackup), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.Uint64("page", uint64(req.Page)), slog.Uint64("limit", uint64(req.Limit)))

	Success(w, chix.M{
		"total": total,
		"items": accounts,
	})
}

func (s *BackupStorageService) Create(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.BackupStorageCreate](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.validateStorage(r.Context(), req.Type, req.Info); err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	account, err := s.backupAccountRepo.Create(r.Context(), req)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, account)
}

func (s *BackupStorageService) Update(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.BackupStorageUpdate](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.validateStorage(r.Context(), req.Type, req.Info); err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.backupAccountRepo.Update(r.Context(), req); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *BackupStorageService) Get(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ID](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	account, err := s.backupAccountRepo.Get(req.ID)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("backup storage viewed"), slog.String("type", biz.OperationTypeBackup), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.Uint64("id", uint64(req.ID)), slog.String("name", account.Name), slog.String("account_type", string(account.Type)))

	Success(w, account)
}

func (s *BackupStorageService) Delete(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ID](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.backupAccountRepo.Delete(r.Context(), req.ID); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

// validateStorage 验证存储配置是否正确
func (s *BackupStorageService) validateStorage(ctx context.Context, accountType string, info types.BackupStorageInfo) error {
	var err error
	var client storage.Storage

	switch biz.BackupStorageType(accountType) {
	case biz.BackupStorageTypeS3:
		client = storage.NewS3(storage.S3Config{
			Region:          info.Region,
			Bucket:          info.Bucket,
			AccessKey:       info.AccessKey,
			SecretKey:       info.SecretKey,
			Endpoint:        info.Endpoint,
			Scheme:          info.Scheme,
			BasePath:        info.Path,
			AddressingStyle: storage.S3AddressingStyle(info.Style),
		})
	case biz.BackupStorageTypeSFTP:
		client, err = storage.NewSFTP(storage.SFTPConfig{
			Host:       info.Host,
			Port:       info.Port,
			Username:   info.Username,
			Password:   info.Password,
			PrivateKey: info.PrivateKey,
			BasePath:   info.Path,
		})
		if err != nil {
			return errors.New(s.t.Get("sftp configuration error: %v", err))
		}
	case biz.BackupStorageTypeWebDAV:
		client, err = storage.NewWebDav(storage.WebDavConfig{
			URL:      info.URL,
			Username: info.Username,
			Password: info.Password,
			BasePath: info.Path,
		})
		if err != nil {
			return errors.New(s.t.Get("webdav configuration error: %v", err))
		}
	default:
		return nil
	}

	if _, err = client.List(ctx, ""); err != nil {
		return errors.New(s.t.Get("storage connection error: %v", err))
	}

	return nil
}
