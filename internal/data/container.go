package data

import (
	"bytes"
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"net/netip"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/pkg/docker"
	"github.com/acepanel/panel/v3/pkg/shell"
	"github.com/acepanel/panel/v3/pkg/types"
)

// dockerRepo 按面板设置的容器 socket 访问 Docker，四个容器仓储共用
type dockerRepo struct {
	setting biz.SettingRepo
}

func (r dockerRepo) sock() string {
	return biz.ContainerSock(r.setting)
}

func (r dockerRepo) client() (*client.Client, error) {
	return client.New(client.WithHost(r.sock()))
}

// command 拼接指向面板所设 socket 的 docker 命令，各参数均做转义
func (r dockerRepo) command(args ...string) string {
	args = append([]string{"docker", "--host", r.sock()}, args...)
	for i := range args {
		args[i] = shell.Quote(args[i])
	}
	return strings.Join(args, " ")
}

type containerRepo struct {
	dockerRepo
}

func NewContainerRepo(setting biz.SettingRepo) biz.ContainerRepo {
	return &containerRepo{dockerRepo{setting}}
}

// ListAll 列出所有容器
func (r *containerRepo) ListAll(ctx context.Context) ([]types.Container, error) {
	apiClient, err := r.client()
	if err != nil {
		return nil, err
	}
	defer func(apiClient *client.Client) { _ = apiClient.Close() }(apiClient)

	resp, err := apiClient.ContainerList(ctx, client.ContainerListOptions{
		All: true,
	})
	if err != nil {
		return nil, err
	}

	var containers []types.Container
	for _, item := range resp.Items {
		ports := make([]types.ContainerPort, len(item.Ports))
		for i, port := range item.Ports {
			ports[i] = types.ContainerPort{
				ContainerStart: uint(port.PrivatePort),
				ContainerEnd:   uint(port.PrivatePort),
				HostStart:      uint(port.PublicPort),
				HostEnd:        uint(port.PublicPort),
				Protocol:       port.Type,
				Host:           port.IP,
			}
		}
		slices.SortFunc(ports, func(a, b types.ContainerPort) int {
			aOffset := int64(a.HostStart) - int64(a.ContainerStart) //nolint:gosec
			if a.HostStart == 0 {
				aOffset = 0
			}
			bOffset := int64(b.HostStart) - int64(b.ContainerStart) //nolint:gosec
			if b.HostStart == 0 {
				bOffset = 0
			}
			return cmp.Or(
				a.Host.Compare(b.Host),
				strings.Compare(a.Protocol, b.Protocol),
				cmp.Compare(aOffset, bOffset),
				cmp.Compare(a.ContainerStart, b.ContainerStart),
				cmp.Compare(a.HostStart, b.HostStart),
			)
		})

		merged := ports[:0]
		for _, port := range ports {
			if len(merged) > 0 {
				last := &merged[len(merged)-1]
				unmapped := last.HostStart == 0 && port.HostStart == 0
				if last.Host == port.Host && last.Protocol == port.Protocol &&
					last.ContainerEnd+1 == port.ContainerStart && (unmapped || last.HostEnd+1 == port.HostStart) {
					last.ContainerEnd = port.ContainerEnd
					last.HostEnd = port.HostEnd
					continue
				}
			}
			merged = append(merged, port)
		}

		if len(item.Names) == 0 {
			item.Names = append(item.Names, "")
		}
		containers = append(containers, types.Container{
			ID:        item.ID,
			Name:      strings.TrimPrefix(item.Names[0], "/"), // https://github.com/moby/moby/issues/7519
			Image:     item.Image,
			ImageID:   item.ImageID,
			Command:   item.Command,
			CreatedAt: time.Unix(item.Created, 0),
			State:     string(item.State),
			Status:    item.Status,
			Ports:     merged,
			Labels:    types.MapToKV(item.Labels),
		})
	}

	slices.SortFunc(containers, func(a types.Container, b types.Container) int {
		return strings.Compare(a.Name, b.Name)
	})

	return containers, nil
}

// Inspect 获取容器详细信息
func (r *containerRepo) Inspect(ctx context.Context, id string) (any, error) {
	apiClient, err := r.client()
	if err != nil {
		return nil, err
	}
	defer func(apiClient *client.Client) { _ = apiClient.Close() }(apiClient)

	resp, err := apiClient.ContainerInspect(ctx, id, client.ContainerInspectOptions{})
	if err != nil {
		return nil, err
	}

	return resp.Container, nil
}

// Create 创建容器
func (r *containerRepo) Create(ctx context.Context, req *request.ContainerCreate) (string, error) {
	apiClient, err := r.client()
	if err != nil {
		return "", err
	}
	defer func(apiClient *client.Client) { _ = apiClient.Close() }(apiClient)

	// 获取镜像信息
	image, err := apiClient.ImageInspect(ctx, req.Image)
	if err != nil {
		return "", fmt.Errorf("failed to inspect image: %w", err)
	}
	// 兼容一些没有指定命令和入口点的镜像
	if image.Config != nil {
		if len(req.Command) == 0 && len(image.Config.Cmd) > 0 {
			req.Command = image.Config.Cmd
		}
		if len(req.Entrypoint) == 0 && len(image.Config.Entrypoint) > 0 {
			req.Entrypoint = image.Config.Entrypoint
		}
	}

	// 构建容器配置
	config := &container.Config{
		Image:        req.Image,
		Hostname:     req.Hostname,
		WorkingDir:   req.WorkingDir,
		User:         req.User,
		Tty:          req.Tty,
		OpenStdin:    req.OpenStdin,
		AttachStdin:  req.OpenStdin,
		AttachStdout: true,
		AttachStderr: true,
		Env:          types.KVToSlice(req.Env),
		Labels:       types.KVToMap(req.Labels),
		Entrypoint:   req.Entrypoint,
		Cmd:          req.Command,
		StopSignal:   req.StopSignal,
	}
	if req.StopTimeout > 0 {
		config.StopTimeout = &req.StopTimeout
	}
	if hc := req.Healthcheck; hc != nil {
		config.Healthcheck = &container.HealthConfig{
			Test: hc.Test, Interval: time.Duration(hc.Interval) * time.Second, Timeout: time.Duration(hc.Timeout) * time.Second,
			StartPeriod: time.Duration(hc.StartPeriod) * time.Second, Retries: hc.Retries,
		}
	}

	// 构建主机配置
	hostConfig := &container.HostConfig{
		AutoRemove:      req.AutoRemove,
		Privileged:      req.Privileged,
		PublishAllPorts: req.PublishAllPorts,
		ReadonlyRootfs:  req.ReadonlyRootfs,
		ExtraHosts:      req.ExtraHosts,
		CapAdd:          req.CapAdd,
		CapDrop:         req.CapDrop,
		SecurityOpt:     req.SecurityOpt,
		Sysctls:         types.KVToMap(req.Sysctls),
		Tmpfs:           types.KVToMap(req.Tmpfs),
		ShmSize:         req.ShmSize * 1024 * 1024,
	}
	if req.Init {
		hostConfig.Init = &req.Init
	}
	for _, dns := range req.DNS {
		address, parseErr := netip.ParseAddr(dns)
		if parseErr != nil {
			return "", fmt.Errorf("invalid DNS server address: %w", parseErr)
		}
		hostConfig.DNS = append(hostConfig.DNS, address)
	}
	// 与 docker run --device 的默认值对齐，API 收到空权限会让容器无法启动
	for _, device := range req.Devices {
		hostConfig.Devices = append(hostConfig.Devices, container.DeviceMapping{
			PathOnHost: device.Host, PathInContainer: cmp.Or(device.Container, device.Host), CgroupPermissions: cmp.Or(device.Permissions, "rwm"),
		})
	}
	for _, ulimit := range req.Ulimits {
		hostConfig.Ulimits = append(hostConfig.Ulimits, &container.Ulimit{Name: ulimit.Name, Soft: ulimit.Soft, Hard: ulimit.Hard})
	}

	// 同 docker run：网络名兼作 NetworkMode 与端点 key，预置网络上的别名和静态 IP 交给 Docker 报错
	networkConfig := &network.NetworkingConfig{}
	if req.Network != "" {
		endpoint := &network.EndpointSettings{Aliases: req.NetworkAliases}
		if req.StaticIP != "" {
			address, parseErr := netip.ParseAddr(req.StaticIP)
			if parseErr != nil {
				return "", fmt.Errorf("invalid static IP address: %w", parseErr)
			}
			// IPAddress 只是运行时回显，指定的 IP 要写在 IPAMConfig 里才生效
			endpoint.IPAMConfig = &network.EndpointIPAMConfig{IPv4Address: address}
			if address.Is6() {
				endpoint.IPAMConfig = &network.EndpointIPAMConfig{IPv6Address: address}
			}
		}
		hostConfig.NetworkMode = container.NetworkMode(req.Network)
		networkConfig.EndpointsConfig = map[string]*network.EndpointSettings{req.Network: endpoint}
	}

	// 设置端口映射
	if !req.PublishAllPorts && len(req.Ports) > 0 {
		portMap := make(network.PortMap)
		for _, port := range req.Ports {
			if port.ContainerStart > port.ContainerEnd || port.HostStart > port.HostEnd || port.ContainerStart < 1 || port.HostStart < 1 {
				return "", errors.New("port range is invalid")
			}
			containerCount, hostCount := port.ContainerEnd-port.ContainerStart+1, port.HostEnd-port.HostStart+1
			if containerCount != hostCount {
				return "", fmt.Errorf("container port and host port count do not match (container: %d host: %d)", containerCount, hostCount)
			}

			count := uint(0)
			for i := port.HostStart; i <= port.HostEnd; i++ {
				bindItem := network.PortBinding{HostIP: port.Host, HostPort: strconv.Itoa(int(i))}
				portMap[network.MustParsePort(fmt.Sprintf("%d/%s", port.ContainerStart+count, port.Protocol))] = []network.PortBinding{bindItem}
				count++
			}
		}

		exposed := make(network.PortSet)
		for port := range portMap {
			exposed[port] = struct{}{}
		}

		config.ExposedPorts = exposed
		hostConfig.PortBindings = portMap
	}
	// 设置卷挂载
	volumes := make(map[string]struct{})
	for _, v := range req.Volumes {
		volumes[v.Container] = struct{}{}
		hostConfig.Binds = append(hostConfig.Binds, fmt.Sprintf("%s:%s:%s", v.Host, v.Container, v.Mode))
	}
	config.Volumes = volumes
	// 设置重启策略
	hostConfig.RestartPolicy = container.RestartPolicy{Name: container.RestartPolicyMode(req.RestartPolicy)}
	if req.RestartPolicy == "on-failure" {
		hostConfig.RestartPolicy.MaximumRetryCount = 5
	}
	// 设置资源限制
	hostConfig.CPUShares = req.CPUShares
	hostConfig.NanoCPUs = int64(req.CPUs * 1e9)
	hostConfig.Memory = req.Memory * 1024 * 1024
	hostConfig.MemorySwap = 0

	// 创建容器
	resp, err := apiClient.ContainerCreate(ctx, client.ContainerCreateOptions{
		Name:             req.Name,
		Config:           config,
		HostConfig:       hostConfig,
		NetworkingConfig: networkConfig,
	})
	if err != nil {
		return "", err
	}

	return resp.ID, nil
}

// CreateShell 生成后台创建容器的命令
func (r *containerRepo) CreateShell(req *request.ContainerCreate) (string, error) {
	args := []string{"run", "--detach"}

	if req.Name != "" {
		args = append(args, "--name", req.Name)
	}
	if req.Network != "" {
		args = append(args, "--network", req.Network)
	}
	if req.Hostname != "" {
		args = append(args, "--hostname", req.Hostname)
	}
	if req.WorkingDir != "" {
		args = append(args, "--workdir", req.WorkingDir)
	}
	if req.User != "" {
		args = append(args, "--user", req.User)
	}
	for _, alias := range req.NetworkAliases {
		args = append(args, "--network-alias", alias)
	}
	if req.StaticIP != "" {
		flag := "--ip"
		if strings.Contains(req.StaticIP, ":") {
			flag = "--ip6"
		}
		args = append(args, flag, req.StaticIP)
	}
	for _, dns := range req.DNS {
		args = append(args, "--dns", dns)
	}
	for _, host := range req.ExtraHosts {
		args = append(args, "--add-host", host)
	}
	for _, capability := range req.CapAdd {
		args = append(args, "--cap-add", capability)
	}
	for _, capability := range req.CapDrop {
		args = append(args, "--cap-drop", capability)
	}
	for _, device := range req.Devices {
		args = append(args, "--device", r.mountSpec(device.Host, cmp.Or(device.Container, device.Host), device.Permissions))
	}
	for _, option := range req.SecurityOpt {
		args = append(args, "--security-opt", option)
	}
	for _, sysctl := range req.Sysctls {
		args = append(args, "--sysctl", sysctl.Key+"="+sysctl.Value)
	}
	for _, ulimit := range req.Ulimits {
		args = append(args, "--ulimit", fmt.Sprintf("%s=%d:%d", ulimit.Name, ulimit.Soft, ulimit.Hard))
	}
	for _, tmpfs := range req.Tmpfs {
		value := tmpfs.Key
		if tmpfs.Value != "" {
			value += ":" + tmpfs.Value
		}
		args = append(args, "--tmpfs", value)
	}
	if req.ShmSize > 0 {
		args = append(args, "--shm-size", strconv.FormatInt(req.ShmSize, 10)+"m")
	}
	if req.Init {
		args = append(args, "--init")
	}
	if req.StopSignal != "" {
		args = append(args, "--stop-signal", req.StopSignal)
	}
	if req.StopTimeout > 0 {
		args = append(args, "--stop-timeout", strconv.Itoa(req.StopTimeout))
	}
	if req.ReadonlyRootfs {
		args = append(args, "--read-only")
	}
	if hc := req.Healthcheck; hc != nil && len(hc.Test) > 0 && hc.Test[0] == "NONE" {
		args = append(args, "--no-healthcheck")
	} else if hc != nil {
		// Test 为空表示沿用镜像的检查命令，只覆盖时长与次数
		if len(hc.Test) > 0 {
			test := hc.Test
			if test[0] == "CMD" || test[0] == "CMD-SHELL" {
				test = test[1:]
			}
			args = append(args, "--health-cmd", strings.Join(test, " "))
		}
		if hc.Interval > 0 {
			args = append(args, "--health-interval", strconv.Itoa(hc.Interval)+"s")
		}
		if hc.Timeout > 0 {
			args = append(args, "--health-timeout", strconv.Itoa(hc.Timeout)+"s")
		}
		if hc.StartPeriod > 0 {
			args = append(args, "--health-start-period", strconv.Itoa(hc.StartPeriod)+"s")
		}
		if hc.Retries > 0 {
			args = append(args, "--health-retries", strconv.Itoa(hc.Retries))
		}
	}
	if req.PublishAllPorts {
		args = append(args, "--publish-all")
	} else {
		for _, port := range req.Ports {
			if port.ContainerStart < 1 || port.ContainerEnd > 65535 || port.ContainerStart > port.ContainerEnd ||
				port.HostStart < 1 || port.HostEnd > 65535 || port.HostStart > port.HostEnd {
				return "", errors.New("port range is invalid")
			}
			if port.ContainerEnd-port.ContainerStart != port.HostEnd-port.HostStart {
				return "", errors.New("container port and host port count do not match")
			}

			for offset := uint(0); offset <= port.HostEnd-port.HostStart; offset++ {
				host := ""
				if port.Host.IsValid() {
					host = port.Host.String() + ":"
					if port.Host.Is6() {
						host = "[" + port.Host.String() + "]:"
					}
				}
				mapping := fmt.Sprintf("%s%d:%d/%s", host, port.HostStart+offset, port.ContainerStart+offset, port.Protocol)
				args = append(args, "--publish", mapping)
			}
		}
	}

	for _, volume := range req.Volumes {
		args = append(args, "--volume", r.mountSpec(volume.Host, volume.Container, volume.Mode))
	}
	for _, env := range req.Env {
		args = append(args, "--env", env.Key+"="+env.Value)
	}
	for _, label := range req.Labels {
		args = append(args, "--label", label.Key+"="+label.Value)
	}

	if len(req.Entrypoint) > 0 {
		args = append(args, "--entrypoint", req.Entrypoint[0])
	}
	if req.RestartPolicy != "" {
		restartPolicy := req.RestartPolicy
		if restartPolicy == "on-failure" {
			restartPolicy += ":5"
		}
		args = append(args, "--restart", restartPolicy)
	}
	if req.AutoRemove {
		args = append(args, "--rm")
	}
	if req.Privileged {
		args = append(args, "--privileged")
	}
	if req.OpenStdin {
		args = append(args, "--interactive")
	}
	if req.Tty {
		args = append(args, "--tty")
	}
	if req.CPUShares > 0 {
		args = append(args, "--cpu-shares", strconv.FormatInt(req.CPUShares, 10))
	}
	if req.CPUs > 0 {
		args = append(args, "--cpus", strconv.FormatFloat(req.CPUs, 'f', -1, 64))
	}
	if req.Memory > 0 {
		args = append(args, "--memory", strconv.FormatInt(req.Memory, 10)+"m")
	}

	args = append(args, req.Image)
	if len(req.Entrypoint) > 1 {
		args = append(args, req.Entrypoint[1:]...)
	}
	args = append(args, req.Command...)

	return r.command(args...), nil
}

// mountSpec 拼接 docker 的 源:目标[:选项] 形式，
// 选项为空时不能留下尾随冒号，否则 docker 报 empty section between colons
func (r *containerRepo) mountSpec(source, target, option string) string {
	spec := source + ":" + target
	if option != "" {
		spec += ":" + option
	}
	return spec
}

// UpdateShell 生成后台重建容器的命令
func (r *containerRepo) UpdateShell(id string, req *request.ContainerCreate) (string, error) {
	create, err := r.CreateShell(req)
	if err != nil {
		return "", err
	}

	return strings.Join([]string{
		"set -e",
		r.command("image", "inspect", req.Image) + " >/dev/null 2>&1 || " + r.command("pull", req.Image),
		r.command("rm", "--force", id),
		create,
	}, "\n"), nil
}

// Remove 移除容器
func (r *containerRepo) Remove(ctx context.Context, id string) error {
	apiClient, err := r.client()
	if err != nil {
		return err
	}
	defer func(apiClient *client.Client) { _ = apiClient.Close() }(apiClient)

	// 中途取消会把容器留在删除中的半状态
	_, err = apiClient.ContainerRemove(context.WithoutCancel(ctx), id, client.ContainerRemoveOptions{
		Force: true,
	})
	return err
}

// Start 启动容器
func (r *containerRepo) Start(ctx context.Context, id string) error {
	apiClient, err := r.client()
	if err != nil {
		return err
	}
	defer func(apiClient *client.Client) { _ = apiClient.Close() }(apiClient)

	_, err = apiClient.ContainerStart(ctx, id, client.ContainerStartOptions{})
	return err
}

// Stop 停止容器
func (r *containerRepo) Stop(ctx context.Context, id string) error {
	apiClient, err := r.client()
	if err != nil {
		return err
	}
	defer func(apiClient *client.Client) { _ = apiClient.Close() }(apiClient)

	// 内部是 SIGTERM 到超时后 SIGKILL 的序列，中途取消会把容器留在中间态
	_, err = apiClient.ContainerStop(context.WithoutCancel(ctx), id, client.ContainerStopOptions{})
	return err
}

// Restart 重启容器
func (r *containerRepo) Restart(ctx context.Context, id string) error {
	apiClient, err := r.client()
	if err != nil {
		return err
	}
	defer func(apiClient *client.Client) { _ = apiClient.Close() }(apiClient)

	// 内部先停后启，中途取消会把容器留在停止状态
	_, err = apiClient.ContainerRestart(context.WithoutCancel(ctx), id, client.ContainerRestartOptions{})
	return err
}

// Pause 暂停容器
func (r *containerRepo) Pause(ctx context.Context, id string) error {
	apiClient, err := r.client()
	if err != nil {
		return err
	}
	defer func(apiClient *client.Client) { _ = apiClient.Close() }(apiClient)

	_, err = apiClient.ContainerPause(ctx, id, client.ContainerPauseOptions{})
	return err
}

// Unpause 恢复容器
func (r *containerRepo) Unpause(ctx context.Context, id string) error {
	apiClient, err := r.client()
	if err != nil {
		return err
	}
	defer func(apiClient *client.Client) { _ = apiClient.Close() }(apiClient)

	_, err = apiClient.ContainerUnpause(ctx, id, client.ContainerUnpauseOptions{})
	return err
}

// Kill 杀死容器
func (r *containerRepo) Kill(ctx context.Context, id string) error {
	apiClient, err := r.client()
	if err != nil {
		return err
	}
	defer func(apiClient *client.Client) { _ = apiClient.Close() }(apiClient)

	_, err = apiClient.ContainerKill(ctx, id, client.ContainerKillOptions{
		Signal: "KILL",
	})
	return err
}

// Rename 重命名容器
func (r *containerRepo) Rename(ctx context.Context, id string, newName string) error {
	apiClient, err := r.client()
	if err != nil {
		return err
	}
	defer func(apiClient *client.Client) { _ = apiClient.Close() }(apiClient)

	_, err = apiClient.ContainerRename(ctx, id, client.ContainerRenameOptions{
		NewName: newName,
	})
	return err
}

// Logs 查看容器末尾 tail 行日志
func (r *containerRepo) Logs(ctx context.Context, id string, tail int) (string, error) {
	apiClient, err := r.client()
	if err != nil {
		return "", err
	}
	defer func(apiClient *client.Client) { _ = apiClient.Close() }(apiClient)

	reader, tty, err := r.openLogs(ctx, apiClient, id, client.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Tail:       strconv.Itoa(tail),
	})
	if err != nil {
		return "", err
	}
	defer func(reader client.ContainerLogsResult) { _ = reader.Close() }(reader)

	var buf bytes.Buffer
	if err = docker.CopyLogs(&buf, reader, tty); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// FollowLogs 把容器新增的日志持续写入 w，直到日志流结束或 ctx 取消
func (r *containerRepo) FollowLogs(ctx context.Context, id string, w io.Writer) error {
	apiClient, err := r.client()
	if err != nil {
		return err
	}
	defer func(apiClient *client.Client) { _ = apiClient.Close() }(apiClient)

	// Tail "0" 表示只推送新增日志，历史由调用方另行分页加载
	reader, tty, err := r.openLogs(ctx, apiClient, id, client.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
		Tail:       "0",
	})
	if err != nil {
		return err
	}
	defer func(reader client.ContainerLogsResult) { _ = reader.Close() }(reader)

	// 容器停止、连接中断或写入失败都只意味着跟随结束
	_ = docker.CopyLogs(w, reader, tty)
	return nil
}

// openLogs 打开容器日志流，非 TTY 容器的日志是多路复用流，一并返回是否 TTY 供解复用
func (r *containerRepo) openLogs(
	ctx context.Context,
	apiClient *client.Client,
	id string,
	options client.ContainerLogsOptions,
) (client.ContainerLogsResult, bool, error) {
	inspect, err := apiClient.ContainerInspect(ctx, id, client.ContainerInspectOptions{})
	if err != nil {
		return nil, false, err
	}
	tty := inspect.Container.Config != nil && inspect.Container.Config.Tty

	reader, err := apiClient.ContainerLogs(ctx, id, options)
	return reader, tty, err
}

// Prune 清理未使用的容器
func (r *containerRepo) Prune(ctx context.Context) error {
	apiClient, err := r.client()
	if err != nil {
		return err
	}
	defer func(apiClient *client.Client) { _ = apiClient.Close() }(apiClient)

	_, err = apiClient.ContainerPrune(context.WithoutCancel(ctx), client.ContainerPruneOptions{
		Filters: make(client.Filters).Add("label!", "created_by=acepanel"),
	})
	return err
}
