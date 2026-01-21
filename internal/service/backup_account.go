package service

import (
	"fmt"
	"net/http"

	"github.com/libtnb/chix"

	"github.com/acepanel/panel/internal/biz"
	"github.com/acepanel/panel/internal/http/request"
	"github.com/acepanel/panel/pkg/storage"
	"github.com/acepanel/panel/pkg/types"
)

type BackupAccountService struct {
	backupAccountRepo biz.BackupAccountRepo
}

func NewBackupAccountService(backupAccount biz.BackupAccountRepo) *BackupAccountService {
	return &BackupAccountService{
		backupAccountRepo: backupAccount,
	}
}

func (s *BackupAccountService) List(w http.ResponseWriter, r *http.Request) {
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

// validateStorage 验证存储账号配置是否正确
// 对于 s3、sftp、webdav 类型，会尝试创建存储连接并验证配置
func (s *BackupAccountService) validateStorage(accountType string, info types.BackupAccountInfo) error {
	var err error
	var client storage.Storage

	switch biz.BackupAccountType(accountType) {
	case biz.BackupAccountTypeS3:
		client, err = storage.NewS3(storage.S3Config{
			Region:          info.Region,
			Bucket:          info.Bucket,
			AccessKeyID:     info.AccessKey,
			SecretAccessKey: info.SecretKey,
			Endpoint:        info.Endpoint,
			BasePath:        info.Path,
			AddressingStyle: storage.S3AddressingStyle(info.Style),
		})
		if err != nil {
			return fmt.Errorf("S3 配置错误: %w", err)
		}
	case biz.BackupAccountTypeSFTP:
		client, err = storage.NewSFTP(storage.SFTPConfig{
			Host:       info.Host,
			Port:       info.Port,
			Username:   info.Username,
			Password:   info.Password,
			PrivateKey: info.PrivateKey,
			BasePath:   info.Path,
		})
		if err != nil {
			return fmt.Errorf("SFTP 配置错误: %w", err)
		}
	case biz.BackupAccountTypeWebDav:
		// WebDAV 在 NewWebDav 中会自动验证连接
		client, err = storage.NewWebDav(storage.WebDavConfig{
			URL:      info.URL,
			Username: info.Username,
			Password: info.Password,
			BasePath: info.Path,
		})
		if err != nil {
			return fmt.Errorf("WebDAV 连接失败: %w", err)
		}
	default:
		// 对于 local 类型或其他类型，不需要验证连接
		return nil
	}

	// 验证连接：尝试列出根目录
	if _, err = client.List(""); err != nil {
		return fmt.Errorf("存储连接失败: %w", err)
	}

	return nil
}

func (s *BackupAccountService) Create(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.BackupAccountCreate](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	// 验证存储连接
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

func (s *BackupAccountService) Update(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.BackupAccountUpdate](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	// 验证存储连接
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

func (s *BackupAccountService) Get(w http.ResponseWriter, r *http.Request) {
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

func (s *BackupAccountService) Delete(w http.ResponseWriter, r *http.Request) {
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
