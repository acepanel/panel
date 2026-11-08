package service

import (
	"log/slog"
	"net/http"
	"path/filepath"

	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/chix/v2"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/pkg/types"
)

type ProjectService struct {
	t           *gotext.Locale
	log         *slog.Logger
	projectRepo *biz.ProjectUsecase
	settingRepo *biz.SettingUsecase
}

func NewProjectService(projectUsecase *biz.ProjectUsecase, settingUsecase *biz.SettingUsecase, t *gotext.Locale, log *slog.Logger) *ProjectService {
	return &ProjectService{
		t:           t,
		log:         log,
		projectRepo: projectUsecase,
		settingRepo: settingUsecase,
	}
}

func (s *ProjectService) List(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ProjectList](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	projects, total, err := s.projectRepo.List(r.Context(), types.ProjectType(req.Type), req.Keyword, req.Page, req.Limit)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("project list viewed"), slog.String("type", biz.OperationTypeProject), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("project_type", req.Type), slog.String("keyword", req.Keyword), slog.Uint64("page", uint64(req.Page)), slog.Uint64("limit", uint64(req.Limit)))

	Success(w, chix.M{
		"total": total,
		"items": projects,
	})
}

func (s *ProjectService) Get(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ID](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	project, err := s.projectRepo.Get(r.Context(), req.ID)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("project viewed"), slog.String("type", biz.OperationTypeProject), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.Uint64("id", uint64(req.ID)), slog.String("name", project.Name))

	Success(w, project)
}

func (s *ProjectService) Create(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ProjectCreate](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if len(req.RootDir) == 0 {
		req.RootDir, _ = s.settingRepo.Get(biz.SettingKeyProjectPath, "/opt/ace/projects")
		req.RootDir = filepath.Join(req.RootDir, req.Name)
	}

	project, err := s.projectRepo.Create(r.Context(), req)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, project)
}

func (s *ProjectService) Update(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ProjectUpdate](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.projectRepo.Update(r.Context(), req); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *ProjectService) Delete(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ProjectDelete](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.projectRepo.Delete(r.Context(), req.ID, req.Websites); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}
