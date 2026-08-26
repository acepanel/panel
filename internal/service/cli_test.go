package service

import (
	"bytes"
	"context"
	stdos "os"
	"strings"
	"testing"

	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/sqlite"
	"github.com/libtnb/validator"
	"github.com/stretchr/testify/suite"
	"github.com/urfave/cli/v3"
	"gorm.io/gorm"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/internal/rule"
)

type CliTestSuite struct {
	suite.Suite
	cli *CliService
}

func TestCliTestSuite(t *testing.T) {
	suite.Run(t, &CliTestSuite{})
}

func (s *CliTestSuite) SetupTest() {
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{SkipDefaultTransaction: true})
	s.Require().NoError(err)
	s.Require().NoError(db.AutoMigrate(&biz.Website{}, &biz.DatabaseServer{}))

	// 预置一条网站记录，用于验证 not_exists / exists 规则确实查库
	s.Require().NoError(db.Create(&biz.Website{Name: "taken", Type: biz.WebsiteTypeStatic, Path: "/www/taken"}).Error)

	// 与 bootstrap.NewValidator 等价的构建方式，此处不能导入 bootstrap（会与 apps 形成循环）
	v := validator.MustNew(
		validator.WithStrictRequired(),
		validator.WithRules(rule.Rules()...),
		validator.WithFallibleRules(rule.FallibleRules(db)...),
	)

	s.cli = &CliService{
		t:         gotext.NewLocale("", ""),
		db:        db,
		validator: v,
	}
}

// TestValidate 校验 CLI 注入的验证器可用，含需要查库的自定义规则
func (s *CliTestSuite) TestValidate() {
	cases := []struct {
		name    string
		req     any
		wantErr bool
	}{
		{"合法静态站", &request.WebsiteCreate{
			Type: "static", Name: "ok", Listens: []string{"80"}, Domains: []string{"a.com"}, Path: "/www/ok",
		}, false},
		{"非法类型", &request.WebsiteCreate{
			Type: "bogus", Name: "ok", Listens: []string{"80"}, Domains: []string{"a.com"}, Path: "/www/ok",
		}, true},
		{"名称已存在", &request.WebsiteCreate{
			Type: "static", Name: "taken", Listens: []string{"80"}, Domains: []string{"a.com"}, Path: "/www/x",
		}, true},
		{"proxy 缺代理地址", &request.WebsiteCreate{
			Type: "proxy", Name: "ok2", Listens: []string{"80"}, Domains: []string{"a.com"},
		}, true},
		{"域名重复", &request.WebsiteCreate{
			Type: "static", Name: "ok3", Listens: []string{"80"}, Domains: []string{"a.com", "a.com"},
		}, true},
		{"建库参数缺失", &request.WebsiteCreate{
			Type: "static", Name: "ok4", Listens: []string{"80"}, Domains: []string{"a.com"}, DB: true,
		}, true},
		{"合法数据库服务器", &request.DatabaseServerCreate{
			Type: "mysql", Name: "srv", Host: "127.0.0.1", Port: 3306,
		}, false},
		{"数据库服务器类型非法", &request.DatabaseServerCreate{
			Type: "bogus", Name: "srv2", Host: "127.0.0.1", Port: 3306,
		}, true},
		{"端口越界", &request.DatabaseServerCreate{
			Type: "mysql", Name: "srv3", Host: "127.0.0.1", Port: 70000,
		}, true},
		{"证书目标网站不存在", &request.WebsiteUpdateCert{
			Name: "nope", Cert: "x", Key: "y",
		}, true},
		{"证书目标网站存在", &request.WebsiteUpdateCert{
			Name: "taken", Cert: "x", Key: "y",
		}, false},
	}

	for _, c := range cases {
		err := s.cli.validate(context.Background(), c.req)
		if c.wantErr {
			s.Error(err, c.name)
			continue
		}
		s.NoError(err, c.name)
	}
}

// TestPrintList 校验 --json 作为全局 flag 在子命令中透传
func (s *CliTestSuite) TestPrintList() {
	data := []biz.Website{{Name: "a", Type: biz.WebsiteTypeStatic}}

	cases := []struct {
		name     string
		args     []string
		wantJSON bool
	}{
		{"默认人类可读", []string{"acepanel", "list"}, false},
		{"--json 在子命令前", []string{"acepanel", "--json", "list"}, true},
		{"--json 在子命令后", []string{"acepanel", "list", "--json"}, true},
	}

	for _, c := range cases {
		var plainCalled bool
		out := s.captureStdout(func() {
			root := &cli.Command{
				Name:  "acepanel",
				Flags: []cli.Flag{&cli.BoolFlag{Name: "json"}},
				Commands: []*cli.Command{{
					Name: "list",
					Action: func(ctx context.Context, cmd *cli.Command) error {
						return s.cli.printList(cmd, data, func() { plainCalled = true })
					},
				}},
			}
			s.Require().NoError(root.Run(context.Background(), c.args))
		})

		s.Equal(c.wantJSON, strings.Contains(out, `"name": "a"`), c.name)
		s.Equal(c.wantJSON, !plainCalled, c.name)
	}
}

// TestReadSecret 校验密码取值优先级：参数 > 环境变量 > 交互输入
func (s *CliTestSuite) TestReadSecret() {
	s.T().Setenv("TEST_PWD", "from-env")

	got, err := s.cli.readSecret("from-arg", "TEST_PWD", "> ")
	s.NoError(err)
	s.Equal("from-arg", got)

	got, err = s.cli.readSecret("", "TEST_PWD", "> ")
	s.NoError(err)
	s.Equal("from-env", got)

	// 参数与环境变量均缺失且非终端时，给出可读提示而非 ioctl 错误
	s.T().Setenv("TEST_PWD", "")
	_, err = s.cli.readSecret("", "TEST_PWD", "> ")
	s.Error(err)
	s.Contains(err.Error(), "TEST_PWD")
}

func (s *CliTestSuite) captureStdout(fn func()) string {
	orig := stdos.Stdout
	r, w, err := stdos.Pipe()
	s.Require().NoError(err)

	stdos.Stdout = w
	fn()
	s.Require().NoError(w.Close())
	stdos.Stdout = orig

	var buf bytes.Buffer
	_, err = buf.ReadFrom(r)
	s.Require().NoError(err)

	return buf.String()
}
