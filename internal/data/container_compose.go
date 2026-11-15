package data

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/pkg/shell"
	"github.com/acepanel/panel/v3/pkg/types"
)

// composeEnv 禁用 Podman Compose 的警告日志
var composeEnv = []string{"PODMAN_COMPOSE_WARNING_LOGS=false"}

type containerComposeRepo struct{}

func NewContainerComposeRepo() biz.ContainerComposeRepo {
	return &containerComposeRepo{}
}

// List 列出所有编排
func (r *containerComposeRepo) List(ctx context.Context) ([]types.ContainerCompose, error) {
	// PODMAN_COMPOSE_WARNING_LOGS 禁用 Podman Compose 的警告日志
	raw, err := shell.ExecfWithEnv(ctx, composeEnv, "docker compose ls -a --format json")
	if err != nil {
		return nil, err
	}

	var composeRaws []types.ContainerComposeRaw
	if err = json.Unmarshal([]byte(raw), &composeRaws); err != nil {
		return nil, err
	}

	composeDir := filepath.Join(app.Root, "compose")
	entries, err := os.ReadDir(composeDir)
	if err != nil {
		return nil, err
	}

	var composes []types.ContainerCompose
	index := make(map[string]int)

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		path := filepath.Join(composeDir, entry.Name())
		var createdAt time.Time
		if info, err := entry.Info(); err == nil {
			createdAt = info.ModTime()
		}

		composes = append(composes, types.ContainerCompose{
			Name:      entry.Name(),
			Path:      path,
			Status:    "unknown",
			CreatedAt: createdAt,
		})
		index[entry.Name()] = len(composes) - 1
	}

	// 更新状态
	for _, item := range composeRaws {
		if item.ConfigFiles == "" {
			continue
		}
		// ConfigFiles 可能包含多个以逗号分隔的路径，取第一个
		first, _, _ := strings.Cut(item.ConfigFiles, ",")
		dir := filepath.Base(filepath.Dir(first))
		if i, ok := index[dir]; ok {
			composes[i].Status = item.Status
		}
	}

	return composes, nil
}

// Get 获取编排文件和环境变量内容
func (r *containerComposeRepo) Get(name string) (string, []types.KV, error) {
	content, _ := os.ReadFile(filepath.Join(app.Root, "compose", name, "docker-compose.yml"))
	env, _ := os.ReadFile(filepath.Join(app.Root, "compose", name, ".env"))

	envs := types.SliceToKV(strings.Split(string(env), "\n"))

	return string(content), envs, nil // 有意忽略错误，这样可以允许新建文件
}

// Create 创建编排文件
func (r *containerComposeRepo) Create(name, compose string, envs []types.KV) error {
	dir := filepath.Join(app.Root, "compose", name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return writeComposeFiles(dir, compose, envs)
}

// Update 更新编排文件
func (r *containerComposeRepo) Update(name, compose string, envs []types.KV) error {
	return writeComposeFiles(filepath.Join(app.Root, "compose", name), compose, envs)
}

// Up 启动编排
func (r *containerComposeRepo) Up(ctx context.Context, name string, force bool) error {
	file := filepath.Join(app.Root, "compose", name, "docker-compose.yml")
	cmd := "docker compose -f %s up -d"
	if force {
		cmd += " --pull always" // 强制拉取镜像
	}
	_, err := shell.ExecfWithEnv(ctx, composeEnv, cmd, file)
	return err
}

// Down 停止编排
func (r *containerComposeRepo) Down(ctx context.Context, name string) error {
	file := filepath.Join(app.Root, "compose", name, "docker-compose.yml")
	_, err := shell.ExecfWithEnv(ctx, composeEnv, "docker compose -f %s down", file)
	return err
}

// RemoveDir 删除编排目录
func (r *containerComposeRepo) RemoveDir(name string) error {
	dir := filepath.Join(app.Root, "compose", name)
	return os.RemoveAll(dir)
}

func writeComposeFiles(dir, compose string, envs []types.KV) error {
	if err := os.WriteFile(filepath.Join(dir, "docker-compose.yml"), []byte(compose), 0644); err != nil {
		return err
	}

	var sb strings.Builder
	for _, kv := range envs {
		sb.WriteString(kv.Key)
		sb.WriteString("=")
		sb.WriteString(kv.Value)
		sb.WriteString("\n")
	}

	return os.WriteFile(filepath.Join(dir, ".env"), []byte(sb.String()), 0644)
}
