package service

import (
	"context"
	"fmt"

	"github.com/urfave/cli/v3"

	"github.com/acepanel/panel/v3/pkg/systemctl"
)

func (s *CliService) Status(ctx context.Context, cmd *cli.Command) error {
	status, err := systemctl.Status(ctx, "acepanel")
	if err != nil {
		return err
	}

	statusStr := s.t.Get("unknown")
	switch status {
	case true:
		statusStr = s.t.Get("running")
	case false:
		statusStr = s.t.Get("stopped")
	}

	fmt.Println(s.t.Get("AcePanel service status: %s", statusStr))

	return nil
}

func (s *CliService) Restart(ctx context.Context, cmd *cli.Command) error {
	if err := systemctl.Restart(ctx, "acepanel"); err != nil {
		return err
	}
	fmt.Println(s.t.Get("AcePanel service restarted"))
	return nil
}

func (s *CliService) Stop(ctx context.Context, cmd *cli.Command) error {
	if err := systemctl.Stop(ctx, "acepanel"); err != nil {
		return err
	}
	fmt.Println(s.t.Get("AcePanel service stopped"))
	return nil
}

func (s *CliService) Start(ctx context.Context, cmd *cli.Command) error {
	if err := systemctl.Start(ctx, "acepanel"); err != nil {
		return err
	}
	fmt.Println(s.t.Get("AcePanel service started"))
	return nil
}
