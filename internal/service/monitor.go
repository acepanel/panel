package service

import (
	"fmt"
	"net/http"
	"time"

	"github.com/samber/lo"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/pkg/types"
)

type MonitorService struct {
	settingRepo *biz.SettingUsecase
	monitorRepo *biz.MonitorUsecase
}

func NewMonitorService(monitorUsecase *biz.MonitorUsecase, settingUsecase *biz.SettingUsecase) *MonitorService {
	return &MonitorService{
		settingRepo: settingUsecase,
		monitorRepo: monitorUsecase,
	}
}

func (s *MonitorService) GetSetting(w http.ResponseWriter, r *http.Request) {
	setting, err := s.monitorRepo.GetSetting(r.Context())
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, setting)
}

func (s *MonitorService) UpdateSetting(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.MonitorSetting](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.monitorRepo.UpdateSetting(r.Context(), req); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *MonitorService) Clear(w http.ResponseWriter, r *http.Request) {
	if err := s.monitorRepo.Clear(r.Context()); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *MonitorService) List(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.MonitorList](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	monitors, err := s.monitorRepo.List(time.UnixMilli(req.Start), time.UnixMilli(req.End))
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	if len(monitors) == 0 {
		Success(w, types.MonitorDetail{})
		return
	}

	var list types.MonitorDetail

	// 用于存储每个网卡的累计流量
	netDeviceData := make(map[string]*types.Network)
	netDevicePrev := make(map[string]struct {
		sent uint64
		recv uint64
	})

	// 用于存储每个磁盘的IO数据
	diskIOData := make(map[string]*types.DiskIO)
	diskIOPrev := make(map[string]struct {
		read  uint64
		write uint64
	})

	// 初始化第一条数据的网卡和磁盘数据
	for _, net := range monitors[0].Info.Net {
		if net.Name == "lo" {
			continue
		}
		netDevicePrev[net.Name] = struct {
			sent uint64
			recv uint64
		}{sent: net.BytesSent, recv: net.BytesRecv}
		netDeviceData[net.Name] = &types.Network{Name: net.Name}
	}

	for _, disk := range monitors[0].Info.DiskIO {
		diskIOPrev[disk.Name] = struct {
			read  uint64
			write uint64
		}{read: disk.ReadBytes, write: disk.WriteBytes}
		diskIOData[disk.Name] = &types.DiskIO{Name: disk.Name}
	}

	for i, monitor := range monitors {
		// 跳过第一条数据，因为第一条数据的流量为 0
		if i == 0 {
			// MB
			list.Mem.Total = fmt.Sprintf("%.2f", float64(monitor.Info.Mem.Total)/1024/1024)
			list.SWAP.Total = fmt.Sprintf("%.2f", float64(monitor.Info.Swap.Total)/1024/1024)
			continue
		}

		// 计算与上一条记录的时间差（秒），用于速率计算
		elapsed := monitor.CreatedAt.Sub(monitors[i-1].CreatedAt).Seconds()
		if elapsed < 1 {
			elapsed = 1
		}

		// 处理网络数据
		for _, net := range monitor.Info.Net {
			if net.Name == "lo" {
				continue
			}

			// 按网卡分组
			if _, ok := netDeviceData[net.Name]; !ok {
				netDeviceData[net.Name] = &types.Network{Name: net.Name}
				netDevicePrev[net.Name] = struct {
					sent uint64
					recv uint64
				}{sent: 0, recv: 0}
			}
			prev := netDevicePrev[net.Name]
			device := netDeviceData[net.Name]
			device.Sent = append(device.Sent, fmt.Sprintf("%.2f", float64(net.BytesSent)/1024/1024))
			device.Recv = append(device.Recv, fmt.Sprintf("%.2f", float64(net.BytesRecv)/1024/1024))
			device.Tx = append(device.Tx, fmt.Sprintf("%.2f", float64(net.BytesSent-prev.sent)/elapsed/1024/1024))
			device.Rx = append(device.Rx, fmt.Sprintf("%.2f", float64(net.BytesRecv-prev.recv)/elapsed/1024/1024))
			netDevicePrev[net.Name] = struct {
				sent uint64
				recv uint64
			}{sent: net.BytesSent, recv: net.BytesRecv}
		}

		// 处理磁盘IO数据
		for _, disk := range monitor.Info.DiskIO {
			if _, ok := diskIOData[disk.Name]; !ok {
				diskIOData[disk.Name] = &types.DiskIO{Name: disk.Name}
				diskIOPrev[disk.Name] = struct {
					read  uint64
					write uint64
				}{read: 0, write: 0}
			}
			prev := diskIOPrev[disk.Name]
			diskData := diskIOData[disk.Name]
			diskData.ReadBytes = append(diskData.ReadBytes, fmt.Sprintf("%.2f", float64(disk.ReadBytes)/1024/1024))
			diskData.WriteBytes = append(diskData.WriteBytes, fmt.Sprintf("%.2f", float64(disk.WriteBytes)/1024/1024))
			// 根据实际时间差计算每秒速度 (KB/s)
			diskData.ReadSpeed = append(diskData.ReadSpeed, fmt.Sprintf("%.2f", float64(disk.ReadBytes-prev.read)/elapsed/1024))
			diskData.WriteSpeed = append(diskData.WriteSpeed, fmt.Sprintf("%.2f", float64(disk.WriteBytes-prev.write)/elapsed/1024))
			diskIOPrev[disk.Name] = struct {
				read  uint64
				write uint64
			}{read: disk.ReadBytes, write: disk.WriteBytes}
		}

		list.Times = append(list.Times, monitor.CreatedAt.Format(time.DateTime))
		list.Load.Load1 = append(list.Load.Load1, monitor.Info.Load.Load1)
		list.Load.Load5 = append(list.Load.Load5, monitor.Info.Load.Load5)
		list.Load.Load15 = append(list.Load.Load15, monitor.Info.Load.Load15)
		list.CPU.Percent = append(list.CPU.Percent, fmt.Sprintf("%.2f", monitor.Info.Percent))
		list.Mem.Available = append(list.Mem.Available, fmt.Sprintf("%.2f", float64(monitor.Info.Mem.Available)/1024/1024))
		list.Mem.Used = append(list.Mem.Used, fmt.Sprintf("%.2f", float64(monitor.Info.Mem.Used)/1024/1024))
		list.SWAP.Used = append(list.SWAP.Used, fmt.Sprintf("%.2f", float64(monitor.Info.Swap.Used)/1024/1024))
		list.SWAP.Free = append(list.SWAP.Free, fmt.Sprintf("%.2f", float64(monitor.Info.Swap.Free)/1024/1024))

		// Top 5 进程数据
		list.TopProcesses = append(list.TopProcesses, monitor.Info.TopProcesses)
	}

	// 将 map 转换为 slice
	list.Net = lo.Map(lo.Values(netDeviceData), func(device *types.Network, _ int) types.Network {
		return *device
	})
	list.DiskIO = lo.Map(lo.Values(diskIOData), func(disk *types.DiskIO, _ int) types.DiskIO {
		return *disk
	})

	Success(w, list)
}
