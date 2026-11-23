package biz_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/assert/check"
	"github.com/libtnb/assert/must"

	"github.com/acepanel/panel/v3/internal/biz"
	mocks "github.com/acepanel/panel/v3/internal/mocks/biz"
	"github.com/acepanel/panel/v3/internal/request"
)

func newFirewallSubscriptionUsecase(repo *mocks.FirewallSubscriptionRepo) *biz.FirewallSubscriptionUsecase {
	return biz.NewFirewallSubscriptionUsecase(gotext.NewLocale("", "en"), slog.New(slog.DiscardHandler), repo)
}

func TestFirewallSubscriptionRefreshDue(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name   string
		sub    biz.FirewallSubscription
		cached bool
		want   bool
	}{
		{
			name:   "周期未到",
			sub:    biz.FirewallSubscription{Schedule: "@every 1h", RefreshedAt: now.Add(-10 * time.Minute)},
			cached: true,
		},
		{
			name:   "周期已到",
			sub:    biz.FirewallSubscription{Schedule: "@every 1h", RefreshedAt: now.Add(-61 * time.Minute)},
			cached: true,
			want:   true,
		},
		{
			name:   "面板停机错过的周期补上",
			sub:    biz.FirewallSubscription{Schedule: "0 3 * * *", RefreshedAt: now.Add(-72 * time.Hour)},
			cached: true,
			want:   true,
		},
		{
			name: "缓存丢失时不等周期",
			sub:  biz.FirewallSubscription{Schedule: "@every 1h", RefreshedAt: now.Add(-10 * time.Minute)},
			want: true,
		},
		{
			name: "缓存丢失但上次拉取失败，仍按周期重试",
			sub:  biz.FirewallSubscription{Schedule: "@every 1h", RefreshedAt: now.Add(-10 * time.Minute), Error: "timeout"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sub := tt.sub
			sub.ID = 1
			repo := &mocks.FirewallSubscriptionRepo{
				ListEnabledFunc: func() ([]*biz.FirewallSubscription, error) {
					return []*biz.FirewallSubscription{&sub}, nil
				},
				CachedFunc: func(uint) bool { return tt.cached },
				FetchFunc: func(_ context.Context, sub *biz.FirewallSubscription) error {
					sub.Count = 42
					return nil
				},
				SaveFunc: func(*biz.FirewallSubscription) error { return nil },
			}

			check.Equal(t, newFirewallSubscriptionUsecase(repo).RefreshDue(t.Context()), tt.want)
			check.Len(t, repo.FetchCalls(), map[bool]int{true: 1}[tt.want])
			if tt.want {
				check.Equal(t, sub.Count, uint(42))
				check.Equal(t, sub.Error, "")
				check.True(t, !sub.RefreshedAt.Before(now))
			}
		})
	}
}

// 拉取失败只记下原因和时间，不算更新成功，下次仍按周期重试
func TestFirewallSubscriptionRefreshDueFailure(t *testing.T) {
	sub := &biz.FirewallSubscription{ID: 1, Schedule: "@every 1h", Count: 7}
	repo := &mocks.FirewallSubscriptionRepo{
		ListEnabledFunc: func() ([]*biz.FirewallSubscription, error) {
			return []*biz.FirewallSubscription{sub}, nil
		},
		CachedFunc: func(uint) bool { return true },
		FetchFunc:  func(context.Context, *biz.FirewallSubscription) error { return errors.New("boom") },
		SaveFunc:   func(*biz.FirewallSubscription) error { return nil },
	}
	usecase := newFirewallSubscriptionUsecase(repo)

	check.False(t, usecase.RefreshDue(t.Context()))
	must.Len(t, repo.SaveCalls(), 1)
	check.Equal(t, sub.Error, "boom")
	check.Equal(t, sub.Count, uint(7))

	check.False(t, usecase.RefreshDue(t.Context()))
	check.Len(t, repo.FetchCalls(), 1)
}

// 列表拉不下来或规则下发不了时不留下订阅
func TestFirewallSubscriptionCreateRollback(t *testing.T) {
	req := &request.FirewallSubscriptionCreate{Name: "test", URL: "https://example.com/list.txt", Type: biz.FirewallSubscriptionBlock, Schedule: "0 3 * * *", Status: true}
	tests := []struct {
		name     string
		fetchErr error
		syncErr  error
		wantErr  bool
	}{
		{name: "成功"},
		{name: "拉取失败", fetchErr: errors.New("404"), wantErr: true},
		{name: "下发失败", syncErr: errors.New("nft: Operation not permitted"), wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mocks.FirewallSubscriptionRepo{
				CreateFunc: func(sub *biz.FirewallSubscription) error {
					sub.ID = 1
					return nil
				},
				FetchFunc:  func(context.Context, *biz.FirewallSubscription) error { return tt.fetchErr },
				SaveFunc:   func(*biz.FirewallSubscription) error { return nil },
				SyncFunc:   func(context.Context) error { return tt.syncErr },
				DeleteFunc: func(*biz.FirewallSubscription) error { return nil },
			}

			_, err := newFirewallSubscriptionUsecase(repo).Create(t.Context(), req)
			check.Equal(t, err != nil, tt.wantErr)
			check.Len(t, repo.DeleteCalls(), map[bool]int{true: 1}[tt.wantErr])
		})
	}
}

// 更新周期要能推算下次时间，秒级表达式和 @reboot 不接受；Create 没有桩，被调到就会 panic
func TestFirewallSubscriptionCreateInvalidSchedule(t *testing.T) {
	usecase := newFirewallSubscriptionUsecase(&mocks.FirewallSubscriptionRepo{})
	for _, schedule := range []string{"@reboot", "*/5 * * * * *", "61 * * * *"} {
		_, err := usecase.Create(t.Context(), &request.FirewallSubscriptionCreate{Schedule: schedule})
		check.Error(t, err, check.Msgf("应拒绝: %s", schedule))
	}
}

// 只改名称、周期这些时不重新拉取：Fetch 没有桩，被调到就会 panic
func TestFirewallSubscriptionUpdate(t *testing.T) {
	stored := &biz.FirewallSubscription{ID: 1, Name: "old", URL: "https://example.com/a.txt", Type: biz.FirewallSubscriptionBlock, Schedule: "0 3 * * *", Status: true, Count: 5}
	repo := &mocks.FirewallSubscriptionRepo{
		GetFunc: func(uint) (*biz.FirewallSubscription, error) {
			sub := *stored
			return &sub, nil
		},
		SaveFunc: func(sub *biz.FirewallSubscription) error {
			stored = sub
			return nil
		},
		SyncFunc: func(context.Context) error { return nil },
	}
	usecase := newFirewallSubscriptionUsecase(repo)

	req := &request.FirewallSubscriptionUpdate{ID: 1}
	req.Name, req.URL, req.Type, req.Schedule = "new", stored.URL, biz.FirewallSubscriptionAllow, "@every 6h"
	must.NoError(t, usecase.Update(t.Context(), req))
	check.Equal(t, stored.Name, "new")
	check.Equal(t, stored.Type, biz.FirewallSubscriptionAllow)
	check.False(t, stored.Status)
	check.Len(t, repo.SyncCalls(), 1)

	// 换了地址要先拉取成功才保存
	req.URL = "https://example.com/b.txt"
	repo.FetchFunc = func(context.Context, *biz.FirewallSubscription) error { return errors.New("404") }
	must.Error(t, usecase.Update(t.Context(), req))
	check.Equal(t, stored.URL, "https://example.com/a.txt")

	repo.FetchFunc = func(_ context.Context, sub *biz.FirewallSubscription) error {
		sub.Count = 9
		return nil
	}
	must.NoError(t, usecase.Update(t.Context(), req))
	check.Equal(t, stored.URL, "https://example.com/b.txt")
	check.Equal(t, stored.Count, uint(9))
}
