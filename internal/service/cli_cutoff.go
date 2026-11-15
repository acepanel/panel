package service

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/urfave/cli/v3"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/pkg/shell"
)

func (s *CliService) CutoffWebsite(ctx context.Context, cmd *cli.Command) error {
	website, err := s.websiteRepo.GetByName(cmd.String("name"))
	if err != nil {
		return err
	}
	path := filepath.Join(app.Root, "sites", website.Name, "log")

	fmt.Println(s.hr)
	fmt.Println(s.t.Get("★ Start log rotation [%s]", time.Now().Format(time.DateTime)))
	fmt.Println(s.hr)
	fmt.Println(s.t.Get("|-Rotation type: website"))
	fmt.Println(s.t.Get("|-Rotation target: %s", website.Name))

	var files []string
	zipPath, err := s.backupRepo.CutoffLog(ctx, path, filepath.Join(path, "access.log"))
	if err != nil {
		return err
	}
	files = append(files, zipPath)
	zipPath, err = s.backupRepo.CutoffLog(ctx, path, filepath.Join(path, "error.log"))
	if err != nil {
		return err
	}
	files = append(files, zipPath)

	// 上传到远程存储
	if cmd.Uint("storage") != 0 {
		if err = s.backupRepo.CutoffUpload(ctx, cmd.Uint("storage"), biz.BackupTypeWebsite, website.Name, files); err != nil {
			return err
		}
	}

	fmt.Println(s.hr)
	fmt.Println(s.t.Get("☆ Rotation successful [%s]", time.Now().Format(time.DateTime)))
	fmt.Println(s.hr)
	return nil
}

func (s *CliService) CutoffContainer(ctx context.Context, cmd *cli.Command) error {
	name := cmd.String("name")

	// 获取容器日志路径
	logPath, err := shell.Execf(ctx, "docker inspect --format='{{.LogPath}}' '%s'", name)
	if err != nil {
		return errors.New(s.t.Get("Failed to get container log path: %v", err))
	}
	logPath = strings.TrimSpace(logPath)
	if logPath == "" {
		return errors.New(s.t.Get("Container %s has no log file", name))
	}

	savePath := filepath.Join(app.Root, "server/cutoff/container", name)

	fmt.Println(s.hr)
	fmt.Println(s.t.Get("★ Start log rotation [%s]", time.Now().Format(time.DateTime)))
	fmt.Println(s.hr)
	fmt.Println(s.t.Get("|-Rotation type: container"))
	fmt.Println(s.t.Get("|-Rotation target: %s", name))

	zipPath, err := s.backupRepo.CutoffLog(ctx, savePath, logPath)
	if err != nil {
		return err
	}

	// 上传到远程存储
	if cmd.Uint("storage") != 0 {
		if err = s.backupRepo.CutoffUpload(ctx, cmd.Uint("storage"), "container", name, []string{zipPath}); err != nil {
			return err
		}
	}

	fmt.Println(s.hr)
	fmt.Println(s.t.Get("☆ Rotation successful [%s]", time.Now().Format(time.DateTime)))
	fmt.Println(s.hr)
	return nil
}

func (s *CliService) CutoffClear(ctx context.Context, cmd *cli.Command) error {
	typ := cmd.String("type")
	name := cmd.String("name")
	keep := cmd.Uint("keep")
	storageID := cmd.Uint("storage")

	fmt.Println(s.hr)
	fmt.Println(s.t.Get("★ Start cleaning rotated logs [%s]", time.Now().Format(time.DateTime)))
	fmt.Println(s.hr)
	fmt.Println(s.t.Get("|-Cleaning type: %s", typ))
	fmt.Println(s.t.Get("|-Cleaning target: %s", name))
	fmt.Println(s.t.Get("|-Keep count: %d", keep))

	// 本地切割目录与各自的文件前缀，访问日志和错误日志分别计份
	var path string
	var prefixes []string
	switch typ {
	case "website":
		website, err := s.websiteRepo.GetByName(name)
		if err != nil {
			return err
		}
		path = filepath.Join(app.Root, "sites", website.Name, "log")
		prefixes = []string{"access_", "error_"}
	case "container":
		path = filepath.Join(app.Root, "server/cutoff/container", name)
		prefixes = []string{""}
	default:
		return errors.New(s.t.Get("Unsupported rotation type: %s", typ))
	}

	for _, prefix := range prefixes {
		if err := s.backupRepo.ClearExpired(path, prefix, keep); err != nil {
			return err
		}
		// 清理远程存储过期日志，切割日志上传在 cutoff/<类型>/<目标> 下
		if storageID != 0 {
			if err := s.backupRepo.ClearStorageExpired(ctx, storageID, filepath.Join("cutoff", typ, name), prefix, keep); err != nil {
				return err
			}
		}
	}

	fmt.Println(s.hr)
	fmt.Println(s.t.Get("☆ Cleaning successful [%s]", time.Now().Format(time.DateTime)))
	fmt.Println(s.hr)
	return nil
}
