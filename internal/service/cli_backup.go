package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/urfave/cli/v3"

	"github.com/acepanel/panel/v3/internal/biz"
)

func (s *CliService) BackupList(ctx context.Context, cmd *cli.Command) error {
	files, err := s.backupRepo.List(biz.BackupType(cmd.String("type")))
	if err != nil {
		return err
	}

	return s.printList(cmd, files, func() {
		for _, file := range files {
			fmt.Println(s.t.Get("Name: %s, Size: %s, Time: %s", file.Name, file.Size, file.Time.Format(time.DateTime)))
		}
	})
}

func (s *CliService) BackupWebsite(ctx context.Context, cmd *cli.Command) error {
	return s.backupRepo.Create(ctx, biz.BackupTypeWebsite, cmd.String("name"), cmd.Uint("storage"))
}

func (s *CliService) BackupDatabase(ctx context.Context, cmd *cli.Command) error {
	return s.backupRepo.Create(ctx, biz.BackupType(cmd.String("type")), cmd.String("name"), cmd.Uint("storage"))
}

func (s *CliService) BackupPath(ctx context.Context, cmd *cli.Command) error {
	return s.backupRepo.Create(ctx, biz.BackupTypePath, cmd.String("path"), cmd.Uint("storage"))
}

func (s *CliService) BackupPanel(ctx context.Context, cmd *cli.Command) error {
	fmt.Println(s.hr)
	fmt.Println(s.t.Get("★ Start backup [%s]", time.Now().Format(time.DateTime)))
	fmt.Println(s.hr)
	fmt.Println(s.t.Get("|-Backup type: panel"))
	if err := s.backupRepo.CreatePanel(ctx); err != nil {
		return errors.New(s.t.Get("Backup failed: %v", err))
	}
	fmt.Println(s.hr)
	fmt.Println(s.t.Get("☆ Backup successful [%s]", time.Now().Format(time.DateTime)))
	fmt.Println(s.hr)
	return nil
}

func (s *CliService) BackupClear(ctx context.Context, cmd *cli.Command) error {
	fmt.Println(s.hr)
	fmt.Println(s.t.Get("★ Start cleaning [%s]", time.Now().Format(time.DateTime)))
	fmt.Println(s.hr)
	fmt.Println(s.t.Get("|-Cleaning type: %s", cmd.String("type")))
	fmt.Println(s.t.Get("|-Cleaning target: %s", cmd.String("file")))
	fmt.Println(s.t.Get("|-Keep count: %d", cmd.Uint("keep")))

	if cmd.Uint("storage") != 0 {
		if err := s.backupRepo.ClearStorageExpired(ctx, cmd.Uint("storage"), cmd.String("type"), cmd.String("file"), cmd.Uint("keep")); err != nil {
			return errors.New(s.t.Get("Cleaning failed: %v", err))
		}
	} else {
		path := s.backupRepo.GetDefaultPath(biz.BackupType(cmd.String("type")))
		if err := s.backupRepo.ClearExpired(path, cmd.String("file"), cmd.Uint("keep")); err != nil {
			return errors.New(s.t.Get("Cleaning failed: %v", err))
		}
	}

	fmt.Println(s.hr)
	fmt.Println(s.t.Get("☆ Cleaning successful [%s]", time.Now().Format(time.DateTime)))
	fmt.Println(s.hr)
	return nil
}
