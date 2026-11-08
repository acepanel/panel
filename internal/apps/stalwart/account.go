package stalwart

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/libtnb/chix/v2"
	"github.com/samber/lo"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/service"
)

var accountProps = []string{"name", "domainId", "emailAddress", "description", "quotas", "usedDiskQuota", "aliases", "createdAt", "@type"}

// domainNames 返回域名 id 与名称的双向映射，别名要靠它把邮箱地址换成 domainId
func (s *App) domainNames(ctx context.Context, c *client) (map[string]string, map[string]string, error) {
	domains, err := get[swDomain](ctx, c, "Domain", nil, []string{"name"})
	if err != nil {
		return nil, nil, err
	}
	byID := map[string]string{}
	byName := map[string]string{}
	for _, d := range domains {
		byID[d.ID] = d.Name
		byName[d.Name] = d.ID
	}
	return byID, byName, nil
}

// buildAliases 把 user@example.com 形式的别名换成 Stalwart 的别名对象
func (s *App) buildAliases(aliases []string, byName map[string]string, selfEmail string) (map[string]any, error) {
	result := map[string]any{}
	for _, alias := range aliases {
		alias = strings.ToLower(strings.TrimSpace(alias))
		if alias == "" {
			continue
		}
		local, domain, ok := strings.Cut(alias, "@")
		if !ok || local == "" || domain == "" {
			return nil, errors.New(s.t.Get("alias %s must be a full email address", alias))
		}
		domainID, ok := byName[domain]
		if !ok {
			return nil, errors.New(s.t.Get("alias domain %s is not managed by this mail server", domain))
		}
		if alias == selfEmail {
			return nil, errors.New(s.t.Get("alias %s is the same as the account address", alias))
		}
		result[strconv.Itoa(len(result))] = map[string]any{
			"enable":   true,
			"name":     local,
			"domainId": domainID,
		}
	}
	return result, nil
}

func toAccount(a swAccount, byID map[string]string) Account {
	aliases := lo.FilterMap(lo.Values(a.Aliases), func(alias swAlias, _ int) (string, bool) {
		domain, ok := byID[alias.DomainID]
		return alias.Name + "@" + domain, ok
	})
	slices.Sort(aliases)
	return Account{
		ID:          a.ID,
		Name:        a.Name,
		Email:       a.EmailAddress,
		Description: deref(a.Description),
		Quota:       a.Quotas["maxDiskQuota"],
		Used:        a.UsedDiskQuota,
		Aliases:     aliases,
		CreatedAt:   a.CreatedAt,
	}
}

// AccountList 邮箱账户列表，分页由 JMAP query 完成
func (s *App) AccountList(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[AccountList](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}
	c, err := s.adminClient()
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}
	if req.Page == 0 {
		req.Page = 1
	}
	if req.Limit == 0 {
		req.Limit = 20
	}

	filter := map[string]any{}
	if req.DomainID != "" {
		filter["domainId"] = req.DomainID
	}
	if req.Keyword != "" {
		filter["text"] = req.Keyword
	}
	accounts, total, err := queryGet[swAccount](r.Context(), c, "Account", filter, (req.Page-1)*req.Limit, req.Limit, accountProps)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	byID, _, err := s.domainNames(r.Context(), c)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("mail account list viewed"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", slug), slog.Uint64("page", uint64(req.Page)), slog.String("domain_id", req.DomainID), slog.String("keyword", req.Keyword))
	service.Success(w, chix.M{
		"total": total,
		"items": lo.Map(accounts, func(a swAccount, _ int) Account { return toAccount(a, byID) }),
	})
}

// AccountCreate 创建邮箱账户，密码明文提交由 Stalwart 自行哈希
func (s *App) AccountCreate(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[AccountCreate](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}
	c, err := s.adminClient()
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	byID, byName, err := s.domainNames(r.Context(), c)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	domain, ok := byID[req.DomainID]
	if !ok {
		service.Error(w, http.StatusNotFound, s.t.Get("domain not found"))
		return
	}
	name := strings.ToLower(req.Name)
	aliases, err := s.buildAliases(req.Aliases, byName, name+"@"+domain)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	obj := map[string]any{
		"@type":       "User",
		"name":        name,
		"domainId":    req.DomainID,
		"description": lo.Ternary[any](req.Description != "", req.Description, nil),
		"credentials": map[string]any{"0": map[string]any{"@type": "Password", "secret": req.Password}},
		"roles":       map[string]any{"@type": "User"},
		"quotas":      quotaMap(req.Quota),
		"aliases":     aliases,
	}
	id, err := c.create(r.Context(), "Account", obj)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("mail account created"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", slug), slog.String("email", name+"@"+domain), slog.String("account_id", id), slog.Uint64("quota", req.Quota), slog.Any("aliases", req.Aliases))
	service.Success(w, nil)
}

// AccountUpdate 修改备注、配额与别名
func (s *App) AccountUpdate(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[AccountUpdate](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}
	c, err := s.adminClient()
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	account, err := getOne[swAccount](r.Context(), c, "Account", req.ID)
	if err != nil {
		service.Error(w, http.StatusNotFound, "%v", err)
		return
	}
	_, byName, err := s.domainNames(r.Context(), c)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	aliases, err := s.buildAliases(req.Aliases, byName, account.EmailAddress)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	patch := map[string]any{
		"description": lo.Ternary[any](req.Description != "", req.Description, nil),
		"quotas":      quotaMap(req.Quota),
		"aliases":     aliases,
	}
	if err = c.update(r.Context(), "Account", req.ID, patch); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("mail account updated"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", slug), slog.String("email", account.EmailAddress), slog.Uint64("quota", req.Quota), slog.Any("aliases", req.Aliases))
	service.Success(w, nil)
}

// AccountPassword 重置账户密码，会替换掉该账户的全部登录凭据
func (s *App) AccountPassword(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[AccountPassword](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}
	c, err := s.adminClient()
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	account, err := getOne[swAccount](r.Context(), c, "Account", req.ID)
	if err != nil {
		service.Error(w, http.StatusNotFound, "%v", err)
		return
	}
	patch := map[string]any{"credentials": map[string]any{"0": map[string]any{"@type": "Password", "secret": req.Password}}}
	if err = c.update(r.Context(), "Account", req.ID, patch); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	// 面板自己用的管理员改了密码要同步保存，否则下次请求就登不上
	if adminUser, _ := s.settingRepo.Get(biz.SettingKeyStalwartAdminUser); adminUser == account.EmailAddress {
		if err = s.settingRepo.Set(biz.SettingKeyStalwartAdminPassword, req.Password); err != nil {
			service.Error(w, http.StatusInternalServerError, "%v", err)
			return
		}
	}

	s.log.Info(s.t.Get("mail account password updated"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", slug), slog.String("email", account.EmailAddress))
	service.Success(w, nil)
}

// AccountDelete 删除账户及其全部邮件
func (s *App) AccountDelete(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[AccountID](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}
	c, err := s.adminClient()
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	account, err := getOne[swAccount](r.Context(), c, "Account", req.ID)
	if err != nil {
		service.Error(w, http.StatusNotFound, "%v", err)
		return
	}
	if adminUser, _ := s.settingRepo.Get(biz.SettingKeyStalwartAdminUser); adminUser == account.EmailAddress {
		service.Error(w, http.StatusUnprocessableEntity, s.t.Get("the administrator account used by the panel cannot be deleted"))
		return
	}
	if err = c.destroy(r.Context(), "Account", []string{req.ID}); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Warn(s.t.Get("mail account deleted"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", slug), slog.String("email", account.EmailAddress), slog.String("account_id", req.ID))
	service.Success(w, nil)
}

// quotaMap 0 表示不限，Stalwart 侧用空映射表示
func quotaMap(quota uint64) map[string]any {
	if quota == 0 {
		return map[string]any{}
	}
	return map[string]any{"maxDiskQuota": quota}
}
