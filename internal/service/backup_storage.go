package service

import (
	"errors"
	"net/http"

	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/chix/v2"
	"github.com/samber/do/v2"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/pkg/storage"
	"github.com/acepanel/panel/v3/pkg/types"
)

type BackupStorageService struct {
	t                 *gotext.Locale
	backupAccountRepo *biz.BackupAccountUsecase
}

func NewBackupStorageService(i do.Injector) (*BackupStorageService, error) {
	return &BackupStorageService{
		t:                 do.MustInvoke[*gotext.Locale](i),
		backupAccountRepo: do.MustInvoke[*biz.BackupAccountUsecase](i),
	}, nil
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

	if err = s.validateStorage(req.Type, req.Info); err != nil {
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

	if err = s.validateStorage(req.Type, req.Info); err != nil {
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
func (s *BackupStorageService) validateStorage(accountType string, info types.BackupStorageInfo) error {
	var err error
	var client storage.Storage

	switch biz.BackupStorageType(accountType) {
	case biz.BackupStorageTypeS3:
		client, err = storage.NewS3(storage.S3Config{
			Region:          info.Region,
			Bucket:          info.Bucket,
			AccessKey:       info.AccessKey,
			SecretKey:       info.SecretKey,
			Endpoint:        info.Endpoint,
			Scheme:          info.Scheme,
			BasePath:        info.Path,
			AddressingStyle: storage.S3AddressingStyle(info.Style),
		})
		if err != nil {
			return errors.New(s.t.Get("s3 configuration error: %v", err))
		}
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

	if _, err = client.List(""); err != nil {
		return errors.New(s.t.Get("storage connection error: %v", err))
	}

	return nil
}
