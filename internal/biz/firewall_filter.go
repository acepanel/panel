package biz

import (
	"context"
	"errors"
	"log/slog"

	"github.com/leonelquinteros/gotext"
	"github.com/spf13/cast"
)

// FirewallFilterRepo 面板在系统防火墙之外自己维护的内核过滤表，IP 订阅和容器端口管控都靠它生效
type FirewallFilterRepo interface {
	// Sync 让过滤表与启用的订阅、容器端口管控开关、防火墙运行状态和自定义规则保持一致
	Sync(ctx context.Context) error
}

// FirewallFilterUsecase 防火墙过滤表业务逻辑
type FirewallFilterUsecase struct {
	t       *gotext.Locale
	log     *slog.Logger
	repo    FirewallFilterRepo
	setting SettingRepo
}

func NewFirewallFilterUsecase(
	t *gotext.Locale,
	log *slog.Logger,
	repo FirewallFilterRepo,
	setting SettingRepo,
) *FirewallFilterUsecase {
	return &FirewallFilterUsecase{t: t, log: log, repo: repo, setting: setting}
}

// Sync 把过滤表下发到内核，防火墙状态或自定义规则变化后都要调用
func (uc *FirewallFilterUsecase) Sync(ctx context.Context) error {
	return uc.repo.Sync(ctx)
}

// ContainerPorts 容器的映射端口是否和宿主机端口一样受防火墙规则管控
func (uc *FirewallFilterUsecase) ContainerPorts() (bool, error) {
	return uc.setting.GetBool(SettingKeyFirewallContainerPorts)
}

// UpdateContainerPorts 规则下发不了（没装 nftables、容器里没权限）就不保存
func (uc *FirewallFilterUsecase) UpdateContainerPorts(ctx context.Context, enabled bool) error {
	old, err := uc.ContainerPorts()
	if err != nil {
		return err
	}
	if err = uc.setting.Set(SettingKeyFirewallContainerPorts, cast.ToString(enabled)); err != nil {
		return err
	}
	if err = uc.repo.Sync(ctx); err != nil {
		_ = uc.setting.Set(SettingKeyFirewallContainerPorts, cast.ToString(old))
		return errors.New(uc.t.Get("failed to apply firewall filter rules: %v", err))
	}

	uc.log.Info(uc.t.Get("firewall container port filtering updated"), slog.String("type", OperationTypeFirewall), slog.Uint64("operator_id", OperatorID(ctx)), slog.Bool("status", enabled))

	return nil
}
