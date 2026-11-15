package service

import (
	"context"

	"github.com/urfave/cli/v3"

	"github.com/acepanel/panel/v3/internal/biz"
)

func (s *CliService) RestoreWebsite(ctx context.Context, cmd *cli.Command) error {
	return s.backupRepo.Restore(ctx, biz.BackupTypeWebsite, cmd.String("file"), cmd.String("name"))
}

func (s *CliService) RestoreDatabase(ctx context.Context, cmd *cli.Command) error {
	return s.backupRepo.Restore(ctx, biz.BackupType(cmd.String("type")), cmd.String("file"), cmd.String("name"))
}

// RestorePanel 从面板备份恢复，恢复完成后面板会自行重启
func (s *CliService) RestorePanel(ctx context.Context, cmd *cli.Command) error {
	return s.backupRepo.Restore(ctx, biz.BackupTypePanel, cmd.String("file"), "")
}
