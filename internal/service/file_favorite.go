package service

import (
	"net/http"
	"path/filepath"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
)

type FileFavoriteService struct {
	fileFavoriteRepo *biz.FileFavoriteUsecase
}

func NewFileFavoriteService(fileFavoriteUsecase *biz.FileFavoriteUsecase) *FileFavoriteService {
	return &FileFavoriteService{
		fileFavoriteRepo: fileFavoriteUsecase,
	}
}

func (s *FileFavoriteService) List(w http.ResponseWriter, r *http.Request) {
	favorites, err := s.fileFavoriteRepo.List()
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, favorites)
}

func (s *FileFavoriteService) Create(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.FileFavoriteCreate](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	favorite, err := s.fileFavoriteRepo.Create(filepath.Clean(req.Path), req.Remark)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, favorite)
}

func (s *FileFavoriteService) Update(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.FileFavoriteUpdate](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.fileFavoriteRepo.UpdateRemark(req.ID, req.Remark); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *FileFavoriteService) Delete(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ID](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.fileFavoriteRepo.Delete(req.ID); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *FileFavoriteService) UpdateOrder(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.FileFavoriteOrder](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.fileFavoriteRepo.UpdateOrder(req.IDs); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}
