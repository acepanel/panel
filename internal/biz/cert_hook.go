package biz

import (
	"context"
	"log/slog"
	"sync"
)

// CertHook 在证书签发、续签或重新生成后被调用，供面板外的服务（如邮件服务）同步最新证书
type CertHook func(ctx context.Context, cert *Cert) error

var (
	certHooksMu sync.RWMutex
	certHooks   []CertHook
)

// RegisterCertHook 注册证书更新钩子，子应用在构造时调用
func RegisterCertHook(hook CertHook) {
	certHooksMu.Lock()
	defer certHooksMu.Unlock()
	certHooks = append(certHooks, hook)
}

// runCertHooks 依次执行钩子，单个钩子失败不影响其它钩子，只记录警告
func (uc *CertUsecase) runCertHooks(ctx context.Context, cert *Cert) {
	certHooksMu.RLock()
	hooks := append([]CertHook(nil), certHooks...)
	certHooksMu.RUnlock()

	for _, hook := range hooks {
		if err := hook(ctx, cert); err != nil {
			uc.log.Warn(uc.t.Get("cert hook failed"), slog.String("type", OperationTypeCert), slog.Uint64("id", uint64(cert.ID)), slog.Any("domains", cert.Domains), slog.Any("err", err))
		}
	}
}
