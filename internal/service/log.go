package service

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/leonelquinteros/gotext"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/pkg/sshlog"
)

type LogService struct {
	t       *gotext.Locale
	log     *slog.Logger
	logRepo *biz.LogUsecase
}

func NewLogService(logUsecase *biz.LogUsecase, t *gotext.Locale, log *slog.Logger) *LogService {
	return &LogService{
		t:       t,
		log:     log,
		logRepo: logUsecase,
	}
}

// List 获取日志列表
func (s *LogService) List(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.LogList](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	// 默认限制
	if req.Limit == 0 {
		req.Limit = 100
	}

	entries, err := s.logRepo.List(req.Type, req.Limit, req.Date, req.Level, req.Keyword)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	// 操作日志就在 app.log 中，查看时再记会不断自我记录
	if req.Type != biz.LogTypeApp {
		s.log.Info(s.t.Get("panel log viewed"), slog.String("type", biz.OperationTypePanel), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("log_type", req.Type), slog.String("date", req.Date), slog.String("log_level", req.Level), slog.String("keyword", req.Keyword))
	}

	Success(w, entries)
}

// Dates 获取日志日期列表
func (s *LogService) Dates(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.LogDates](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	dates, err := s.logRepo.ListDates(req.Type)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, dates)
}

// Clean 清理指定日期及之前的日志
func (s *LogService) Clean(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.LogClean](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.logRepo.Clean(r.Context(), req.Type, req.Date); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

// SSH 获取 SSH 登录日志
func (s *LogService) SSH(w http.ResponseWriter, r *http.Request) {
	limit := 100
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 1000 {
			limit = n
		}
	}

	logs, err := sshlog.Read(r.Context(), limit)
	if errors.Is(err, sshlog.ErrNotFound) {
		err = errors.New(s.t.Get("SSH log file not found"))
	}
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("ssh login log viewed"), slog.String("type", biz.OperationTypePanel), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.Int("limit", limit))

	Success(w, logs)
}
