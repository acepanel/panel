package biz_test

import (
	"log/slog"
	"testing"

	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/assert/check"
	"github.com/libtnb/assert/must"

	"github.com/acepanel/panel/v3/internal/biz"
	mocks "github.com/acepanel/panel/v3/internal/mocks/biz"
	"github.com/acepanel/panel/v3/internal/request"
)

// 改备注只读写面板记录，不连数据库：Get 与 Operator 没有桩，被调到就会 panic
func TestDatabaseUserUpdateRemarkWithoutConnecting(t *testing.T) {
	repo := &mocks.DatabaseUserRepo{
		FindFunc: func(id uint) (*biz.DatabaseUser, error) {
			return &biz.DatabaseUser{ID: id, ServerID: 1, Username: "app"}, nil
		},
		UpdateRemarkFunc: func(*request.DatabaseUserUpdateRemark) error { return nil },
	}
	usecase := biz.NewDatabaseUserUsecase(gotext.NewLocale("", "en"), slog.New(slog.DiscardHandler), &mocks.DatabaseServerRepo{}, repo)

	must.NoError(t, usecase.UpdateRemark(t.Context(), &request.DatabaseUserUpdateRemark{ID: 7, Remark: "note"}))
	check.Len(t, repo.FindCalls(), 1)
	must.Len(t, repo.UpdateRemarkCalls(), 1)
	check.Equal(t, repo.UpdateRemarkCalls()[0].Req.Remark, "note")
}
