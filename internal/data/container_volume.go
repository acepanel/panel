package data

import (
	"context"
	"slices"
	"strings"
	"time"

	"github.com/moby/moby/client"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/pkg/tools"
	"github.com/acepanel/panel/v3/pkg/types"
)

type containerVolumeRepo struct {
	dockerRepo
}

func NewContainerVolumeRepo(setting biz.SettingRepo) biz.ContainerVolumeRepo {
	return &containerVolumeRepo{dockerRepo{setting}}
}

// List 列出存储卷
func (r *containerVolumeRepo) List(ctx context.Context) ([]types.ContainerVolume, error) {
	apiClient, err := r.client()
	if err != nil {
		return nil, err
	}
	defer func(apiClient *client.Client) { _ = apiClient.Close() }(apiClient)

	resp, err := apiClient.VolumeList(ctx, client.VolumeListOptions{})
	if err != nil {
		return nil, err
	}

	var volumes []types.ContainerVolume
	for _, item := range resp.Items {
		createdAt, _ := time.Parse(time.RFC3339Nano, item.CreatedAt)
		var refCount int64
		var size string
		if item.UsageData != nil {
			refCount = item.UsageData.RefCount
			size = tools.FormatBytes(float64(item.UsageData.Size))
		}
		volumes = append(volumes, types.ContainerVolume{
			Name:       item.Name,
			Driver:     item.Driver,
			Scope:      item.Scope,
			MountPoint: item.Mountpoint,
			CreatedAt:  createdAt,
			Labels:     types.MapToKV(item.Labels),
			Options:    types.MapToKV(item.Options),
			RefCount:   refCount,
			Size:       size,
		})
	}

	slices.SortFunc(volumes, func(a types.ContainerVolume, b types.ContainerVolume) int {
		return strings.Compare(a.Name, b.Name)
	})

	return volumes, nil
}

// Create 创建存储卷
func (r *containerVolumeRepo) Create(ctx context.Context, req *request.ContainerVolumeCreate) (string, error) {
	apiClient, err := r.client()
	if err != nil {
		return "", err
	}
	defer func(apiClient *client.Client) { _ = apiClient.Close() }(apiClient)

	resp, err := apiClient.VolumeCreate(ctx, client.VolumeCreateOptions{
		Name:       req.Name,
		Driver:     req.Driver,
		DriverOpts: types.KVToMap(req.Options),
		Labels:     types.KVToMap(req.Labels),
	})
	if err != nil {
		return "", err
	}

	return resp.Volume.Name, nil
}

// Remove 删除存储卷
func (r *containerVolumeRepo) Remove(ctx context.Context, id string) error {
	apiClient, err := r.client()
	if err != nil {
		return err
	}
	defer func(apiClient *client.Client) { _ = apiClient.Close() }(apiClient)

	// 中途取消会留下元数据已删但磁盘数据仍在的卷
	_, err = apiClient.VolumeRemove(context.WithoutCancel(ctx), id, client.VolumeRemoveOptions{
		Force: true,
	})
	return err
}

// Prune 清理未使用的存储卷
func (r *containerVolumeRepo) Prune(ctx context.Context) error {
	apiClient, err := r.client()
	if err != nil {
		return err
	}
	defer func(apiClient *client.Client) { _ = apiClient.Close() }(apiClient)

	_, err = apiClient.VolumePrune(context.WithoutCancel(ctx), client.VolumePruneOptions{
		Filters: make(client.Filters).Add("label!", "created_by=acepanel"),
	})
	return err
}
