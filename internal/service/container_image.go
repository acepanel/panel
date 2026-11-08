package service

import (
	"log/slog"
	"net/http"

	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/chix/v2"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
)

type ContainerImageService struct {
	containerImageRepo *biz.ContainerImageUsecase
	t                  *gotext.Locale
	log                *slog.Logger
}

func NewContainerImageService(containerImageUsecase *biz.ContainerImageUsecase, t *gotext.Locale, log *slog.Logger) *ContainerImageService {
	return &ContainerImageService{
		containerImageRepo: containerImageUsecase,
		t:                  t,
		log:                log,
	}
}

func (s *ContainerImageService) List(w http.ResponseWriter, r *http.Request) {
	images, err := s.containerImageRepo.List(r.Context())
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	paged, total := Paginate(r, images)

	s.log.Info(s.t.Get("image list viewed"), slog.String("type", biz.OperationTypeContainer), slog.Uint64("operator_id", biz.OperatorID(r.Context())))
	Success(w, chix.M{
		"total": total,
		"items": paged,
	})
}

func (s *ContainerImageService) Exist(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ContainerImagePull](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	exist, err := s.containerImageRepo.Exist(r.Context(), req.Name)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, exist)
}

func (s *ContainerImageService) Pull(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ContainerImagePull](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if req.Background {
		if err = s.containerImageRepo.PullBackground(r.Context(), req); err != nil {
			Error(w, http.StatusInternalServerError, "%v", err)
			return
		}
		Success(w, nil)
		return
	}

	if err = s.containerImageRepo.Pull(r.Context(), req); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *ContainerImageService) Remove(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ContainerImageID](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.containerImageRepo.Remove(r.Context(), req.ID); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *ContainerImageService) Prune(w http.ResponseWriter, r *http.Request) {
	if err := s.containerImageRepo.Prune(r.Context()); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Warn(s.t.Get("images pruned"), slog.String("type", biz.OperationTypeContainer), slog.Uint64("operator_id", biz.OperatorID(r.Context())))
	Success(w, nil)
}
