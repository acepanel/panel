package service

import (
	"net/http"
	"path/filepath"

	"github.com/libtnb/chix/v2"
	"github.com/samber/do/v2"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/pkg/types"
)

type ProjectService struct {
	projectRepo *biz.ProjectUsecase
	settingRepo *biz.SettingUsecase
}

func NewProjectService(i do.Injector) (*ProjectService, error) {
	return &ProjectService{
		projectRepo: do.MustInvoke[*biz.ProjectUsecase](i),
		settingRepo: do.MustInvoke[*biz.SettingUsecase](i),
	}, nil
}

func (s *ProjectService) List(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.Paginate](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	typ := types.ProjectType(r.URL.Query().Get("type"))
	projects, total, err := s.projectRepo.List(typ, req.Page, req.Limit)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

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

	project, err := s.projectRepo.Get(req.ID)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

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
	req, err := Bind[request.ID](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.projectRepo.Delete(r.Context(), req.ID); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}
