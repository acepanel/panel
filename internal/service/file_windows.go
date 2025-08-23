//go:build windows

// 这个文件只是为了在 Windows 下能编译通过，实际上并没有任何卵用

package service

import (
	"github.com/gofiber/fiber/v3"
	"net/http"

	"github.com/leonelquinteros/gotext"

	"github.com/tnborg/panel/internal/biz"
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

func (s *FileService) Create(c fiber.Ctx) error {}

func (s *FileService) Content(c fiber.Ctx) error {}

func (s *FileService) Save(c fiber.Ctx) error {}

func (s *FileService) Delete(c fiber.Ctx) error {}

func (s *FileService) Upload(c fiber.Ctx) error {}

func (s *FileService) Exist(c fiber.Ctx) error {}

func (s *FileService) Move(c fiber.Ctx) error {}

func (s *FileService) Copy(c fiber.Ctx) error {}

func (s *FileService) Download(c fiber.Ctx) error {}

func (s *FileService) RemoteDownload(c fiber.Ctx) error {
}

func (s *FileService) Info(c fiber.Ctx) error {}

func (s *FileService) Permission(c fiber.Ctx) error {}

func (s *FileService) Compress(c fiber.Ctx) error {}

func (s *FileService) UnCompress(c fiber.Ctx) error {}

func (s *FileService) Search(c fiber.Ctx) error {}

func (s *FileService) List(c fiber.Ctx) error {}
