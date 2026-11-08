package stalwart

import (
	"log/slog"
	"net/http"
	"slices"
	"strings"

	"github.com/libtnb/chix/v2"
	"github.com/samber/lo"
	"github.com/spf13/cast"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/service"
)

var domainProps = []string{"name", "aliases", "createdAt", "catchAllAddress"}

func toDomain(d swDomain, defaultID string) Domain {
	aliases := lo.Keys(d.Aliases)
	slices.Sort(aliases)
	return Domain{
		ID:        d.ID,
		Name:      d.Name,
		CreatedAt: d.CreatedAt,
		CatchAll:  deref(d.CatchAllAddress),
		Aliases:   aliases,
		IsDefault: d.ID == defaultID,
	}
}

// DomainList 域名列表
func (s *App) DomainList(w http.ResponseWriter, r *http.Request) {
	c, err := s.adminClient()
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}
	domains, err := get[swDomain](r.Context(), c, "Domain", nil, domainProps)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	settings, err := getOne[swSystemSettings](r.Context(), c, "SystemSettings", singleton)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	items := lo.Map(domains, func(d swDomain, _ int) Domain { return toDomain(d, settings.DefaultDomainID) })
	slices.SortFunc(items, func(a, b Domain) int { return strings.Compare(a.Name, b.Name) })
	paged, total := service.Paginate(r, items)

	s.log.Info(s.t.Get("mail domain list viewed"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", slug), slog.Uint64("page", cast.ToUint64(r.URL.Query().Get("page"))))
	service.Success(w, chix.M{
		"total": total,
		"items": paged,
	})
}

// DomainCreate 添加域名，DKIM 密钥由 Stalwart 自动生成
func (s *App) DomainCreate(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[DomainCreate](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}
	c, err := s.adminClient()
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	name := strings.ToLower(req.Name)
	id, err := c.create(r.Context(), "Domain", map[string]any{
		"name":                  name,
		"dkimManagement":        map[string]any{"@type": "Automatic"},
		"dnsManagement":         map[string]any{"@type": "Manual"},
		"certificateManagement": map[string]any{"@type": "Manual"},
	})
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("mail domain created"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", slug), slog.String("domain", name), slog.String("domain_id", id))
	service.Success(w, nil)
}

// DomainDelete 删除域名，Stalwart 要求先清掉关联的账户与 DKIM 密钥
func (s *App) DomainDelete(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[DomainID](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}
	c, err := s.adminClient()
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}
	ctx := r.Context()

	domain, err := getOne[swDomain](ctx, c, "Domain", req.ID)
	if err != nil {
		service.Error(w, http.StatusNotFound, "%v", err)
		return
	}
	settings, err := getOne[swSystemSettings](ctx, c, "SystemSettings", singleton)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	if settings.DefaultDomainID == req.ID {
		service.Error(w, http.StatusUnprocessableEntity, s.t.Get("the default domain cannot be deleted"))
		return
	}

	accounts, err := c.query(ctx, "Account", map[string]any{"domainId": req.ID}, 0, 0)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	if err = c.destroy(ctx, "Account", accounts.IDs); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	signatures, err := c.query(ctx, "DkimSignature", map[string]any{"domainId": req.ID}, 0, 0)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	if err = c.destroy(ctx, "DkimSignature", signatures.IDs); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	if err = c.destroy(ctx, "Domain", []string{req.ID}); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Warn(s.t.Get("mail domain deleted"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", slug), slog.String("domain", domain.Name), slog.String("domain_id", req.ID), slog.Int("accounts", len(accounts.IDs)))
	service.Success(w, nil)
}

// DomainCatchAll 设置或清空域名的兜底收件地址
func (s *App) DomainCatchAll(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[DomainCatchAll](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}
	c, err := s.adminClient()
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	domain, err := getOne[swDomain](r.Context(), c, "Domain", req.ID)
	if err != nil {
		service.Error(w, http.StatusNotFound, "%v", err)
		return
	}
	address := strings.ToLower(strings.TrimSpace(req.Address))
	var value any
	if address != "" {
		if !strings.Contains(address, "@") {
			address += "@" + domain.Name
		}
		value = address
	}
	if err = c.update(r.Context(), "Domain", req.ID, map[string]any{"catchAllAddress": value}); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("mail domain catch-all updated"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", slug), slog.String("domain", domain.Name), slog.String("address", address))
	service.Success(w, nil)
}
