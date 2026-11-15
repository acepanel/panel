package service

import (
	"errors"
	"net/http"

	"github.com/leonelquinteros/gotext"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/pkg/benchmark"
)

type ToolboxBenchmarkService struct {
	t *gotext.Locale
}

func NewToolboxBenchmarkService(t *gotext.Locale) *ToolboxBenchmarkService {
	return &ToolboxBenchmarkService{
		t: t,
	}
}

// Test 运行测试
func (s *ToolboxBenchmarkService) Test(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ToolboxBenchmarkTest](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	var result any
	switch req.Name {
	case "memory":
		result, err = benchmark.Memory(r.Context())
	case "disk":
		// 测面板所在的盘，系统临时目录可能是 tmpfs
		result, err = benchmark.Disk(r.Context(), app.Root)
	default:
		result, err = benchmark.CPU(r.Context(), req.Name)
	}
	if err != nil {
		if reason := s.unsupported(err); reason != "" {
			Error(w, http.StatusUnprocessableEntity, "%s", reason)
			return
		}
		Error(w, http.StatusInternalServerError, s.t.Get("failed to run benchmark: %v", err))
		return
	}

	Success(w, result)
}

// unsupported 环境不具备测试条件时给出原因，其它错误返回空串
func (s *ToolboxBenchmarkService) unsupported(err error) string {
	switch {
	case errors.Is(err, benchmark.ErrUnknownTest):
		return s.t.Get("unknown test type")
	case errors.Is(err, benchmark.ErrNoMemory):
		return s.t.Get("not enough available memory to run the memory benchmark")
	case errors.Is(err, benchmark.ErrMemoryFS):
		return s.t.Get("the panel directory is on a memory file system, disk performance cannot be measured")
	case errors.Is(err, benchmark.ErrDirectIO):
		return s.t.Get("the file system of the panel directory does not support direct IO, disk performance cannot be measured")
	case errors.Is(err, benchmark.ErrNoSpace):
		return s.t.Get("not enough free space in the panel directory to run the disk benchmark")
	default:
		return ""
	}
}
