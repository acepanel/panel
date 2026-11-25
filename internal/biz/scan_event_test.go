package biz_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/assert/check"
	"github.com/libtnb/assert/must"

	"github.com/acepanel/panel/v3/internal/biz"
	mocks "github.com/acepanel/panel/v3/internal/mocks/biz"
)

func newScanEventUsecase(repo *mocks.ScanEventRepo, filter *mocks.FirewallFilterRepo) *biz.ScanEventUsecase {
	setting := &mocks.SettingRepo{
		SetFunc:      func(biz.SettingKey, string) error { return nil },
		SetSliceFunc: func(biz.SettingKey, []string) error { return nil },
	}
	return biz.NewScanEventUsecase(gotext.NewLocale("", "en"), slog.New(slog.DiscardHandler), repo, setting, filter)
}

func TestScanEventUnblock(t *testing.T) {
	var deleted []string
	repo := &mocks.ScanEventRepo{DeleteBlocksFunc: func(ips []string) error {
		deleted = ips
		return nil
	}}
	filter := &mocks.FirewallFilterRepo{SyncFunc: func(context.Context) error { return nil }}

	must.NoError(t, newScanEventUsecase(repo, filter).Unblock(t.Context(), []string{"198.51.100.1"}))
	check.DeepEqual(t, deleted, []string{"198.51.100.1"})
	check.Len(t, filter.SyncCalls(), 1)
}

// 关闭扫描感知时解封全部 IP；保持开启时 ClearBlocks 与 Sync 没有桩，被调到就会 panic
func TestScanEventUpdateSettingUnblocks(t *testing.T) {
	must.NoError(t, newScanEventUsecase(&mocks.ScanEventRepo{}, &mocks.FirewallFilterRepo{}).UpdateSetting(t.Context(), &biz.ScanSetting{Enabled: true}))

	cleared := false
	repo := &mocks.ScanEventRepo{ClearBlocksFunc: func() error {
		cleared = true
		return nil
	}}
	var syncErr error
	filter := &mocks.FirewallFilterRepo{SyncFunc: func(context.Context) error {
		check.True(t, cleared, check.Msgf("先删记录再下发"))
		return syncErr
	}}
	usecase := newScanEventUsecase(repo, filter)
	must.NoError(t, usecase.UpdateSetting(t.Context(), &biz.ScanSetting{}))
	check.Len(t, filter.SyncCalls(), 1)

	syncErr = errors.New("nft failed")
	must.Error(t, usecase.UpdateSetting(t.Context(), &biz.ScanSetting{}))
}
