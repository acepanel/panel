package biz

import (
	"context"
	"errors"
	"log/slog"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/leonelquinteros/gotext"
	"github.com/samber/lo"
	"github.com/spf13/cast"
	"gorm.io/gorm"

	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/pkg/acme"
	"github.com/acepanel/panel/v3/pkg/types"
)

type WebsiteType string

const (
	WebsiteTypeProxy  WebsiteType = "proxy"
	WebsiteTypePHP    WebsiteType = "php"
	WebsiteTypeStatic WebsiteType = "static"
)

type Website struct {
	ID        uint        `gorm:"primaryKey" json:"id"`
	Name      string      `gorm:"not null;default:'';unique" json:"name"`
	Type      WebsiteType `gorm:"not null;index;default:'static'" json:"type"`
	Status    bool        `gorm:"not null;default:true" json:"status"`
	Path      string      `gorm:"not null;default:''" json:"path"`
	SSL       bool        `gorm:"not null;default:false" json:"ssl"`
	CertID    uint        `gorm:"not null;index;default:0" json:"cert_id"`    // 部署的证书 ID
	ProjectID uint        `gorm:"not null;index;default:0" json:"project_id"` // 绑定的项目 ID，0 表示未绑定
	Remark    string      `gorm:"not null;default:''" json:"remark"`
	ExpireAt  *time.Time  `json:"expire_at"` // 到期时间，nil 表示不限时
	CreatedAt time.Time   `json:"created_at"`
	UpdatedAt time.Time   `json:"updated_at"`

	CertExpire string   `gorm:"-:all" json:"cert_expire"` // 仅显示
	PHP        uint     `gorm:"-:all" json:"php"`         // 仅显示
	Domains    []string `gorm:"-:all" json:"domains"`     // 仅显示

	Cert *Cert `gorm:"foreignKey:CertID" json:"cert"`
}

type WebsiteRepo interface {
	GetRewrites() (map[string]string, error)
	UpdateDefaultConfig(ctx context.Context, req *request.WebsiteDefaultConfig) error
	Count() (int64, error)
	Get(id uint) (*types.WebsiteSetting, error)
	GetByName(name string) (*types.WebsiteSetting, error)
	List(typ, keyword string, projectID, page, limit uint) ([]*Website, int64, error)
	Create(ctx context.Context, req *request.WebsiteCreate) (*Website, error)
	Update(ctx context.Context, req *request.WebsiteUpdate) (*Website, error)
	SwitchType(ctx context.Context, req *request.WebsiteSwitchType) (*Website, error)
	GetForDelete(id uint) (*Website, error)
	RemoveFiles(ctx context.Context, name string, removePath bool) error
	Delete(website *Website) error
	ReloadWebServer(ctx context.Context) error
	UpdateRemark(id uint, remark string) error
	ResetConfig(ctx context.Context, id uint) error
	Rebuild(ctx context.Context, website *Website) (bool, []string, error)
	UpdateStatus(ctx context.Context, id uint, status bool) error
	UpdateExpireAt(id uint, expireAt *time.Time) error
	UpdateCert(ctx context.Context, req *request.WebsiteUpdateCert) error
	UpdateProject(id, projectID uint) error
	UnbindProject(projectID uint) error
}

type WebsiteUsecase struct {
	repo           WebsiteRepo
	log            *slog.Logger
	t              *gotext.Locale
	cert           *CertUsecase
	certAccount    *CertAccountUsecase
	database       *DatabaseUsecase
	databaseUser   *DatabaseUserUsecase
	databaseServer DatabaseServerRepo
	tamper         *TamperUsecase
	stat           *WebsiteStatUsecase
}

func NewWebsiteUsecase(certAccountUsecase *CertAccountUsecase, certUsecase *CertUsecase, databaseUsecase *DatabaseUsecase, databaseUserUsecase *DatabaseUserUsecase, tamperUsecase *TamperUsecase, websiteStatUsecase *WebsiteStatUsecase, t *gotext.Locale, log *slog.Logger, databaseServerRepo DatabaseServerRepo, websiteRepo WebsiteRepo) *WebsiteUsecase {
	return &WebsiteUsecase{
		repo:           websiteRepo,
		log:            log,
		t:              t,
		cert:           certUsecase,
		certAccount:    certAccountUsecase,
		database:       databaseUsecase,
		databaseUser:   databaseUserUsecase,
		databaseServer: databaseServerRepo,
		tamper:         tamperUsecase,
		stat:           websiteStatUsecase,
	}
}

func (uc *WebsiteUsecase) GetRewrites() (map[string]string, error) {
	return uc.repo.GetRewrites()
}

func (uc *WebsiteUsecase) UpdateDefaultConfig(ctx context.Context, req *request.WebsiteDefaultConfig) error {
	if err := uc.repo.UpdateDefaultConfig(ctx, req); err != nil {
		return err
	}

	uc.log.Info(uc.t.Get("website default config saved"), slog.String("type", OperationTypeWebsite), slog.Uint64("operator_id", OperatorID(ctx)), slog.Any("tls_versions", req.TLSVersions), slog.Bool("listen_ipv6", req.ListenIPv6), slog.Bool("custom_not_found", req.NotFound != ""))

	return nil
}

func (uc *WebsiteUsecase) Count() (int64, error) {
	return uc.repo.Count()
}

func (uc *WebsiteUsecase) Get(id uint) (*types.WebsiteSetting, error) {
	return uc.repo.Get(id)
}

func (uc *WebsiteUsecase) GetByName(name string) (*types.WebsiteSetting, error) {
	return uc.repo.GetByName(name)
}

func (uc *WebsiteUsecase) List(typ, keyword string, projectID, page, limit uint) ([]*Website, int64, error) {
	return uc.repo.List(typ, keyword, projectID, page, limit)
}

func (uc *WebsiteUsecase) Create(ctx context.Context, req *request.WebsiteCreate) (*Website, []string, error) {
	w, err := uc.repo.Create(ctx, req)
	if err != nil {
		return nil, nil, err
	}

	var warnings []string
	if err = uc.repo.ReloadWebServer(ctx); err != nil {
		warnings = append(warnings, uc.t.Get("failed to reload web server: %v", err))
	}
	if req.DB {
		if err = uc.createDatabase(ctx, req); err != nil {
			warnings = append(warnings, uc.t.Get("failed to create database: %v", err))
		}
	}

	uc.log.Info(uc.t.Get("website created"), slog.String("type", OperationTypeWebsite), slog.Uint64("operator_id", OperatorID(ctx)), slog.Uint64("id", uint64(w.ID)), slog.String("name", req.Name), slog.String("website_type", req.Type), slog.Any("domains", req.Domains), slog.String("path", req.Path), slog.Bool("db", req.DB), slog.String("db_name", lo.Ternary(req.DB, req.DBName, "")))

	return w, warnings, nil
}

func (uc *WebsiteUsecase) createDatabase(ctx context.Context, req *request.WebsiteCreate) error {
	name := "local_" + req.DBType
	server, err := uc.databaseServer.GetByName(ctx, name)
	if err != nil {
		return errors.New(uc.t.Get("can't find %s database server, please add it first", name))
	}
	return uc.database.Create(ctx, &request.DatabaseCreate{
		ServerID:   server.ID,
		Name:       req.DBName,
		CreateUser: true,
		Username:   req.DBUser,
		Password:   req.DBPassword,
		Host:       "localhost",
		Comment:    "website " + req.Name,
	})
}

func (uc *WebsiteUsecase) Update(ctx context.Context, req *request.WebsiteUpdate) error {
	website, err := uc.repo.Update(ctx, req)
	if err != nil {
		return err
	}

	// 记录日志
	uc.log.Info(uc.t.Get("website updated"), slog.String("type", OperationTypeWebsite), slog.Uint64("operator_id", OperatorID(ctx)), slog.Uint64("id", uint64(req.ID)), slog.String("name", website.Name), slog.Any("domains", req.Domains), slog.String("path", req.Path), slog.String("root", req.Root), slog.Bool("ssl", req.SSL), slog.Uint64("php", uint64(req.PHP)))

	return uc.repo.ReloadWebServer(ctx)
}

func (uc *WebsiteUsecase) SwitchType(ctx context.Context, req *request.WebsiteSwitchType) error {
	website, err := uc.repo.SwitchType(ctx, req)
	if err != nil {
		return err
	}

	uc.log.Info(uc.t.Get("website type switched"), slog.String("type", OperationTypeWebsite), slog.Uint64("operator_id", OperatorID(ctx)), slog.Uint64("id", uint64(req.ID)), slog.String("name", website.Name), slog.String("website_type", req.Type), slog.Uint64("php", uint64(req.PHP)), slog.String("proxy", req.Proxy))

	return nil
}

func (uc *WebsiteUsecase) Delete(ctx context.Context, req *request.WebsiteDelete) error {
	website, err := uc.repo.GetForDelete(req.ID)
	if err != nil {
		return err
	}

	// 删站是删文件、删库、删记录、reload 的连续破坏性动作，中途取消只会留下半删的站点
	ctx = context.WithoutCancel(ctx)

	// 清理防篡改规则
	if req.Path && website.Path != "" {
		if rules, listErr := uc.tamper.ListRules(); listErr == nil {
			cleaned := filepath.Clean(website.Path)
			for _, rule := range rules {
				if rule.Path != "" && filepath.Clean(rule.Path) == cleaned {
					_ = uc.tamper.deleteRule(rule.ID)
				}
			}
		}
	}

	_ = uc.repo.RemoveFiles(ctx, website.Name, req.Path)
	if req.DB {
		if mysql, err := uc.databaseServer.GetByName(ctx, "local_mysql"); err == nil {
			_ = uc.databaseUser.DeleteByNames(ctx, mysql.ID, []string{website.Name})
			_ = uc.database.Delete(ctx, mysql.ID, website.Name)
		}
		if postgres, err := uc.databaseServer.GetByName(ctx, "local_postgresql"); err == nil {
			_ = uc.databaseUser.DeleteByNames(ctx, postgres.ID, []string{website.Name})
			_ = uc.database.Delete(ctx, postgres.ID, website.Name)
		}
	}

	if err = uc.repo.Delete(website); err != nil {
		return err
	}

	// 清理该站点在统计库中的全部数据
	_ = uc.stat.DeleteBySite(website.Name)

	// 记录日志
	uc.log.Warn(uc.t.Get("website deleted"), slog.String("type", OperationTypeWebsite), slog.Uint64("operator_id", OperatorID(ctx)), slog.Uint64("id", uint64(req.ID)), slog.String("name", website.Name), slog.String("path", website.Path), slog.Bool("remove_path", req.Path), slog.Bool("remove_db", req.DB))

	return uc.repo.ReloadWebServer(ctx)
}

func (uc *WebsiteUsecase) UpdateRemark(ctx context.Context, id uint, remark string) error {
	website, err := uc.repo.GetForDelete(id)
	if err != nil {
		return err
	}
	if err = uc.repo.UpdateRemark(id, remark); err != nil {
		return err
	}

	uc.log.Info(uc.t.Get("website remark updated"), slog.String("type", OperationTypeWebsite), slog.Uint64("operator_id", OperatorID(ctx)), slog.Uint64("id", uint64(id)), slog.String("name", website.Name), slog.String("remark", remark))

	return nil
}

// UpdateProject 绑定到项目，projectID 为 0 时解除绑定
func (uc *WebsiteUsecase) UpdateProject(ctx context.Context, id, projectID uint) error {
	website, err := uc.repo.GetForDelete(id)
	if err != nil {
		return err
	}
	if err = uc.repo.UpdateProject(id, projectID); err != nil {
		return err
	}

	uc.log.Info(uc.t.Get("website project updated"), slog.String("type", OperationTypeWebsite), slog.Uint64("operator_id", OperatorID(ctx)), slog.Uint64("id", uint64(id)), slog.String("name", website.Name), slog.Uint64("project_id", uint64(projectID)))

	return nil
}

// ReleaseProject 处理项目删除后留下的网站：remove 为 true 时一并删除，否则解除绑定成为普通网站
func (uc *WebsiteUsecase) ReleaseProject(ctx context.Context, projectID uint, remove bool) error {
	if !remove {
		return uc.repo.UnbindProject(projectID)
	}

	websites, _, err := uc.repo.List("all", "", projectID, 1, 10000)
	if err != nil {
		return err
	}
	for _, website := range websites {
		if err = uc.Delete(ctx, &request.WebsiteDelete{ID: website.ID, Path: true}); err != nil {
			return err
		}
	}

	return nil
}

// Rebuild 按当前 Web 服务器重建站点配置
func (uc *WebsiteUsecase) Rebuild(ctx context.Context, website *Website) (bool, []string, error) {
	return uc.repo.Rebuild(ctx, website)
}

func (uc *WebsiteUsecase) ResetConfig(ctx context.Context, id uint) error {
	website, err := uc.repo.GetForDelete(id)
	if err != nil {
		return err
	}
	if err = uc.repo.ResetConfig(ctx, id); err != nil {
		return err
	}

	uc.log.Info(uc.t.Get("website config reset"), slog.String("type", OperationTypeWebsite), slog.Uint64("operator_id", OperatorID(ctx)), slog.Uint64("id", uint64(id)), slog.String("name", website.Name))

	return nil
}

func (uc *WebsiteUsecase) UpdateStatus(ctx context.Context, id uint, status bool) error {
	website, err := uc.repo.GetForDelete(id)
	if err != nil {
		return err
	}
	if err = uc.repo.UpdateStatus(ctx, id, status); err != nil {
		return err
	}

	uc.log.Info(uc.t.Get("website status updated"), slog.String("type", OperationTypeWebsite), slog.Uint64("operator_id", OperatorID(ctx)), slog.Uint64("id", uint64(id)), slog.String("name", website.Name), slog.Bool("status", status))

	return nil
}

func (uc *WebsiteUsecase) UpdateExpireAt(ctx context.Context, id uint, expireAt *time.Time) error {
	website, err := uc.repo.GetForDelete(id)
	if err != nil {
		return err
	}
	if err = uc.repo.UpdateExpireAt(id, expireAt); err != nil {
		return err
	}

	var expire string
	if expireAt != nil {
		expire = expireAt.Format(time.DateTime)
	}
	uc.log.Info(uc.t.Get("website expire time updated"), slog.String("type", OperationTypeWebsite), slog.Uint64("operator_id", OperatorID(ctx)), slog.Uint64("id", uint64(id)), slog.String("name", website.Name), slog.String("expire_at", expire))

	return nil
}

func (uc *WebsiteUsecase) UpdateCert(ctx context.Context, req *request.WebsiteUpdateCert) error {
	if err := uc.repo.UpdateCert(ctx, req); err != nil {
		return err
	}

	uc.log.Info(uc.t.Get("website cert updated"), slog.String("type", OperationTypeWebsite), slog.Uint64("operator_id", OperatorID(ctx)), slog.String("name", req.Name))

	return nil
}

func (uc *WebsiteUsecase) ObtainCert(ctx context.Context, id uint, dnsID uint) error {
	website, err := uc.repo.Get(id)
	if err != nil {
		return err
	}

	// 泛域名必须使用 DNS 验证
	hasWildcard := slices.ContainsFunc(website.Domains, func(d string) bool {
		return strings.Contains(d, "*")
	})
	if hasWildcard && dnsID == 0 {
		return errors.New(uc.t.Get("wildcard domains require DNS verification, please select a DNS provider"))
	}

	account, err := uc.certAccount.GetDefault(ctx, cast.ToUint(ctx.Value("user_id")))
	if err != nil {
		return err
	}

	newCert, err := uc.cert.GetByWebsite(website.ID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			newCert, err = uc.cert.repo.Create(&request.CertCreate{
				Type:        string(acme.KeyEC256),
				Domains:     website.Domains,
				AutoRenewal: true,
				AccountID:   account.ID,
				DNSID:       dnsID,
				WebsiteIDs:  []uint{website.ID},
			})
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}
	newCert.Domains = website.Domains
	newCert.DNSID = dnsID
	if err = uc.cert.Save(newCert); err != nil {
		return err
	}

	_, err = uc.cert.ObtainAutoWithProgressCallback(ctx, newCert.ID, nil)
	if err != nil {
		return err
	}

	_, _, err = uc.cert.deploy(ctx, newCert.ID, []uint{website.ID}, true)
	return err
}
