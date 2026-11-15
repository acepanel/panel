package service

import (
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/leonelquinteros/gotext"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/pkg/shell"
	"github.com/acepanel/panel/v3/pkg/types"
)

// environmentRuntime 是一种语言环境的命令行入口，binDir 相对版本目录
type environmentRuntime struct {
	binDir string
	bins   []string
}

var environmentRuntimes = map[string]environmentRuntime{
	"go":     {binDir: "bin", bins: []string{"go", "gofmt"}},
	"python": {binDir: "bin", bins: []string{"python3", "pip3"}},
	"nodejs": {binDir: "bin", bins: []string{"node", "npm", "npx", "corepack"}},
	"java":   {binDir: "bin", bins: []string{"java", "javac", "jar", "jshell"}},
	"dotnet": {bins: []string{"dotnet"}},
	"php":    {binDir: "bin", bins: []string{"php"}},
}

// environmentMirror 是包管理器读写镜像源的方式，写入时地址直接接在 set 之后
type environmentMirror struct {
	bin      string
	get      string
	set      string
	fallback string
	logKey   string
	redact   func(string) string
}

var environmentMirrors = map[string]environmentMirror{
	"go":     {bin: "go", get: "env GOPROXY", set: "env -w GOPROXY=", fallback: "https://proxy.golang.org,direct", logKey: "proxy", redact: redactProxyList},
	"python": {bin: "pip3", get: "config --global get global.index-url", set: "config --global set global.index-url ", fallback: "https://pypi.org/simple", logKey: "mirror", redact: redactURL},
	"nodejs": {bin: "npm", get: "config get --global registry", set: "config set --global registry ", fallback: "https://registry.npmjs.org/", logKey: "registry", redact: redactURL},
}

type EnvironmentService struct {
	t               *gotext.Locale
	log             *slog.Logger
	environmentRepo *biz.EnvironmentUsecase
}

func NewEnvironmentService(
	environmentUsecase *biz.EnvironmentUsecase,
	t *gotext.Locale,
	log *slog.Logger,
) *EnvironmentService {
	return &EnvironmentService{
		t:               t,
		log:             log,
		environmentRepo: environmentUsecase,
	}
}

func (s *EnvironmentService) Types(w http.ResponseWriter, r *http.Request) {
	Success(w, s.environmentRepo.Types())
}

func (s *EnvironmentService) List(w http.ResponseWriter, r *http.Request) {
	typ := r.URL.Query().Get("type")
	query := strings.ToLower(r.URL.Query().Get("query"))
	onlyInstalled := r.URL.Query().Get("installed") == "true"
	all := s.environmentRepo.All()
	environments := make([]types.EnvironmentDetail, 0)
	for _, item := range all {
		if typ != "" && !strings.EqualFold(item.Type, typ) {
			continue
		}
		if query != "" &&
			!strings.Contains(strings.ToLower(item.Name), query) &&
			!strings.Contains(strings.ToLower(item.Description), query) {
			continue
		}
		installed := s.environmentRepo.IsInstalled(item.Type, item.Slug)
		if onlyInstalled && !installed {
			continue
		}
		// 性能优化，避免重复探测版本
		var installedVersion string
		if installed {
			installedVersion = s.environmentRepo.InstalledVersion(r.Context(), item.Type, item.Slug)
		}
		environments = append(environments, types.EnvironmentDetail{
			Type:             item.Type,
			Name:             item.Name,
			Description:      item.Description,
			Slug:             item.Slug,
			Version:          item.Version,
			InstalledVersion: installedVersion,
			Installed:        installed,
			HasUpdate:        installed && item.Version != "" && installedVersion != "" && item.Version != installedVersion,
			CustomSupported:  biz.CustomCompileEnv(item.Type),
		})
	}

	s.log.Info(s.t.Get("environment list viewed"), slog.String("type", biz.OperationTypeEnvironment), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("environment_type", typ), slog.String("query", query), slog.Bool("installed", onlyInstalled))

	Success(w, environments)
}

func (s *EnvironmentService) Install(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.EnvironmentAction](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.environmentRepo.Install(r.Context(), req.Type, req.Slug); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *EnvironmentService) Uninstall(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.EnvironmentAction](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.environmentRepo.Uninstall(r.Context(), req.Type, req.Slug); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *EnvironmentService) Update(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.EnvironmentAction](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.environmentRepo.Update(r.Context(), req.Type, req.Slug); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *EnvironmentService) IsInstalled(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.EnvironmentAction](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	installed := s.environmentRepo.IsInstalled(req.Type, req.Slug)
	Success(w, installed)
}

// environmentBase 是各语言环境服务共用的依赖与处理器
type environmentBase struct {
	t               *gotext.Locale
	log             *slog.Logger
	environmentRepo *biz.EnvironmentUsecase
}

func (s *environmentBase) setCli(w http.ResponseWriter, r *http.Request, typ string) {
	slug, ok := s.installedSlug(w, r, typ)
	if !ok {
		return
	}

	binPath, err := linkEnvironmentCLI(typ, slug)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("environment cli set"), slog.String("type", biz.OperationTypeEnvironment), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("environment_type", typ), slog.String("slug", slug), slog.String("bin_path", binPath))

	Success(w, nil)
}

// getMirror 读取包管理器的镜像源，logMsg 是已翻译的操作日志文案
func (s *environmentBase) getMirror(w http.ResponseWriter, r *http.Request, typ, logMsg string) {
	slug, ok := s.installedSlug(w, r, typ)
	if !ok {
		return
	}

	mirror := environmentMirrors[typ]
	value, err := shell.Execf(r.Context(), "%s %s", filepath.Join(environmentBinPath(typ, slug), mirror.bin), mirror.get)
	if err != nil {
		value = mirror.fallback
	}

	s.log.Info(logMsg, slog.String("type", biz.OperationTypeEnvironment), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("slug", slug))

	Success(w, strings.TrimSpace(value))
}

func (s *environmentBase) setMirror(w http.ResponseWriter, r *http.Request, typ, slug, value, logMsg string) {
	if !s.installed(w, typ, slug) {
		return
	}

	mirror := environmentMirrors[typ]
	if _, err := shell.Execf(r.Context(), "%s %s%s", filepath.Join(environmentBinPath(typ, slug), mirror.bin), mirror.set, shell.Quote(value)); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(logMsg, slog.String("type", biz.OperationTypeEnvironment), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("slug", slug), slog.String(mirror.logKey, mirror.redact(value)))

	Success(w, nil)
}

// installedSlug 绑定路径里的版本并确认已安装，失败时响应已写入
func (s *environmentBase) installedSlug(w http.ResponseWriter, r *http.Request, typ string) (string, bool) {
	req, err := Bind[request.EnvironmentSlug](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return "", false
	}

	return req.Slug, s.installed(w, typ, req.Slug)
}

// installed 版本未安装时写入错误响应；提示语的 msgid 必须是字面量，放不进表里
func (s *environmentBase) installed(w http.ResponseWriter, typ, slug string) bool {
	if s.environmentRepo.IsInstalled(typ, slug) {
		return true
	}

	var msg string
	switch typ {
	case "go":
		msg = s.t.Get("Go-%s is not installed", slug)
	case "python":
		msg = s.t.Get("Python-%s is not installed", slug)
	case "nodejs":
		msg = s.t.Get("Node.js-%s is not installed", slug)
	case "java":
		msg = s.t.Get("Java-%s is not installed", slug)
	case "dotnet":
		msg = s.t.Get(".NET-%s is not installed", slug)
	}
	Error(w, http.StatusUnprocessableEntity, "%s", msg)
	return false
}

func environmentBinPath(typ, slug string) string {
	return filepath.Join(app.Root, "server", typ, slug, environmentRuntimes[typ].binDir)
}

// linkEnvironmentCLI 把指定版本的可执行文件链接到 /usr/local/bin，返回它们所在的目录
func linkEnvironmentCLI(typ, slug string) (string, error) {
	binPath := environmentBinPath(typ, slug)
	for _, name := range environmentRuntimes[typ].bins {
		link := filepath.Join("/usr/local/bin", name)
		_ = os.Remove(link)
		if err := os.Symlink(filepath.Join(binPath, name), link); err != nil {
			return binPath, err
		}
	}
	return binPath, nil
}

// redactURL 去掉 URL 中的账号密码，解析失败返回空串
func redactURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	u.User = nil
	return u.String()
}

// redactProxyList GOPROXY 可以是逗号或竖线分隔的多个地址，逐个脱敏
func redactProxyList(raw string) string {
	return regexp.MustCompile(`[^,|]+`).ReplaceAllStringFunc(raw, redactURL)
}
