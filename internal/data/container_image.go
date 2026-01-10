package data

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"slices"
	"strings"
	"time"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/registry"
	"github.com/moby/moby/client"

	"github.com/acepanel/panel/internal/biz"
	"github.com/acepanel/panel/internal/http/request"
	"github.com/acepanel/panel/pkg/tools"
	"github.com/acepanel/panel/pkg/types"
)

type containerImageRepo struct{}

func NewContainerImageRepo() biz.ContainerImageRepo {
	return &containerImageRepo{}
}

// List 列出镜像
func (r *containerImageRepo) List() ([]types.ContainerImage, error) {
	apiClient, err := getDockerClient("/var/run/docker.sock")
	if err != nil {
		return nil, err
	}
	defer func(apiClient *client.Client) { _ = apiClient.Close() }(apiClient)

	resp, err := apiClient.ImageList(context.Background(), client.ImageListOptions{
		All: true,
	})
	if err != nil {
		return nil, err
	}

	var images []types.ContainerImage
	for _, item := range resp.Items {
		images = append(images, types.ContainerImage{
			ID:          item.ID,
			Containers:  item.Containers,
			RepoTags:    item.RepoTags,
			RepoDigests: item.RepoDigests,
			Size:        tools.FormatBytes(float64(item.Size)),
			Labels:      types.MapToKV(item.Labels),
			CreatedAt:   time.Unix(item.Created, 0),
		})
	}

	slices.SortFunc(images, func(a types.ContainerImage, b types.ContainerImage) int {
		return strings.Compare(a.ID, b.ID)
	})

	return images, nil
}

// Exist 检查镜像是否存在
func (r *containerImageRepo) Exist(name string) (bool, error) {
	apiClient, err := getDockerClient("/var/run/docker.sock")
	if err != nil {
		return false, err
	}
	defer func(apiClient *client.Client) { _ = apiClient.Close() }(apiClient)

	_, err = apiClient.ImageInspect(context.Background(), name)
	if err != nil {
		if cerrdefs.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

// Pull 拉取镜像
func (r *containerImageRepo) Pull(req *request.ContainerImagePull) error {
	apiClient, err := getDockerClient("/var/run/docker.sock")
	if err != nil {
		return err
	}
	defer func(apiClient *client.Client) { _ = apiClient.Close() }(apiClient)

	options := client.ImagePullOptions{}
	if req.Auth {
		authConfig := registry.AuthConfig{
			Username: req.Username,
			Password: req.Password,
		}
		encodedJSON, err := json.Marshal(authConfig)
		if err != nil {
			return err
		}
		authStr := base64.URLEncoding.EncodeToString(encodedJSON)
		options.RegistryAuth = authStr
	}

	out, err := apiClient.ImagePull(context.Background(), req.Name, options)
	if err != nil {
		return err
	}
	defer func(out client.ImagePullResponse) { _ = out.Close() }(out)

	// TODO 实现流式显示拉取进度
	return out.Wait(context.Background())
}

// Remove 删除镜像
func (r *containerImageRepo) Remove(id string) error {
	apiClient, err := getDockerClient("/var/run/docker.sock")
	if err != nil {
		return err
	}
	defer func(apiClient *client.Client) { _ = apiClient.Close() }(apiClient)

	_, err = apiClient.ImageRemove(context.Background(), id, client.ImageRemoveOptions{
		Force:         true,
		PruneChildren: true,
	})
	return err
}

// Prune 清理未使用的镜像
func (r *containerImageRepo) Prune() error {
	apiClient, err := getDockerClient("/var/run/docker.sock")
	if err != nil {
		return err
	}
	defer func(apiClient *client.Client) { _ = apiClient.Close() }(apiClient)

	_, err = apiClient.ImagePrune(context.Background(), client.ImagePruneOptions{})
	return err
}
