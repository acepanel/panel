package service

import (
	"net/http"

	"github.com/acepanel/panel/internal/biz"
	"github.com/acepanel/panel/internal/http/request"
)

type LogService struct {
	logRepo biz.LogRepo
}

func NewLogService(logRepo biz.LogRepo) *LogService {
	return &LogService{
		logRepo: logRepo,
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

	entries, err := s.logRepo.List(req.Type, req.Limit)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, entries)
}
