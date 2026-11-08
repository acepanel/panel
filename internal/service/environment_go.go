package service

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/leonelquinteros/gotext"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/pkg/shell"
)

type EnvironmentGoService struct {
	t               *gotext.Locale
	log             *slog.Logger
	environmentRepo *biz.EnvironmentUsecase
}

func NewEnvironmentGoService(environmentUsecase *biz.EnvironmentUsecase, t *gotext.Locale, log *slog.Logger) *EnvironmentGoService {
	return &EnvironmentGoService{
		t:               t,
		log:             log,
		environmentRepo: environmentUsecase,
	}
}

func (s *EnvironmentGoService) SetCli(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.EnvironmentSlug](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}
	if !s.environmentRepo.IsInstalled("go", req.Slug) {
		Error(w, http.StatusUnprocessableEntity, s.t.Get("Go-%s is not installed", req.Slug))
		return
	}

	binPath := fmt.Sprintf("%s/server/go/%s/bin", app.Root, req.Slug)
	if err = linkCLIBinaries(binPath, "go", "gofmt"); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("environment cli set"), slog.String("type", biz.OperationTypeEnvironment), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("environment_type", "go"), slog.String("slug", req.Slug), slog.String("bin_path", binPath))

	Success(w, nil)
}

func (s *EnvironmentGoService) GetProxy(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.EnvironmentSlug](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}
	if !s.environmentRepo.IsInstalled("go", req.Slug) {
		Error(w, http.StatusUnprocessableEntity, s.t.Get("Go-%s is not installed", req.Slug))
		return
	}

	goBin := fmt.Sprintf("%s/server/go/%s/bin/go", app.Root, req.Slug)
	proxy, err := shell.Execf(r.Context(), "%s env GOPROXY", goBin)
	if err != nil {
		proxy = "https://proxy.golang.org,direct"
	}

	s.log.Info(s.t.Get("go proxy viewed"), slog.String("type", biz.OperationTypeEnvironment), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("slug", req.Slug))

	Success(w, strings.TrimSpace(proxy))
}

func (s *EnvironmentGoService) SetProxy(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.EnvironmentProxy](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}
	if !s.environmentRepo.IsInstalled("go", req.Slug) {
		Error(w, http.StatusUnprocessableEntity, s.t.Get("Go-%s is not installed", req.Slug))
		return
	}

	goBin := fmt.Sprintf("%s/server/go/%s/bin/go", app.Root, req.Slug)
	if _, err = shell.Execf(r.Context(), "%s env -w GOPROXY=%s", goBin, shell.Quote(req.Proxy)); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	// GOPROXY 可以是逗号或竖线分隔的多个地址，逐个脱敏
	proxy := regexp.MustCompile(`[^,|]+`).ReplaceAllStringFunc(req.Proxy, redactURL)
	s.log.Info(s.t.Get("go proxy set"), slog.String("type", biz.OperationTypeEnvironment), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("slug", req.Slug), slog.String("proxy", proxy))

	Success(w, nil)
}

// redactURL 去掉 URL 中的账号密码，解析失败返回空串
func redactURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	u.User = nil
	return u.String()
}
