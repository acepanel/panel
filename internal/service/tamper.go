package service

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/chix/v2"
	"github.com/spf13/cast"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/pkg/tools"
)

type TamperService struct {
	t          *gotext.Locale
	log        *slog.Logger
	tamperRepo *biz.TamperUsecase
}

func NewTamperService(tamperUsecase *biz.TamperUsecase, t *gotext.Locale, log *slog.Logger) *TamperService {
	return &TamperService{
		t:          t,
		log:        log,
		tamperRepo: tamperUsecase,
	}
}

// Status 防篡改运行状态与环境检测
func (s *TamperService) Status(w http.ResponseWriter, r *http.Request) {
	setting, err := s.tamperRepo.GetSetting()
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	Success(w, chix.M{
		"supported": s.tamperRepo.Supported(),
		"setting":   setting,
		"stats":     s.tamperRepo.Stats(),
		"ebpf":      s.tamperRepo.DetectEBPF(),
	})
}

// GetSetting 获取全局设置
func (s *TamperService) GetSetting(w http.ResponseWriter, r *http.Request) {
	setting, err := s.tamperRepo.GetSetting()
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("tamper settings viewed"), slog.String("type", biz.OperationTypeSafe), slog.Uint64("operator_id", biz.OperatorID(r.Context())))

	Success(w, setting)
}

// SaveSetting 保存全局设置(含开关,立即生效)
func (s *TamperService) SaveSetting(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.TamperSetting](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}
	if !s.tamperRepo.Supported() {
		Error(w, http.StatusUnprocessableEntity, s.t.Get("tamper protection is only supported on Linux"))
		return
	}
	// 启用 eBPF 模式前校验可用性
	if req.Enabled && req.Mode == "ebpf" {
		if st := s.tamperRepo.DetectEBPF(); !st.Available {
			Error(w, http.StatusUnprocessableEntity, s.t.Get("eBPF mode unavailable: %s", st.Reason))
			return
		}
	}

	if err = s.tamperRepo.SaveSetting(r.Context(), &biz.TamperSetting{
		Enabled:       req.Enabled,
		Mode:          req.Mode,
		BlockNewFiles: req.BlockNewFiles,
		LogDays:       req.LogDays,
	}); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	Success(w, nil)
}

// CheckPaths 批量查询路径保护状态(供文件管理器展示)
func (s *TamperService) CheckPaths(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.TamperCheckPaths](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}
	res, err := s.tamperRepo.CheckPaths(req.Paths)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	Success(w, chix.M{
		"running": s.tamperRepo.Running(),
		"items":   res,
	})
}

// Protect 切换路径保护状态(供文件管理器右键操作)
func (s *TamperService) Protect(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.TamperProtect](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}
	if err = s.tamperRepo.SetProtect(r.Context(), req.Path, req.Protect); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	Success(w, nil)
}

// ListRules 保护规则列表
func (s *TamperService) ListRules(w http.ResponseWriter, r *http.Request) {
	rules, err := s.tamperRepo.ListRules()
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	paged, total := Paginate(r, rules)

	s.log.Info(s.t.Get("tamper rule list viewed"), slog.String("type", biz.OperationTypeSafe), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.Uint64("page", cast.ToUint64(r.URL.Query().Get("page"))), slog.Uint64("limit", cast.ToUint64(r.URL.Query().Get("limit"))))

	Success(w, chix.M{"total": total, "items": paged})
}

// CreateRule 新增保护规则
func (s *TamperService) CreateRule(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.TamperRuleCreate](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}
	if err = s.tamperRepo.CreateRule(r.Context(), &biz.TamperRule{
		Name:     req.Name,
		Path:     req.Path,
		Exts:     req.Exts,
		Excludes: req.Excludes,
		Enabled:  req.Enabled,
	}); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	Success(w, nil)
}

// UpdateRule 更新保护规则
func (s *TamperService) UpdateRule(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.TamperRuleUpdate](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}
	rule, err := s.tamperRepo.GetRule(req.ID)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	rule.Path = req.Path
	rule.Exts = req.Exts
	rule.Excludes = req.Excludes
	rule.Enabled = req.Enabled
	if err = s.tamperRepo.UpdateRule(r.Context(), rule); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	Success(w, nil)
}

// DeleteRule 删除保护规则
func (s *TamperService) DeleteRule(w http.ResponseWriter, r *http.Request) {
	id := cast.ToUint(chi.URLParam(r, "id"))
	if err := s.tamperRepo.DeleteRule(r.Context(), id); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	Success(w, nil)
}

// ListLogs 拦截日志
func (s *TamperService) ListLogs(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.Paginate](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}
	logs, total, err := s.tamperRepo.ListLogs(req.Page, req.Limit)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("tamper log viewed"), slog.String("type", biz.OperationTypeSafe), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.Uint64("page", uint64(req.Page)), slog.Uint64("limit", uint64(req.Limit)))

	Success(w, chix.M{"total": total, "items": logs})
}

// ClearLogs 清空拦截日志
func (s *TamperService) ClearLogs(w http.ResponseWriter, r *http.Request) {
	if err := s.tamperRepo.ClearLogs(r.Context()); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	Success(w, nil)
}

// ActivateEBPF 修改 grub 激活 bpf LSM 并重启系统
func (s *TamperService) ActivateEBPF(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 5*time.Minute)
	defer cancel()
	if err := s.tamperRepo.EnableBPFLSMGrub(ctx); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	Success(w, nil)
	// 响应后重启系统使 bpf LSM 生效
	tools.RestartServer(r.Context())
}
