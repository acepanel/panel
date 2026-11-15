package data

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/pkg/config"
	"github.com/acepanel/panel/v3/pkg/shell"
)

type environmentRepo struct {
	conf *config.Config
}

func NewEnvironmentRepo(conf *config.Config) biz.EnvironmentRepo {
	return &environmentRepo{conf: conf}
}

func (r *environmentRepo) IsInstalled(typ, slug string) bool {
	binFile := r.binPath(typ, slug)
	if binFile == "" {
		return false
	}

	_, err := os.Stat(binFile)
	return err == nil
}

func (r *environmentRepo) InstalledVersion(ctx context.Context, typ, slug string) string {
	if !r.IsInstalled(typ, slug) {
		return ""
	}

	bin := r.binPath(typ, slug)
	var version string
	var err error

	switch typ {
	case "go":
		// go version go1.21.0 linux/amd64 -> 1.21.0
		version, err = shell.Execf(ctx, "%s version | awk '{print $3}' | sed 's/go//'", bin)
	case "java":
		// OpenJDK Runtime Environment Corretto-21.0.9.11.1 (build 21.0.9+11-LTS) -> 21.0.9.11.1
		version, err = shell.Execf(ctx, `%s -version 2>&1 | sed -n 's/.*Corretto-\([0-9.]*\).*/\1/p' | head -n 1`, bin)
	case "nodejs":
		// v20.10.0 -> 20.10.0
		version, err = shell.Execf(ctx, "%s -v | sed 's/v//'", bin)
	case "php":
		// PHP 8.3.0 (cli) -> 8.3.0
		version, err = shell.Execf(ctx, "%s -d error_reporting=0 -r 'echo PHP_VERSION;'", bin)
	case "python":
		// Python 3.11.5 -> 3.11.5
		version, err = shell.Execf(ctx, "%s --version | awk '{print $2}'", bin)
	case "dotnet":
		// 8.0.100
		version, err = shell.Execf(ctx, "%s --version", bin)
	}

	if err != nil {
		return ""
	}
	return version
}

// binPath 返回运行时可执行文件路径，不支持的类型返回空
func (r *environmentRepo) binPath(typ, slug string) string {
	path := filepath.Join(app.Root, "server", typ, slug)
	switch typ {
	case "go":
		return filepath.Join(path, "bin", "go")
	case "java":
		return filepath.Join(path, "bin", "java")
	case "nodejs":
		return filepath.Join(path, "bin", "node")
	case "php":
		return filepath.Join(path, "bin", "php")
	case "python":
		return filepath.Join(path, "bin", "python3")
	case "dotnet":
		return filepath.Join(path, "dotnet")
	default:
		return ""
	}
}

func (r *environmentRepo) ScriptCommand(typ, action, slug, version string) string {
	shellUrl := fmt.Sprintf("https://%s/%s/%s.sh", r.conf.App.DownloadEndpoint, typ, action)
	return fmt.Sprintf(`curl -sSLm 10 --retry 3 "%s" | bash -s -- "%s" "%s"`, shellUrl, slug, version)
}

func (r *environmentRepo) ExecScript(ctx context.Context, cmd string) error {
	return shell.ExecWithOutput(ctx, cmd)
}
