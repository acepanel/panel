package biz_test

import (
	"context"
	"log/slog"
	"testing"

	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/assert/check"
	"github.com/libtnb/assert/must"

	"github.com/acepanel/panel/v3/internal/biz"
	mocks "github.com/acepanel/panel/v3/internal/mocks/biz"
	pkgcert "github.com/acepanel/panel/v3/pkg/cert"
)

// 文件没变时不保存也不分发：Save 没有桩，被调到就会 panic
func TestCertRenewFromPath(t *testing.T) {
	oldCert, oldKey, err := pkgcert.GenerateSelfSigned([]string{"old.test"})
	must.NoError(t, err)
	newCert, newKey, err := pkgcert.GenerateSelfSigned([]string{"new.test"})
	must.NoError(t, err)

	stored := &biz.Cert{ID: 1, Type: "upload", Cert: string(oldCert), Key: string(oldKey), CertPath: "/c.pem", KeyPath: "/k.pem"}
	files := [2]string{string(oldCert), string(oldKey)}
	repo := &mocks.CertRepo{
		GetFunc: func(uint) (*biz.Cert, error) {
			cert := *stored
			return &cert, nil
		},
		ReadCertFilesFunc: func(string, string) (string, string, error) { return files[0], files[1], nil },
	}
	usecase := biz.NewCertUsecase(gotext.NewLocale("", "en"), slog.New(slog.DiscardHandler), repo, &mocks.SettingRepo{})

	_, err = usecase.RenewWithProgressCallback(t.Context(), 1, nil)
	must.NoError(t, err)

	// 证书与私钥不成对时保持原样，否则部署后 Web 服务器重载失败
	files = [2]string{string(newCert), string(oldKey)}
	_, err = usecase.RenewWithProgressCallback(t.Context(), 1, nil)
	must.Error(t, err)

	files = [2]string{string(newCert), string(newKey)}
	repo.SaveFunc = func(cert *biz.Cert) error {
		stored = cert
		return nil
	}
	repo.RunScriptFunc = func(context.Context, *biz.Cert) error { return nil }
	_, err = usecase.RenewWithProgressCallback(t.Context(), 1, nil)
	must.NoError(t, err)
	must.Len(t, repo.SaveCalls(), 1)
	check.Equal(t, stored.Cert, string(newCert))
	check.Equal(t, stored.Key, string(newKey))
	check.DeepEqual(t, stored.Domains, []string{"new.test"})
}

func TestCertRenewPastedUpload(t *testing.T) {
	repo := &mocks.CertRepo{
		GetFunc: func(id uint) (*biz.Cert, error) { return &biz.Cert{ID: id, Type: "upload"}, nil },
	}
	usecase := biz.NewCertUsecase(gotext.NewLocale("", "en"), slog.New(slog.DiscardHandler), repo, &mocks.SettingRepo{})

	_, err := usecase.RenewWithProgressCallback(t.Context(), 1, nil)
	must.Error(t, err)
}
