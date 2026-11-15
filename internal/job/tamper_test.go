package job

import (
	"errors"
	"log/slog"
	"testing"

	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/assert/check"
	"github.com/libtnb/assert/must"

	"github.com/acepanel/panel/v3/internal/biz"
	mocks "github.com/acepanel/panel/v3/internal/mocks/biz"
)

// 首次对账失败不算完成，下一轮要重试
func TestTamperRetriesReconcile(t *testing.T) {
	failing := true
	tamperRepo := &mocks.TamperRepo{
		ListRulesFunc: func() ([]*biz.TamperRule, error) {
			if failing {
				return nil, errors.New("database is locked")
			}
			return nil, nil
		},
	}
	settingRepo := &mocks.SettingRepo{
		GetFunc:     func(biz.SettingKey, ...string) (string, error) { return "", nil },
		GetBoolFunc: func(biz.SettingKey, ...bool) (bool, error) { return true, nil },
		GetIntFunc:  func(biz.SettingKey, ...int) (int, error) { return 0, nil },
	}
	locale := gotext.NewLocale("", "en")
	log := slog.New(slog.DiscardHandler)
	usecase := biz.NewTamperUsecase(nil, biz.NewSettingUsecase(locale, log, settingRepo, nil), locale, log, tamperRepo)
	task := &Tamper{log: log, tamperRepo: usecase, t: locale}

	must.NoError(t, task.Run(t.Context()))
	check.False(t, task.reconciled)
	check.Len(t, tamperRepo.ListRulesCalls(), 1)

	failing = false
	must.NoError(t, task.Run(t.Context()))
	check.True(t, task.reconciled)
	check.Len(t, tamperRepo.ListRulesCalls(), 2)

	// 成功后不再对账
	must.NoError(t, task.Run(t.Context()))
	check.Len(t, tamperRepo.ListRulesCalls(), 2)
}
