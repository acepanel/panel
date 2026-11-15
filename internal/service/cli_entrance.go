package service

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/libtnb/utils/str"
	"github.com/urfave/cli/v3"

	"github.com/acepanel/panel/v3/pkg/config"
)

func (s *CliService) EntranceOn(ctx context.Context, cmd *cli.Command) error {
	// 支持自定义入口，未指定时随机生成
	entrance := strings.Trim(cmd.StringArg("entrance"), "/")
	if entrance == "" {
		entrance = str.Random(6)
	}
	if err := s.validate(ctx, &struct {
		Entrance string `validate:"entrance"`
	}{entrance}); err != nil {
		return err
	}

	if err := s.updateConfig(func(conf *config.Config) { conf.HTTP.Entrance = "/" + entrance }); err != nil {
		return err
	}

	fmt.Println(s.t.Get("Entrance enabled"))
	fmt.Println(s.t.Get("Entrance: %s", s.conf.HTTP.Entrance))
	s.printAccessAddresses() //nolint:contextcheck
	return s.Restart(ctx, cmd)
}

func (s *CliService) EntranceOff(ctx context.Context, cmd *cli.Command) error {
	if err := s.updateConfig(func(conf *config.Config) { conf.HTTP.Entrance = "/" }); err != nil {
		return err
	}

	fmt.Println(s.t.Get("Entrance disabled"))
	return s.Restart(ctx, cmd)
}

func (s *CliService) EntranceError(ctx context.Context, cmd *cli.Command) error {
	errorType := cmd.StringArg("type")
	if !slices.Contains([]string{"418", "nginx", "close"}, errorType) {
		return errors.New(s.t.Get("Error page type must be one of 418, nginx, close"))
	}

	if err := s.updateConfig(func(conf *config.Config) { conf.HTTP.EntranceError = errorType }); err != nil {
		return err
	}

	fmt.Println(s.t.Get("Entrance error page set to %s", errorType))
	return s.Restart(ctx, cmd)
}

func (s *CliService) BindDomainOn(ctx context.Context, cmd *cli.Command) error {
	domains := cmd.StringArgs("domain")
	if len(domains) == 0 {
		return errors.New(s.t.Get("Please specify at least one domain"))
	}

	if err := s.updateConfig(func(conf *config.Config) { conf.HTTP.BindDomain = domains }); err != nil {
		return err
	}

	fmt.Println(s.t.Get("Bind domain enabled: %s", strings.Join(domains, ", ")))
	return s.Restart(ctx, cmd)
}

func (s *CliService) BindDomainOff(ctx context.Context, cmd *cli.Command) error {
	if err := s.updateConfig(func(conf *config.Config) { conf.HTTP.BindDomain = nil }); err != nil {
		return err
	}

	fmt.Println(s.t.Get("Bind domain disabled"))
	return s.Restart(ctx, cmd)
}

func (s *CliService) BindIPOn(ctx context.Context, cmd *cli.Command) error {
	ips := cmd.StringArgs("ip")
	if len(ips) == 0 {
		return errors.New(s.t.Get("Please specify at least one IP"))
	}

	if err := s.updateConfig(func(conf *config.Config) { conf.HTTP.BindIP = ips }); err != nil {
		return err
	}

	fmt.Println(s.t.Get("Bind IP enabled: %s", strings.Join(ips, ", ")))
	return s.Restart(ctx, cmd)
}

func (s *CliService) BindIPOff(ctx context.Context, cmd *cli.Command) error {
	if err := s.updateConfig(func(conf *config.Config) { conf.HTTP.BindIP = nil }); err != nil {
		return err
	}

	fmt.Println(s.t.Get("Bind IP disabled"))
	return s.Restart(ctx, cmd)
}

func (s *CliService) BindUAOn(ctx context.Context, cmd *cli.Command) error {
	uas := cmd.StringArgs("user-agent")
	if len(uas) == 0 {
		return errors.New(s.t.Get("Please specify at least one User-Agent"))
	}

	if err := s.updateConfig(func(conf *config.Config) { conf.HTTP.BindUA = uas }); err != nil {
		return err
	}

	fmt.Println(s.t.Get("Bind UA enabled: %s", strings.Join(uas, ", ")))
	return s.Restart(ctx, cmd)
}

func (s *CliService) BindUAOff(ctx context.Context, cmd *cli.Command) error {
	if err := s.updateConfig(func(conf *config.Config) { conf.HTTP.BindUA = nil }); err != nil {
		return err
	}

	fmt.Println(s.t.Get("Bind UA disabled"))
	return s.Restart(ctx, cmd)
}
