package service

import (
	"log/slog"
	"net/http"

	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/chix/v2"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
)

type SSHService struct {
	t       *gotext.Locale
	log     *slog.Logger
	sshRepo *biz.SSHUsecase
}

func NewSSHService(sshUsecase *biz.SSHUsecase, t *gotext.Locale, log *slog.Logger) *SSHService {
	return &SSHService{
		t:       t,
		log:     log,
		sshRepo: sshUsecase,
	}
}

func (s *SSHService) List(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.Paginate](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	cron, total, err := s.sshRepo.List(req.Page, req.Limit)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("ssh host list viewed"), slog.String("type", biz.OperationTypeSSH), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.Uint64("page", uint64(req.Page)), slog.Uint64("limit", uint64(req.Limit)))

	Success(w, chix.M{
		"total": total,
		"items": cron,
	})
}

func (s *SSHService) Create(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.SSHCreate](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.sshRepo.Create(r.Context(), req); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *SSHService) Update(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.SSHUpdate](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.sshRepo.Update(r.Context(), req); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *SSHService) Get(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ID](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	cron, err := s.sshRepo.Get(req.ID)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("ssh host viewed"), slog.String("type", biz.OperationTypeSSH), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.Uint64("id", uint64(cron.ID)), slog.String("name", cron.Name), slog.String("host", cron.Host))

	Success(w, cron)
}

func (s *SSHService) Delete(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ID](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.sshRepo.Delete(r.Context(), req.ID); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *SSHService) ListFiles(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.SSHFile](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	files, err := s.sshRepo.ListFiles(req.ID, req.Path)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("ssh file list viewed"), slog.String("type", biz.OperationTypeSSH), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.Uint64("host_id", uint64(req.ID)), slog.String("path", req.Path))

	Success(w, files)
}

func (s *SSHService) Mkdir(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.SSHFile](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.sshRepo.Mkdir(r.Context(), req.ID, req.Path); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}
