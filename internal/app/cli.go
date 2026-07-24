package app

import (
	"context"
	"os"

	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/samber/do/v2"
	"github.com/urfave/cli/v3"

	"github.com/acepanel/panel/v3/pkg/apploader"
)

type Cli struct {
	cmd      *cli.Command
	migrator *gormigrate.Gormigrate
}

func NewCli(i do.Injector) (*Cli, error) {
	IsCli = true
	_ = do.MustInvoke[*apploader.Loader](i) // 强制构造 loader，触发全局应用注册
	return &Cli{
		cmd:      do.MustInvoke[*cli.Command](i),
		migrator: do.MustInvoke[*gormigrate.Gormigrate](i),
	}, nil
}

func (r *Cli) Run() error {
	// migrate database
	// 这里不处理错误，这么做是为了在异常时用户可以用 fix 命令尝试修复
	_ = r.migrator.Migrate()

	// 错误必须向上返回，调用方据此以非零码退出，后台任务才能正确判定失败
	return r.cmd.Run(context.TODO(), os.Args)
}
