package data

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"strings"

	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/pkg/types"
)

type containerNetworkRepo struct {
	dockerRepo
}

func NewContainerNetworkRepo(setting biz.SettingRepo) biz.ContainerNetworkRepo {
	return &containerNetworkRepo{dockerRepo{setting}}
}

// List 列出网络
func (r *containerNetworkRepo) List(ctx context.Context) ([]types.ContainerNetwork, error) {
	apiClient, err := r.client()
	if err != nil {
		return nil, err
	}
	defer func(apiClient *client.Client) { _ = apiClient.Close() }(apiClient)

	resp, err := apiClient.NetworkList(ctx, client.NetworkListOptions{})
	if err != nil {
		return nil, err
	}

	var networks []types.ContainerNetwork
	for _, item := range resp.Items {
		ipamConfigs := make([]types.ContainerNetworkIPAMConfig, 0)
		for _, ipam := range item.IPAM.Config {
			ipamConfigs = append(ipamConfigs, types.ContainerNetworkIPAMConfig{
				Subnet:     ipam.Subnet,
				IPRange:    ipam.IPRange,
				Gateway:    ipam.Gateway,
				AuxAddress: ipam.AuxAddress,
			})
		}
		networks = append(networks, types.ContainerNetwork{
			ID:         item.ID,
			Name:       item.Name,
			Driver:     item.Driver,
			IPv6:       item.EnableIPv6,
			Internal:   item.Internal,
			Attachable: item.Attachable,
			Ingress:    item.Ingress,
			Scope:      item.Scope,
			CreatedAt:  item.Created,
			IPAM: types.ContainerNetworkIPAM{
				Driver:  item.IPAM.Driver,
				Options: types.MapToKV(item.IPAM.Options),
				Config:  ipamConfigs,
			},
			Options: types.MapToKV(item.Options),
			Labels:  types.MapToKV(item.Labels),
		})
	}

	slices.SortFunc(networks, func(a types.ContainerNetwork, b types.ContainerNetwork) int {
		return strings.Compare(a.Name, b.Name)
	})

	return networks, nil
}

// Create 创建网络
func (r *containerNetworkRepo) Create(ctx context.Context, req *request.ContainerNetworkCreate) (string, error) {
	apiClient, err := r.client()
	if err != nil {
		return "", err
	}
	defer func(apiClient *client.Client) { _ = apiClient.Close() }(apiClient)

	var ipamConfigs []network.IPAMConfig
	for _, item := range []struct {
		family string
		conf   types.ContainerContainerNetwork
	}{{"ipv4", req.Ipv4}, {"ipv6", req.Ipv6}} {
		conf := item.conf
		if !conf.Enabled || (conf.Subnet == "" && conf.Gateway == "" && conf.IPRange == "") {
			continue
		}
		var config network.IPAMConfig
		if conf.Subnet != "" {
			config.Subnet, err = netip.ParsePrefix(conf.Subnet)
			if err != nil {
				return "", fmt.Errorf("invalid %s subnet: %w", item.family, err)
			}
		}
		if conf.Gateway != "" {
			config.Gateway, err = netip.ParseAddr(conf.Gateway)
			if err != nil {
				return "", fmt.Errorf("invalid %s gateway: %w", item.family, err)
			}
		}
		if conf.IPRange != "" {
			config.IPRange, err = netip.ParsePrefix(conf.IPRange)
			if err != nil {
				return "", fmt.Errorf("invalid %s ip range: %w", item.family, err)
			}
		}
		ipamConfigs = append(ipamConfigs, config)
	}

	options := client.NetworkCreateOptions{
		EnableIPv4: &req.Ipv4.Enabled,
		EnableIPv6: &req.Ipv6.Enabled,
		Driver:     req.Driver,
		Options:    types.KVToMap(req.Options),
		Labels:     types.KVToMap(req.Labels),
	}
	if len(ipamConfigs) > 0 {
		options.IPAM = &network.IPAM{
			Config: ipamConfigs,
		}
	}

	resp, err := apiClient.NetworkCreate(ctx, req.Name, options)
	if err != nil {
		return "", err
	}

	return resp.ID, err
}

// Remove 删除网络
func (r *containerNetworkRepo) Remove(ctx context.Context, id string) error {
	apiClient, err := r.client()
	if err != nil {
		return err
	}
	defer func(apiClient *client.Client) { _ = apiClient.Close() }(apiClient)

	// 拦截受保护网络的删除
	info, err := apiClient.NetworkInspect(ctx, id, client.NetworkInspectOptions{})
	if err != nil {
		return err
	}
	if info.Network.Name == "acepanel-network" {
		return errors.New("cannot delete acepanel-network")
	}

	// 中途取消会留下拆到一半的网络
	_, err = apiClient.NetworkRemove(context.WithoutCancel(ctx), id, client.NetworkRemoveOptions{})
	return err
}

// Prune 清理未使用的网络
func (r *containerNetworkRepo) Prune(ctx context.Context) error {
	apiClient, err := r.client()
	if err != nil {
		return err
	}
	defer func(apiClient *client.Client) { _ = apiClient.Close() }(apiClient)

	_, err = apiClient.NetworkPrune(context.WithoutCancel(ctx), client.NetworkPruneOptions{
		Filters: make(client.Filters).Add("label!", "created_by=acepanel"),
	})
	return err
}
