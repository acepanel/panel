package service

import (
	"log/slog"
	"net/http"

	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/chix/v2"
	"github.com/spf13/cast"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
)

type TemplateService struct {
	t            *gotext.Locale
	log          *slog.Logger
	templateRepo *biz.TemplateUsecase
	settingRepo  *biz.SettingUsecase
}

func NewTemplateService(settingUsecase *biz.SettingUsecase, templateUsecase *biz.TemplateUsecase, t *gotext.Locale, log *slog.Logger) *TemplateService {
	return &TemplateService{
		t:            t,
		log:          log,
		templateRepo: templateUsecase,
		settingRepo:  settingUsecase,
	}
}

// List 获取所有模版
func (s *TemplateService) List(w http.ResponseWriter, r *http.Request) {
	category := r.URL.Query().Get("category")
	query := r.URL.Query().Get("query")
	templates := s.templateRepo.List()

	// 按分类过滤
	if category != "" {
		templates = templates.FilterByCategory(category)
	}
	// 按关键词过滤
	if query != "" {
		templates = templates.FilterByQuery(query)
	}

	paged, total := Paginate(r, templates)
	s.log.Info(s.t.Get("template list viewed"), slog.String("type", biz.OperationTypeContainer), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("category", category), slog.String("query", query), slog.Uint64("page", cast.ToUint64(r.URL.Query().Get("page"))))
	Success(w, chix.M{
		"total": total,
		"items": paged,
	})
}

// Get 获取模版详情
func (s *TemplateService) Get(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.TemplateSlug](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	template, err := s.templateRepo.Get(req.Slug)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("template viewed"), slog.String("type", biz.OperationTypeContainer), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("slug", req.Slug))
	Success(w, template)
}

// Create 使用模版创建编排
func (s *TemplateService) Create(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.TemplateCreate](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	// 获取模版
	template, err := s.templateRepo.Get(req.Slug)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	// 使用用户自定义的 compose 内容，如果没有则使用模版默认内容
	compose := req.Compose
	if compose == "" {
		compose = template.Compose
	}

	// 创建编排
	dir, err := s.templateRepo.CreateCompose(r.Context(), req.Slug, req.Name, compose, req.Envs)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	// 本地模板无需回调
	if !template.Local {
		_ = s.templateRepo.Callback(req.Slug)
	}

	Success(w, dir)
}

// Callback 模版下载回调
func (s *TemplateService) Callback(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.TemplateSlug](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.templateRepo.Callback(req.Slug); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}
