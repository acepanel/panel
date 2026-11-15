package data

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"iter"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/distribution/reference"
	"github.com/libtnb/utils/str"
	"github.com/moby/moby/api/types/image"
	"github.com/moby/moby/api/types/registry"
	"github.com/moby/moby/client"
	"github.com/samber/lo"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/pkg/shell"
	"github.com/acepanel/panel/v3/pkg/tools"
	"github.com/acepanel/panel/v3/pkg/types"
)

type containerImageRepo struct {
	dockerRepo
}

func NewContainerImageRepo(setting biz.SettingRepo) biz.ContainerImageRepo {
	return &containerImageRepo{dockerRepo{setting}}
}

// List 列出镜像
func (r *containerImageRepo) List(ctx context.Context) ([]types.ContainerImage, error) {
	apiClient, err := r.client()
	if err != nil {
		return nil, err
	}
	defer func(apiClient *client.Client) { _ = apiClient.Close() }(apiClient)

	resp, err := apiClient.ImageList(ctx, client.ImageListOptions{
		All: true,
	})
	if err != nil {
		return nil, err
	}

	images := lo.Map(resp.Items, func(item image.Summary, _ int) types.ContainerImage {
		return types.ContainerImage{
			ID:          item.ID,
			Containers:  item.Containers,
			RepoTags:    item.RepoTags,
			RepoDigests: item.RepoDigests,
			Size:        tools.FormatBytes(float64(item.Size)),
			Labels:      types.MapToKV(item.Labels),
			CreatedAt:   time.Unix(item.Created, 0),
		}
	})

	slices.SortFunc(images, func(a types.ContainerImage, b types.ContainerImage) int {
		return strings.Compare(a.ID, b.ID)
	})

	return images, nil
}

// Exist 检查镜像是否存在
func (r *containerImageRepo) Exist(ctx context.Context, name string) (bool, error) {
	apiClient, err := r.client()
	if err != nil {
		return false, err
	}
	defer func(apiClient *client.Client) { _ = apiClient.Close() }(apiClient)

	_, err = apiClient.ImageInspect(ctx, name)
	if err != nil {
		if cerrdefs.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

// Pull 拉取镜像
func (r *containerImageRepo) Pull(ctx context.Context, req *request.ContainerImagePull) error {
	apiClient, out, err := r.pull(ctx, req)
	if err != nil {
		return err
	}
	defer func(apiClient *client.Client) { _ = apiClient.Close() }(apiClient)
	defer func(out client.ImagePullResponse) { _ = out.Close() }(out)

	return out.Wait(ctx)
}

// PullStream 发起镜像拉取并返回进度流，流读到结束时释放连接
func (r *containerImageRepo) PullStream(
	ctx context.Context,
	req *request.ContainerImagePull,
) (iter.Seq2[biz.ImagePullProgress, error], error) {
	apiClient, out, err := r.pull(ctx, req)
	if err != nil {
		return nil, err
	}

	return func(yield func(biz.ImagePullProgress, error) bool) {
		defer func(apiClient *client.Client) { _ = apiClient.Close() }(apiClient)
		out.JSONMessages(ctx)(yield)
	}, nil
}

// pull 发起镜像拉取，成功时调用方负责关闭返回的客户端
func (r *containerImageRepo) pull(
	ctx context.Context,
	req *request.ContainerImagePull,
) (*client.Client, client.ImagePullResponse, error) {
	apiClient, err := r.client()
	if err != nil {
		return nil, nil, err
	}

	options := client.ImagePullOptions{}
	if req.Auth {
		authConfig := registry.AuthConfig{
			Username: req.Username,
			Password: req.Password,
		}
		encodedJSON, err := json.Marshal(authConfig) //nolint:gosec
		if err != nil {
			_ = apiClient.Close()
			return nil, nil, err
		}
		options.RegistryAuth = base64.URLEncoding.EncodeToString(encodedJSON)
	}

	out, err := apiClient.ImagePull(ctx, req.Name, options)
	if err != nil {
		_ = apiClient.Close()
		return nil, nil, err
	}

	return apiClient, out, nil
}

// PullShell 生成拉取镜像的命令，需要认证时返回配套的清理命令
func (r *containerImageRepo) PullShell(req *request.ContainerImagePull) (string, string, error) {
	if !req.Auth {
		return r.command("pull", req.Name), "", nil
	}

	named, err := reference.ParseNormalizedNamed(req.Name)
	if err != nil {
		return "", "", fmt.Errorf("invalid image reference: %w", err)
	}

	configDir := filepath.Join(os.TempDir(), "ace-docker-task-"+str.Random(16))
	cleanup := "rm -rf " + shell.Quote(configDir)
	shell := strings.Join([]string{
		"set -e",
		"mkdir -p " + shell.Quote(configDir),
		"trap " + shell.Quote(cleanup) + " EXIT",
		"printf %s " + shell.Quote(req.Password) + " | " + r.command("--config", configDir, "login", "--username", req.Username, "--password-stdin", reference.Domain(named)),
		r.command("--config", configDir, "pull", req.Name),
	}, "\n")

	return shell, cleanup, nil
}

// Remove 删除镜像
func (r *containerImageRepo) Remove(ctx context.Context, id string) error {
	apiClient, err := r.client()
	if err != nil {
		return err
	}
	defer func(apiClient *client.Client) { _ = apiClient.Close() }(apiClient)

	// 级联删除父层，中途取消会留下删了一半的镜像链
	_, err = apiClient.ImageRemove(context.WithoutCancel(ctx), id, client.ImageRemoveOptions{
		Force:         true,
		PruneChildren: true,
	})
	return err
}

// Prune 清理未使用的镜像
func (r *containerImageRepo) Prune(ctx context.Context) error {
	apiClient, err := r.client()
	if err != nil {
		return err
	}
	defer func(apiClient *client.Client) { _ = apiClient.Close() }(apiClient)

	_, err = apiClient.ImagePrune(context.WithoutCancel(ctx), client.ImagePruneOptions{
		Filters: make(client.Filters).
			Add("dangling", "false").
			Add("label!", "created_by=acepanel"),
	})
	return err
}
