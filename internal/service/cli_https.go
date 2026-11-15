package service

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/urfave/cli/v3"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/pkg/cert"
	"github.com/acepanel/panel/v3/pkg/config"
	"github.com/acepanel/panel/v3/pkg/io"
	"github.com/acepanel/panel/v3/pkg/tools"
)

func (s *CliService) HTTPSOn(ctx context.Context, cmd *cli.Command) error {
	if err := s.updateConfig(func(conf *config.Config) { conf.HTTP.TLS = "acme" }); err != nil {
		return err
	}

	fmt.Println(s.t.Get("HTTPS enabled"))
	return s.Restart(ctx, cmd)
}

func (s *CliService) HTTPSOff(ctx context.Context, cmd *cli.Command) error {
	if err := s.updateConfig(func(conf *config.Config) { conf.HTTP.TLS = "off" }); err != nil {
		return err
	}

	fmt.Println(s.t.Get("HTTPS disabled"))
	return s.Restart(ctx, cmd)
}

func (s *CliService) HTTPSGenerate(ctx context.Context, cmd *cli.Command) error {
	//nolint:contextcheck
	names := tools.CollectLocalNames()

	var crt, key []byte
	var err error

	// ACME 模式任一环节失败（含注册账户）都回退到自签，保证面板能启动
	switch s.conf.HTTP.TLS {
	case "self-signed", "off", "custom":
	default:
		crt, key, err = obtainPanelACME(ctx, s.t, s.userRepo, s.certAccountRepo, s.certRepo, s.conf.HTTP.BindDomain)
		if err == nil {
			fmt.Println(s.t.Get("Successfully obtained panel certificate via ACME"))
		} else {
			fmt.Println(s.t.Get("Failed to obtain panel certificate via ACME, using self-signed certificate: %v", err))
		}
	}

	if crt == nil || key == nil {
		crt, key, err = cert.GenerateSelfSigned(names)
		if err != nil {
			return err
		}
	}

	if err = io.Write(filepath.Join(app.Root, "panel/storage/cert.pem"), string(crt), 0600); err != nil {
		return err
	}
	if err = io.Write(filepath.Join(app.Root, "panel/storage/cert.key"), string(key), 0600); err != nil {
		return err
	}

	fmt.Println(s.t.Get("HTTPS certificate generated"))
	return s.Restart(ctx, cmd)
}
