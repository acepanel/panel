package biz

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"time"

	"github.com/leonelquinteros/gotext"
	"github.com/spf13/cast"

	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/pkg/types"
)

type Monitor struct {
	ID        uint              `gorm:"primaryKey" json:"id"`
	Info      types.CurrentInfo `gorm:"not null;default:'{}';serializer:zstd" json:"info"`
	CreatedAt time.Time         `gorm:"index:idx_monitors_created_at" json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}

type MonitorRepo interface {
	Create(monitor *Monitor) error
	ClearBefore(t time.Time) error
	Clear() error
	List(start, end time.Time) ([]*Monitor, error)
	VacuumDB() error
}

type MonitorUsecase struct {
	t       *gotext.Locale
	log     *slog.Logger
	repo    MonitorRepo
	setting SettingRepo
}

func NewMonitorUsecase(t *gotext.Locale, log *slog.Logger, repo MonitorRepo, setting SettingRepo) *MonitorUsecase {
	return &MonitorUsecase{t: t, log: log, repo: repo, setting: setting}
}

func (uc *MonitorUsecase) GetSetting(ctx context.Context) (*request.MonitorSetting, error) {
	monitor, err := uc.setting.Get(SettingKeyMonitor)
	if err != nil {
		return nil, err
	}
	monitorDays, err := uc.setting.Get(SettingKeyMonitorDays)
	if err != nil {
		return nil, err
	}
	monitorInterval, err := uc.setting.GetInt(SettingKeyMonitorInterval, 1)
	if err != nil {
		return nil, err
	}
	alertDays, err := uc.setting.GetInt(SettingKeyAlertLogDays, 30)
	if err != nil {
		return nil, err
	}

	setting := new(request.MonitorSetting)
	setting.Enabled = cast.ToBool(monitor)
	setting.Days = cast.ToUint(monitorDays)
	setting.Interval = uint(monitorInterval)
	setting.AlertDays = uint(alertDays)

	uc.log.Info(uc.t.Get("monitor setting viewed"), slog.String("type", OperationTypeMonitor), slog.Uint64("operator_id", OperatorID(ctx)))

	return setting, nil
}

func (uc *MonitorUsecase) UpdateSetting(ctx context.Context, setting *request.MonitorSetting) error {
	if err := uc.setting.Set(SettingKeyMonitor, cast.ToString(setting.Enabled)); err != nil {
		return err
	}
	if err := uc.setting.Set(SettingKeyMonitorDays, cast.ToString(setting.Days)); err != nil {
		return err
	}
	if err := uc.setting.Set(SettingKeyMonitorInterval, cast.ToString(setting.Interval)); err != nil {
		return err
	}

	if err := uc.setting.Set(SettingKeyAlertLogDays, cast.ToString(setting.AlertDays)); err != nil {
		return err
	}

	uc.log.Info(uc.t.Get("monitor setting updated"), slog.String("type", OperationTypeMonitor), slog.Uint64("operator_id", OperatorID(ctx)), slog.Bool("enabled", setting.Enabled), slog.Uint64("days", uint64(setting.Days)), slog.Uint64("interval", uint64(setting.Interval)), slog.Uint64("alert_days", uint64(setting.AlertDays)))

	return nil
}

func (uc *MonitorUsecase) Create(monitor *Monitor) error {
	return uc.repo.Create(monitor)
}

func (uc *MonitorUsecase) ClearBefore(t time.Time) error {
	return uc.repo.ClearBefore(t)
}

func (uc *MonitorUsecase) Clear(ctx context.Context) error {
	if err := uc.repo.Clear(); err != nil {
		return err
	}

	uc.log.Warn(uc.t.Get("monitor data cleared"), slog.String("type", OperationTypeMonitor), slog.Uint64("operator_id", OperatorID(ctx)))

	return nil
}

func (uc *MonitorUsecase) Detail(start, end time.Time) (types.MonitorDetail, error) {
	monitors, err := uc.repo.List(start, end)
	if err != nil {
		return types.MonitorDetail{}, err
	}

	return uc.buildDetail(monitors), nil
}

func (uc *MonitorUsecase) VacuumDB() error {
	return uc.repo.VacuumDB()
}

// buildDetail 第一条采样只作增量基准，不进时间轴
func (uc *MonitorUsecase) buildDetail(monitors []*Monitor) types.MonitorDetail {
	var detail types.MonitorDetail
	if len(monitors) == 0 {
		return detail
	}

	nets := newMonitorSeries(len(monitors) - 1)
	disks := newMonitorSeries(len(monitors) - 1)
	record := func(idx int, info types.CurrentInfo, elapsed float64) {
		for _, item := range info.Net {
			if item.Name != "lo" {
				nets.record(idx, item.Name, monitorCounter{in: item.BytesRecv, out: item.BytesSent}, elapsed)
			}
		}
		for _, item := range info.DiskIO {
			disks.record(idx, item.Name, monitorCounter{in: item.ReadBytes, out: item.WriteBytes}, elapsed)
		}
	}

	prev := monitors[0]
	record(-1, prev.Info, 1)
	detail.Mem.Total = monitorMB(prev.Info.Mem.Total)
	detail.SWAP.Total = monitorMB(prev.Info.Swap.Total)

	for i, monitor := range monitors[1:] {
		info := monitor.Info
		record(i, info, max(monitor.CreatedAt.Sub(prev.CreatedAt).Seconds(), 1))
		prev = monitor

		detail.Times = append(detail.Times, monitor.CreatedAt.Format(time.DateTime))
		detail.Load.Load1 = append(detail.Load.Load1, info.Load.Load1)
		detail.Load.Load5 = append(detail.Load.Load5, info.Load.Load5)
		detail.Load.Load15 = append(detail.Load.Load15, info.Load.Load15)
		detail.CPU.Percent = append(detail.CPU.Percent, fmt.Sprintf("%.2f", info.Percent))
		detail.Mem.Available = append(detail.Mem.Available, monitorMB(info.Mem.Available))
		detail.Mem.Used = append(detail.Mem.Used, monitorMB(info.Mem.Used))
		detail.SWAP.Used = append(detail.SWAP.Used, monitorMB(info.Swap.Used))
		detail.SWAP.Free = append(detail.SWAP.Free, monitorMB(info.Swap.Free))
		detail.TopProcesses = append(detail.TopProcesses, info.TopProcesses)
	}

	detail.Net = make([]types.Network, 0, len(nets.devices))
	for _, name := range slices.Sorted(maps.Keys(nets.devices)) {
		device := nets.devices[name]
		detail.Net = append(detail.Net, types.Network{Name: name, Sent: device.out, Recv: device.in, Tx: device.outRate, Rx: device.inRate})
	}
	detail.DiskIO = make([]types.DiskIO, 0, len(disks.devices))
	for _, name := range slices.Sorted(maps.Keys(disks.devices)) {
		device := disks.devices[name]
		detail.DiskIO = append(detail.DiskIO, types.DiskIO{Name: name, ReadBytes: device.in, WriteBytes: device.out, ReadSpeed: device.inRate, WriteSpeed: device.outRate})
	}

	return detail
}

// monitorCounter 是设备的一对累计计数器，网卡为接收与发送，磁盘为读与写
type monitorCounter struct {
	in, out uint64
}

// monitorDevice 是单个设备与时间轴等长的序列，设备缺席的时间点保持零值
type monitorDevice struct {
	last                     int // 最近一次出现的采样下标
	prev                     monitorCounter
	in, out, inRate, outRate []string
}

type monitorSeries struct {
	points  int
	devices map[string]*monitorDevice
}

func newMonitorSeries(points int) *monitorSeries {
	return &monitorSeries{points: points, devices: make(map[string]*monitorDevice)}
}

// record 记下设备在第 idx 个时间点的数据，idx 为 -1 的首条采样只作增量基准
func (s *monitorSeries) record(idx int, name string, cur monitorCounter, elapsed float64) {
	device, ok := s.devices[name]
	if !ok {
		device = &monitorDevice{
			in:      s.zeros(s.points),
			out:     s.zeros(s.points),
			inRate:  s.zeros(s.points),
			outRate: s.zeros(s.points),
		}
		s.devices[name] = device
	}
	// 上一个采样点没有该设备就没有可比的基准，速率留零
	continuous := ok && device.last == idx-1
	prev := device.prev
	device.last, device.prev = idx, cur
	if idx < 0 {
		return
	}

	device.in[idx] = monitorMB(cur.in)
	device.out[idx] = monitorMB(cur.out)
	if continuous {
		device.inRate[idx] = s.kbps(s.delta(cur.in, prev.in), elapsed)
		device.outRate[idx] = s.kbps(s.delta(cur.out, prev.out), elapsed)
	}
}

// delta 计数器归零或回绕（重启、设备重建）时视为没有增量
func (s *monitorSeries) delta(cur, prev uint64) uint64 {
	if cur < prev {
		return 0
	}
	return cur - prev
}

func (s *monitorSeries) zeros(n int) []string {
	return slices.Repeat([]string{"0.00"}, n)
}

func (s *monitorSeries) kbps(bytes uint64, elapsed float64) string {
	return fmt.Sprintf("%.2f", float64(bytes)/elapsed/1024)
}

func monitorMB(bytes uint64) string {
	return fmt.Sprintf("%.2f", float64(bytes)/1024/1024)
}
