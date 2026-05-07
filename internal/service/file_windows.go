//go:build windows

// 这个文件只是为了在 Windows 下能编译通过，实际上并没有任何卵用

package service

import (
	"net/http"

	"github.com/leonelquinteros/gotext"

	"github.com/acepanel/panel/v3/internal/biz"
)

type FileService struct {
	t        *gotext.Locale
	taskRepo biz.TaskRepo
}

func NewFileService(t *gotext.Locale, task biz.TaskRepo) *FileService {
	return &FileService{
		t:        t,
		taskRepo: task,
	}
}

func (s *FileService) Create(w http.ResponseWriter, r *http.Request) {}

func (s *FileService) Content(w http.ResponseWriter, r *http.Request) {}

func (s *FileService) Tail(w http.ResponseWriter, r *http.Request) {}

func (s *FileService) Save(w http.ResponseWriter, r *http.Request) {}

func (s *FileService) Delete(w http.ResponseWriter, r *http.Request) {}

func (s *FileService) Upload(w http.ResponseWriter, r *http.Request) {}

func (s *FileService) Exist(w http.ResponseWriter, r *http.Request) {}

func (s *FileService) Move(w http.ResponseWriter, r *http.Request) {}

func (s *FileService) Copy(w http.ResponseWriter, r *http.Request) {}

func (s *FileService) Download(w http.ResponseWriter, r *http.Request) {}

func (s *FileService) RemoteDownload(w http.ResponseWriter, r *http.Request) {
}

func (s *FileService) Info(w http.ResponseWriter, r *http.Request) {}

func (s *FileService) Permission(w http.ResponseWriter, r *http.Request) {}

func (s *FileService) Compress(w http.ResponseWriter, r *http.Request) {}

func (s *FileService) UnCompress(w http.ResponseWriter, r *http.Request) {}

func (s *FileService) List(w http.ResponseWriter, r *http.Request) {}
