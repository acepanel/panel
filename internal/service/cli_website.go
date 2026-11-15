package service

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"math"
	stdos "os"
	"path/filepath"
	"slices"

	"github.com/libtnb/utils/str"
	"github.com/urfave/cli/v3"
	"gorm.io/gorm"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/pkg/webserver/openlitespeed"
)

func (s *CliService) WebsiteCreate(ctx context.Context, cmd *cli.Command) error {
	req := &request.WebsiteCreate{
		Type:       cmd.String("type"),
		Name:       cmd.String("name"),
		Domains:    cmd.StringSlice("domains"),
		Listens:    cmd.StringSlice("listens"),
		Path:       cmd.String("path"),
		Remark:     cmd.String("remark"),
		PHP:        cmd.Uint("php"),
		Proxy:      cmd.String("proxy"),
		DB:         cmd.String("db") != "",
		DBType:     cmd.String("db"),
		DBName:     cmd.String("db-name"),
		DBUser:     cmd.String("db-user"),
		DBPassword: cmd.String("db-password"),
	}

	// 未指定目录时使用默认目录，与面板端行为保持一致
	if req.Path == "" {
		root, err := s.settingRepo.Get(biz.SettingKeyWebsitePath)
		if err != nil {
			return err
		}
		req.Path = filepath.Join(root, req.Name, "public")
	}
	// 数据库参数缺省时按网站名生成，密码随机
	if req.DB {
		req.DBName = cmp.Or(req.DBName, req.Name)
		req.DBUser = cmp.Or(req.DBUser, req.Name)
		req.DBPassword = cmp.Or(req.DBPassword, str.Random(16))
	}

	if err := s.validate(ctx, req); err != nil {
		return err
	}

	website, warnings, err := s.websiteRepo.Create(ctx, req)
	if err != nil {
		return err
	}

	fmt.Println(s.t.Get("Website %s created successfully", website.Name))
	fmt.Println(s.t.Get("Path: %s", req.Path))
	if req.DB {
		fmt.Println(s.t.Get("Database: %s, username: %s, password: %s", req.DBName, req.DBUser, req.DBPassword))
	}
	for _, warning := range warnings {
		fmt.Println(s.t.Get("Warning: %s", warning))
	}
	return nil
}

// WebsiteRebuild 按当前 Web 服务器重建全部站点配置，供切换 Web 服务器的安装脚本调用
func (s *CliService) WebsiteRebuild(ctx context.Context, cmd *cli.Command) error {
	websites, err := s.websiteRepo.All()
	if err != nil {
		return err
	}

	// 换用别的 Web 服务器时要把 php-fpm 拉回来
	webServer, _ := s.settingRepo.Get(biz.SettingKeyWebserver)
	if err = openlitespeed.SyncFPM(ctx, webServer == "openlitespeed"); err != nil {
		fmt.Printf("[WARN] php-fpm: %v\n", err)
	}

	for _, website := range websites {
		rebuilt, notes, err := s.websiteRepo.Rebuild(ctx, website)
		switch {
		case err != nil:
			fmt.Printf("[ERROR] %s: %v\n", website.Name, err)
		case !rebuilt:
			fmt.Printf("[SKIP] %s\n", website.Name)
		default:
			fmt.Printf("[OK] %s\n", website.Name)
			for _, note := range notes {
				fmt.Printf("[WARN] %s: %s\n", website.Name, note)
			}
		}
	}

	return s.ReloadWebserver(ctx, cmd)
}

func (s *CliService) WebsiteRemove(ctx context.Context, cmd *cli.Command) error {
	website, err := s.websiteRepo.GetByName(cmd.String("name"))
	if err != nil {
		return err
	}
	req := &request.WebsiteDelete{
		ID: website.ID,
	}

	if err = s.websiteRepo.Delete(ctx, req); err != nil {
		return err
	}

	fmt.Println(s.t.Get("Website %s removed successfully", website.Name))
	return nil
}

func (s *CliService) WebsiteDelete(ctx context.Context, cmd *cli.Command) error {
	website, err := s.websiteRepo.GetByName(cmd.String("name"))
	if err != nil {
		return err
	}
	req := &request.WebsiteDelete{
		ID:   website.ID,
		Path: true,
		DB:   true,
	}

	if err = s.websiteRepo.Delete(ctx, req); err != nil {
		return err
	}

	fmt.Println(s.t.Get("Website %s deleted successfully", website.Name))
	return nil
}

func (s *CliService) WebsiteList(ctx context.Context, cmd *cli.Command) error {
	websites, _, err := s.websiteRepo.List("all", "", 0, 1, math.MaxUint32)
	if err != nil {
		return err
	}

	return s.printList(cmd, websites, func() {
		for _, website := range websites {
			status := s.t.Get("stopped")
			if website.Status {
				status = s.t.Get("running")
			}
			fmt.Println(s.t.Get("ID: %d, Name: %s, Type: %s, Status: %s, Path: %s", website.ID, website.Name, string(website.Type), status, website.Path))
		}
	})
}

// WebsiteCert 从文件写入网站证书，供外部签发工具对接
func (s *CliService) WebsiteCert(ctx context.Context, cmd *cli.Command) error {
	certPEM, err := stdos.ReadFile(cmd.String("cert"))
	if err != nil {
		return errors.New(s.t.Get("Failed to read certificate file: %v", err))
	}
	keyPEM, err := stdos.ReadFile(cmd.String("key"))
	if err != nil {
		return errors.New(s.t.Get("Failed to read private key file: %v", err))
	}

	req := &request.WebsiteUpdateCert{
		Name: cmd.String("name"),
		Cert: string(certPEM),
		Key:  string(keyPEM),
	}
	if err = s.validate(ctx, req); err != nil {
		return err
	}

	if err = s.websiteRepo.UpdateCert(ctx, req); err != nil {
		return err
	}

	fmt.Println(s.t.Get("Certificate for website %s updated successfully", req.Name))
	return nil
}

// WebsiteWrite 仅写入网站数据到面板数据库，不创建网站目录和配置文件
func (s *CliService) WebsiteWrite(ctx context.Context, cmd *cli.Command) error {
	name := cmd.String("name")
	typ := biz.WebsiteType(cmd.String("type"))
	if !slices.Contains([]biz.WebsiteType{biz.WebsiteTypeProxy, biz.WebsiteTypeStatic, biz.WebsiteTypePHP}, typ) {
		return errors.New(s.t.Get("Unsupported website type: %s", cmd.String("type")))
	}

	path := cmd.String("path")
	if path == "" {
		root, err := s.settingRepo.Get(biz.SettingKeyWebsitePath)
		if err != nil {
			return err
		}
		path = filepath.Join(root, name, "public")
	}
	if !filepath.IsAbs(path) {
		return errors.New(s.t.Get("Website path must be an absolute path"))
	}

	// 已存在则覆盖，用于修复错误的网站数据
	website := new(biz.Website)
	if err := s.db.Where("name", name).First(website).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New(s.t.Get("Failed to get website: %v", err))
		}
	}
	website.Name = name
	website.Type = typ
	website.Path = path
	website.Status = cmd.Bool("status")
	website.SSL = cmd.Bool("ssl")
	website.Remark = cmd.String("remark")
	if err := s.db.Save(website).Error; err != nil {
		return errors.New(s.t.Get("Failed to save website: %v", err))
	}

	fmt.Println(s.t.Get("Website %s written successfully", name))
	return nil
}
