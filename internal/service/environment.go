package service

import (
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/leonelquinteros/gotext"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/pkg/types"
)

type EnvironmentService struct {
	t               *gotext.Locale
	log             *slog.Logger
	environmentRepo *biz.EnvironmentUsecase
	taskRepo        *biz.TaskUsecase
}

func NewEnvironmentService(environmentUsecase *biz.EnvironmentUsecase, taskUsecase *biz.TaskUsecase, t *gotext.Locale, log *slog.Logger) *EnvironmentService {
	return &EnvironmentService{
		t:               t,
		log:             log,
		environmentRepo: environmentUsecase,
		taskRepo:        taskUsecase,
	}
}

func linkCLIBinaries(binPath string, names ...string) error {
	for _, name := range names {
		link := filepath.Join("/usr/local/bin", name)
		_ = os.Remove(link)
		if err := os.Symlink(filepath.Join(binPath, name), link); err != nil {
			return err
		}
	}
	return nil
}

func (s *EnvironmentService) Types(w http.ResponseWriter, r *http.Request) {
	Success(w, s.environmentRepo.Types())
}

func (s *EnvironmentService) List(w http.ResponseWriter, r *http.Request) {
	typ := r.URL.Query().Get("type")
	query := strings.ToLower(r.URL.Query().Get("query"))
	onlyInstalled := r.URL.Query().Get("installed") == "true"
	all := s.environmentRepo.All()
	environments := make([]types.EnvironmentDetail, 0)
	for _, item := range all {
		if typ != "" && !strings.EqualFold(item.Type, typ) {
			continue
		}
		if query != "" &&
			!strings.Contains(strings.ToLower(item.Name), query) &&
			!strings.Contains(strings.ToLower(item.Description), query) {
			continue
		}
		installed := s.environmentRepo.IsInstalled(item.Type, item.Slug)
		if onlyInstalled && !installed {
			continue
		}
		// 性能优化，避免重复探测版本
		var installedVersion string
		if installed {
			installedVersion = s.environmentRepo.InstalledVersion(r.Context(), item.Type, item.Slug)
		}
		environments = append(environments, types.EnvironmentDetail{
			Type:             item.Type,
			Name:             item.Name,
			Description:      item.Description,
			Slug:             item.Slug,
			Version:          item.Version,
			InstalledVersion: installedVersion,
			Installed:        installed,
			HasUpdate:        installed && item.Version != "" && installedVersion != "" && item.Version != installedVersion,
			CustomSupported:  biz.CustomCompileEnv(item.Type),
		})
	}

	s.log.Info(s.t.Get("environment list viewed"), slog.String("type", biz.OperationTypeEnvironment), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("environment_type", typ), slog.String("query", query), slog.Bool("installed", onlyInstalled))

	Success(w, environments)
}

func (s *EnvironmentService) Install(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.EnvironmentAction](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.environmentRepo.Install(r.Context(), req.Type, req.Slug); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *EnvironmentService) Uninstall(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.EnvironmentAction](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.environmentRepo.Uninstall(r.Context(), req.Type, req.Slug); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *EnvironmentService) Update(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.EnvironmentAction](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.environmentRepo.Update(r.Context(), req.Type, req.Slug); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *EnvironmentService) IsInstalled(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.EnvironmentAction](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	installed := s.environmentRepo.IsInstalled(req.Type, req.Slug)
	Success(w, installed)
}
