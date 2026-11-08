package stalwart

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/samber/lo"
	"github.com/spf13/cast"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/service"
	"github.com/acepanel/panel/v3/pkg/firewall"
	"github.com/acepanel/panel/v3/pkg/systemctl"
)

const relayRoute = "smarthost"

// GetSettings 汇总主机名、证书绑定、可选监听器与出站中继
func (s *App) GetSettings(w http.ResponseWriter, r *http.Request) {
	c, err := s.adminClient()
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}
	ctx := r.Context()

	system, err := getOne[swSystemSettings](ctx, c, "SystemSettings", singleton)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	listeners, err := get[swListener](ctx, c, "NetworkListener", nil, []string{"name"})
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	relay, err := s.readRelay(ctx, c)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	certID, _ := s.settingRepo.GetInt(biz.SettingKeyStalwartCertID)

	names := lo.Map(listeners, func(l swListener, _ int) string { return l.Name })
	s.log.Info(s.t.Get("mail server settings viewed"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(ctx)), slog.String("app", slug))
	service.Success(w, Settings{
		Hostname: system.DefaultHostname,
		CertID:   uint(certID),
		Listeners: Listeners{
			Submission: lo.Contains(names, "submission"),
			IMAP:       lo.Contains(names, "imap"),
			POP3:       lo.Contains(names, "pop3"),
		},
		Relay: *relay,
	})
}

// readRelay 中继是否启用看出站策略的兜底路由是否指向 smarthost
func (s *App) readRelay(ctx context.Context, c *client) (*Relay, error) {
	strategy, err := getOne[swOutboundStrategy](ctx, c, "MtaOutboundStrategy", singleton)
	if err != nil {
		return nil, err
	}
	relay := &Relay{Enabled: strings.Contains(strategy.Route.Else, relayRoute), Port: 587}
	route, err := s.findRelayRoute(ctx, c)
	if err != nil {
		return nil, err
	}
	if route != nil {
		relay.Host = route.Host
		relay.Port = route.Port
		relay.Username = deref(route.AuthUsername)
		relay.ImplicitTLS = route.ImplicitTLS
	}
	return relay, nil
}

func (s *App) findRelayRoute(ctx context.Context, c *client) (*swRoute, error) {
	routes, err := get[swRoute](ctx, c, "MtaRoute", nil, []string{"name", "host", "port", "authUsername", "implicitTls", "@type"})
	if err != nil {
		return nil, err
	}
	for _, route := range routes {
		if route.Name == relayRoute {
			return &route, nil
		}
	}
	return nil, nil
}

// UpdateHostname 修改对外主机名，影响 EHLO、MX 记录与自动配置
func (s *App) UpdateHostname(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[SettingHostname](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}
	c, err := s.adminClient()
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	hostname := strings.ToLower(req.Hostname)
	if err = c.update(r.Context(), "SystemSettings", singleton, map[string]any{"defaultHostname": hostname}); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	if err = c.action(r.Context(), "ReloadSettings"); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("mail server hostname updated"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", slug), slog.String("hostname", hostname))
	service.Success(w, nil)
}

// UpdateCert 绑定面板证书作为默认 TLS 证书，cert_id 为 0 时解绑
func (s *App) UpdateCert(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[SettingCert](r)
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

	if req.CertID == 0 {
		if err = s.unbindCert(ctx, c); err != nil {
			service.Error(w, http.StatusInternalServerError, "%v", err)
			return
		}
		s.log.Info(s.t.Get("mail server cert unbound"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(ctx)), slog.String("app", slug))
		service.Success(w, nil)
		return
	}

	cert, err := s.certRepo.Get(req.CertID)
	if err != nil {
		service.Error(w, http.StatusNotFound, "%v", err)
		return
	}
	if cert.Cert == "" || cert.Key == "" {
		service.Error(w, http.StatusUnprocessableEntity, s.t.Get("this certificate has not been obtained successfully"))
		return
	}
	if err = s.pushCert(ctx, c, cert); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	if err = s.settingRepo.Set(biz.SettingKeyStalwartCertID, cast.ToString(cert.ID)); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("mail server cert bound"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(ctx)), slog.String("app", slug), slog.Uint64("cert_id", uint64(cert.ID)), slog.Any("domains", cert.Domains))
	service.Success(w, nil)
}

// pushCert 把证书内容写进 Stalwart 的证书对象并设为默认，再热加载
func (s *App) pushCert(ctx context.Context, c *client, cert *biz.Cert) error {
	obj := map[string]any{
		"certificate": map[string]any{"@type": "Text", "value": cert.Cert},
		"privateKey":  map[string]any{"@type": "Text", "secret": cert.Key},
	}
	objectID, _ := s.settingRepo.Get(biz.SettingKeyStalwartCertObject)
	if objectID != "" {
		if err := c.update(ctx, "Certificate", objectID, obj); err != nil {
			// 对象可能已被用户在 WebUI 里删掉，重新创建
			objectID = ""
		}
	}
	if objectID == "" {
		id, err := c.create(ctx, "Certificate", obj)
		if err != nil {
			return err
		}
		objectID = id
		if err = s.settingRepo.Set(biz.SettingKeyStalwartCertObject, objectID); err != nil {
			return err
		}
	}
	if err := c.update(ctx, "SystemSettings", singleton, map[string]any{"defaultCertificateId": objectID}); err != nil {
		return err
	}
	return c.action(ctx, "ReloadTlsCertificates")
}

func (s *App) unbindCert(ctx context.Context, c *client) error {
	if err := c.update(ctx, "SystemSettings", singleton, map[string]any{"defaultCertificateId": nil}); err != nil {
		return err
	}
	if objectID, _ := s.settingRepo.Get(biz.SettingKeyStalwartCertObject); objectID != "" {
		if err := c.destroy(ctx, "Certificate", []string{objectID}); err != nil {
			return err
		}
	}
	if err := c.action(ctx, "ReloadTlsCertificates"); err != nil {
		return err
	}
	if err := s.settingRepo.Delete(biz.SettingKeyStalwartCertID); err != nil {
		return err
	}
	return s.settingRepo.Delete(biz.SettingKeyStalwartCertObject)
}

// onCertRenewed 面板证书续签后同步到邮件服务，只处理已绑定的那张
func (s *App) onCertRenewed(ctx context.Context, cert *biz.Cert) error {
	boundID, _ := s.settingRepo.GetInt(biz.SettingKeyStalwartCertID)
	if boundID == 0 || uint(boundID) != cert.ID {
		return nil
	}
	c, err := s.adminClient()
	if err != nil {
		return nil //nolint:nilerr // 邮件服务未初始化时没有可同步的目标
	}
	if err = s.pushCert(ctx, c, cert); err != nil {
		return err
	}
	s.log.Info(s.t.Get("mail server cert synced"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(ctx)), slog.String("app", slug), slog.Uint64("cert_id", uint64(cert.ID)), slog.Any("domains", cert.Domains))
	return nil
}

// UpdateListeners 开关 STARTTLS 端口，监听器只在启动时生效所以要重启
func (s *App) UpdateListeners(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[SettingListeners](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}
	c, err := s.adminClient()
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}
	// 改到一半取消会让监听器与防火墙状态不一致
	ctx := context.WithoutCancel(r.Context())

	listeners, err := get[swListener](ctx, c, "NetworkListener", nil, []string{"name"})
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	existing := lo.SliceToMap(listeners, func(l swListener) (string, string) { return l.Name, l.ID })
	desired := map[string]bool{"submission": req.Submission, "imap": req.IMAP, "pop3": req.POP3}

	var opened []uint
	for name, want := range desired {
		id, has := existing[name]
		switch {
		case want && !has:
			if _, err = c.create(ctx, "NetworkListener", listenerSpec(name)); err != nil {
				service.Error(w, http.StatusInternalServerError, "%v", err)
				return
			}
			if err = s.openPort(ctx, listenerPort(name)); err != nil {
				service.Error(w, http.StatusInternalServerError, "%v", err)
				return
			}
			opened = append(opened, listenerPort(name))
		case !want && has:
			if err = c.destroy(ctx, "NetworkListener", []string{id}); err != nil {
				service.Error(w, http.StatusInternalServerError, "%v", err)
				return
			}
		}
	}

	if err = systemctl.Restart(ctx, unit); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("mail server listeners updated"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", slug), slog.Bool("submission", req.Submission), slog.Bool("imap", req.IMAP), slog.Bool("pop3", req.POP3), slog.Any("ports_opened", opened))
	service.Success(w, nil)
}

// openPort 防火墙开着时放行端口
func (s *App) openPort(ctx context.Context, port uint) error {
	fw := firewall.NewFirewall()
	running, err := fw.Status(ctx)
	if err != nil {
		return err
	}
	if !running {
		return nil
	}
	return fw.Port(ctx, firewall.FireInfo{
		Type:      firewall.TypeNormal,
		PortStart: port,
		PortEnd:   port,
		Protocol:  firewall.ProtocolTCP,
		Strategy:  firewall.StrategyAccept,
		Direction: firewall.DirectionIn,
	}, firewall.OperationAdd)
}

// UpdateRelay 配置出站中继：维护名为 smarthost 的 Relay 路由，并把出站策略的兜底路由指过去
func (s *App) UpdateRelay(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[SettingRelay](r)
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

	route, err := s.findRelayRoute(ctx, c)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	if req.Enabled {
		obj := map[string]any{
			"@type":       "Relay",
			"name":        relayRoute,
			"description": "Panel smarthost",
			"host":        strings.TrimSpace(req.Host),
			"port":        req.Port,
			"implicitTls": req.ImplicitTLS,
		}
		if req.Username != "" {
			obj["authUsername"] = req.Username
			if req.Password != "" {
				obj["authSecret"] = map[string]any{"@type": "Value", "secret": req.Password}
			} else if route == nil {
				service.Error(w, http.StatusUnprocessableEntity, s.t.Get("relay password is required"))
				return
			}
		} else {
			obj["authUsername"] = nil
			obj["authSecret"] = map[string]any{"@type": "None"}
		}
		if route == nil {
			if _, err = c.create(ctx, "MtaRoute", obj); err != nil {
				service.Error(w, http.StatusInternalServerError, "%v", err)
				return
			}
		} else {
			delete(obj, "@type")
			delete(obj, "name")
			if err = c.update(ctx, "MtaRoute", route.ID, obj); err != nil {
				service.Error(w, http.StatusInternalServerError, "%v", err)
				return
			}
		}
	}

	// 本地域照常本地投递，其余按开关走中继或直连 MX
	target := lo.Ternary(req.Enabled, "'"+relayRoute+"'", "'mx'")
	expr := map[string]any{
		"match": map[string]any{"0": map[string]any{"if": "is_local_domain(rcpt_domain)", "then": "'local'"}},
		"else":  target,
	}
	if err = c.update(ctx, "MtaOutboundStrategy", singleton, map[string]any{"route": expr}); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	if err = c.action(ctx, "ReloadSettings"); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("mail server relay updated"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(ctx)), slog.String("app", slug), slog.Bool("enabled", req.Enabled), slog.String("host", req.Host), slog.Uint64("port", uint64(req.Port)), slog.String("username", req.Username), slog.Bool("implicit_tls", req.ImplicitTLS))
	service.Success(w, nil)
}
