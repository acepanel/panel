package service

import (
	"log/slog"
	"net/http"

	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/chix/v2"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
)

type CronService struct {
	t        *gotext.Locale
	log      *slog.Logger
	cronRepo *biz.CronUsecase
}

func NewCronService(cronUsecase *biz.CronUsecase, t *gotext.Locale, log *slog.Logger) *CronService {
	return &CronService{
		t:        t,
		log:      log,
		cronRepo: cronUsecase,
	}
}

func (s *CronService) List(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.Paginate](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	cron, total, err := s.cronRepo.List(req.Page, req.Limit)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("cron list viewed"), slog.String("type", biz.OperationTypeCron), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.Uint64("page", uint64(req.Page)))

	Success(w, chix.M{
		"total": total,
		"items": cron,
	})
}

func (s *CronService) Create(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.CronCreate](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.cronRepo.Create(r.Context(), req); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *CronService) Update(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.CronUpdate](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.cronRepo.Update(r.Context(), req); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *CronService) Get(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ID](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	cron, err := s.cronRepo.Get(req.ID)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("cron viewed"), slog.String("type", biz.OperationTypeCron), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.Uint64("id", uint64(req.ID)), slog.String("name", cron.Name))

	Success(w, cron)
}

func (s *CronService) Delete(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ID](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.cronRepo.Delete(r.Context(), req.ID); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *CronService) Status(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.CronStatus](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.cronRepo.Status(r.Context(), req.ID, req.Status); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}
