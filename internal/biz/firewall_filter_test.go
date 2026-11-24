package biz_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/assert/check"
	"github.com/spf13/cast"

	"github.com/acepanel/panel/v3/internal/biz"
	mocks "github.com/acepanel/panel/v3/internal/mocks/biz"
)

// 规则下发不了时开关退回原来的状态
func TestFirewallFilterUpdateContainerPorts(t *testing.T) {
	tests := []struct {
		name    string
		stored  bool
		enabled bool
		syncErr error
		want    bool
	}{
		{name: "开启", enabled: true, want: true},
		{name: "关闭", stored: true},
		{name: "开启时下发失败", enabled: true, syncErr: errors.New("nftables is not installed")},
		{name: "关闭时下发失败", stored: true, syncErr: errors.New("nft: Operation not permitted"), want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stored := tt.stored
			setting := &mocks.SettingRepo{
				GetBoolFunc: func(biz.SettingKey, ...bool) (bool, error) { return stored, nil },
				SetFunc: func(key biz.SettingKey, value string) error {
					check.Equal(t, key, biz.SettingKeyFirewallContainerPorts)
					stored = cast.ToBool(value)
					return nil
				},
			}
			// 下发时读到的必须已经是新状态
			repo := &mocks.FirewallFilterRepo{SyncFunc: func(context.Context) error {
				check.Equal(t, stored, tt.enabled)
				return tt.syncErr
			}}
			usecase := biz.NewFirewallFilterUsecase(gotext.NewLocale("", "en"), slog.New(slog.DiscardHandler), repo, setting)

			err := usecase.UpdateContainerPorts(t.Context(), tt.enabled)
			check.Equal(t, err != nil, tt.syncErr != nil)
			check.Equal(t, stored, tt.want)
		})
	}
}
