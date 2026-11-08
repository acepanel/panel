package service

import (
	"log/slog"
	"net/http"
	stdos "os"
	"path/filepath"
	"strings"

	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/chix/v2"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/pkg/config"
)

type FileShareService struct {
	t             *gotext.Locale
	log           *slog.Logger
	conf          *config.Config
	fileShareRepo *biz.FileShareUsecase
}

func NewFileShareService(fileShareUsecase *biz.FileShareUsecase, t *gotext.Locale, log *slog.Logger, conf *config.Config) *FileShareService {
	return &FileShareService{
		t:             t,
		log:           log,
		conf:          conf,
		fileShareRepo: fileShareUsecase,
	}
}

func (s *FileShareService) List(w http.ResponseWriter, r *http.Request) {
	shares, err := s.fileShareRepo.List()
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, shares)
}

func (s *FileShareService) Create(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.FileShareCreate](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	info, err := stdos.Stat(req.Path)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}
	if info.IsDir() {
		Error(w, http.StatusUnprocessableEntity, s.t.Get("can't share a directory"))
		return
	}

	share, err := s.fileShareRepo.Create(r.Context(), req)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, share)
}

func (s *FileShareService) Delete(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ID](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.fileShareRepo.Delete(r.Context(), req.ID); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

// Download 通过分享链接下载文件
func (s *FileShareService) Download(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.FileShareToken](r)
	if err != nil {
		Error(w, http.StatusNotFound, "%v", err)
		return
	}

	// 多连接下载时仅首块请求计数，续传分块请求只校验有效性
	rangeHeader := r.Header.Get("Range")
	count := rangeHeader == "" || strings.HasPrefix(rangeHeader, "bytes=0-")
	share, err := s.fileShareRepo.Consume(req.Token, count)
	if err != nil {
		Error(w, http.StatusNotFound, "%v", err)
		return
	}

	info, err := stdos.Stat(share.Path)
	if err != nil || info.IsDir() {
		Error(w, http.StatusNotFound, s.t.Get("share link not found"))
		return
	}

	if count {
		s.log.Info(s.t.Get("file share downloaded"), slog.String("type", biz.OperationTypeFile), slog.Uint64("operator_id", 0), slog.Uint64("id", uint64(share.ID)), slog.String("path", share.Path), slog.String("ip", clientIP(r, s.conf.HTTP.IPHeader)))
	}

	render := chix.NewRender(w, r)
	defer render.Release()
	render.Download(share.Path, filepath.Base(share.Path))
}
