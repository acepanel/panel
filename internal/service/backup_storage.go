package service

import (
	"log/slog"
	"net/http"

	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/chix/v2"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
)

type BackupStorageService struct {
	t                 *gotext.Locale
	log               *slog.Logger
	backupAccountRepo *biz.BackupAccountUsecase
}

func NewBackupStorageService(
	backupAccountUsecase *biz.BackupAccountUsecase,
	t *gotext.Locale,
	log *slog.Logger,
) *BackupStorageService {
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

	account, err := s.backupAccountRepo.Create(r.Context(), req)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
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

	if err = s.backupAccountRepo.Update(r.Context(), req); err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
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
