//go:build windows

// 这个文件只是为了在 Windows 下能编译通过，实际上并没有任何卵用

package service

import (
	"github.com/gofiber/fiber/v2"
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

func (s *FileWindowsService) Create(c fiber.Ctx) error {}

func (s *FileWindowsService) Content(c fiber.Ctx) error {}

func (s *FileWindowsService) Save(c fiber.Ctx) error {}

func (s *FileWindowsService) Delete(c fiber.Ctx) error {}

func (s *FileWindowsService) Upload(c fiber.Ctx) error {}

func (s *FileWindowsService) Exist(c fiber.Ctx) error {}

func (s *FileWindowsService) Move(c fiber.Ctx) error {}

func (s *FileWindowsService) Copy(c fiber.Ctx) error {}

func (s *FileWindowsService) Download(c fiber.Ctx) error {}

func (s *FileWindowsService) RemoteDownload(c fiber.Ctx) error {
}

func (s *FileWindowsService) Info(c fiber.Ctx) error {}

func (s *FileWindowsService) Permission(c fiber.Ctx) error {}

func (s *FileWindowsService) Compress(c fiber.Ctx) error {}

func (s *FileWindowsService) UnCompress(c fiber.Ctx) error {}

func (s *FileWindowsService) Search(c fiber.Ctx) error {}

func (s *FileWindowsService) List(c fiber.Ctx) error {}
