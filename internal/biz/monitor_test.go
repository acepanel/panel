package biz

import (
	"math"
	"testing"
	"time"

	"github.com/libtnb/assert/check"
	"github.com/libtnb/assert/must"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/net"

	"github.com/acepanel/panel/v3/pkg/types"
)

const testMB = 1024 * 1024

func monitorSample(at time.Time, nets []net.IOCountersStat, disks []disk.IOCountersStat) *Monitor {
	return &Monitor{
		CreatedAt: at,
		Info: types.CurrentInfo{
			Load:   &load.AvgStat{},
			Mem:    &mem.VirtualMemoryStat{Total: 2048 * testMB},
			Swap:   &mem.SwapMemoryStat{Total: 1024 * testMB},
			Net:    nets,
			DiskIO: disks,
		},
	}
}

func TestMonitorDelta(t *testing.T) {
	s := newMonitorSeries(0)
	check.Equal(t, s.delta(150, 100), uint64(50))
	check.Equal(t, s.delta(100, 100), uint64(0))
	// 重启后计数器归零，直接相减会下溢成天文数字
	check.Equal(t, s.delta(10, math.MaxUint64-5), uint64(0))
	check.Equal(t, s.delta(0, 1), uint64(0))
}

func TestMonitorDetailEmpty(t *testing.T) {
	check.DeepEqual(t, new(MonitorUsecase).buildDetail(nil), types.MonitorDetail{})
}

func TestMonitorDetailRate(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	detail := new(MonitorUsecase).buildDetail([]*Monitor{
		monitorSample(base,
			[]net.IOCountersStat{{Name: "lo", BytesSent: 1, BytesRecv: 1}, {Name: "eth0", BytesSent: 10 * testMB, BytesRecv: 20 * testMB}},
			[]disk.IOCountersStat{{Name: "vda", ReadBytes: 1024, WriteBytes: 2048}}),
		monitorSample(base.Add(10*time.Second),
			[]net.IOCountersStat{{Name: "lo", BytesSent: 2, BytesRecv: 2}, {Name: "eth0", BytesSent: 30 * testMB, BytesRecv: 60 * testMB}},
			[]disk.IOCountersStat{{Name: "vda", ReadBytes: 1024 + 10*1024, WriteBytes: 2048 + 30*1024}}),
		// 重启：计数器从零重新累计
		monitorSample(base.Add(20*time.Second),
			[]net.IOCountersStat{{Name: "eth0", BytesSent: 1 * testMB, BytesRecv: 2 * testMB}},
			[]disk.IOCountersStat{{Name: "vda", ReadBytes: 512, WriteBytes: 512}}),
	})

	check.DeepEqual(t, detail.Times, []string{"2026-01-01 00:00:10", "2026-01-01 00:00:20"})
	check.Equal(t, detail.Mem.Total, "2048.00")
	check.Equal(t, detail.SWAP.Total, "1024.00")

	must.Len(t, detail.Net, 1)
	check.DeepEqual(t, detail.Net[0], types.Network{
		Name: "eth0",
		Sent: []string{"30.00", "1.00"},
		Recv: []string{"60.00", "2.00"},
		Tx:   []string{"2048.00", "0.00"},
		Rx:   []string{"4096.00", "0.00"},
	})

	must.Len(t, detail.DiskIO, 1)
	check.DeepEqual(t, detail.DiskIO[0].ReadSpeed, []string{"1.00", "0.00"})
	check.DeepEqual(t, detail.DiskIO[0].WriteSpeed, []string{"3.00", "0.00"})
}

func TestMonitorDetailAlignsDevices(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	eth0 := func(sent uint64) net.IOCountersStat {
		return net.IOCountersStat{Name: "eth0", BytesSent: sent * testMB}
	}
	veth := func(sent uint64) net.IOCountersStat {
		return net.IOCountersStat{Name: "veth1", BytesSent: sent * testMB}
	}
	detail := new(MonitorUsecase).buildDetail([]*Monitor{
		monitorSample(base, []net.IOCountersStat{eth0(0)}, nil),
		monitorSample(base.Add(1*time.Second), []net.IOCountersStat{eth0(1)}, nil),
		// veth1 中途出现：已有的累计量不能算成这一秒的流量
		monitorSample(base.Add(2*time.Second), []net.IOCountersStat{eth0(2), veth(500)}, nil),
		monitorSample(base.Add(3*time.Second), []net.IOCountersStat{eth0(3), veth(505)}, nil),
		// veth1 消失
		monitorSample(base.Add(4*time.Second), []net.IOCountersStat{eth0(4)}, nil),
		// 同名网卡重新出现，中间缺了一个采样点，不能拿消失前的值当基准
		monitorSample(base.Add(5*time.Second), []net.IOCountersStat{eth0(5), veth(900)}, []disk.IOCountersStat{{Name: "vdb", WriteBytes: 4096}}),
	})

	must.Len(t, detail.Times, 5)
	must.Len(t, detail.Net, 2)
	check.Equal(t, detail.Net[0].Name, "eth0")
	check.DeepEqual(t, detail.Net[0].Tx, []string{"1024.00", "1024.00", "1024.00", "1024.00", "1024.00"})
	check.Equal(t, detail.Net[1].Name, "veth1")
	check.DeepEqual(t, detail.Net[1].Sent, []string{"0.00", "500.00", "505.00", "0.00", "900.00"})
	check.DeepEqual(t, detail.Net[1].Tx, []string{"0.00", "0.00", "5120.00", "0.00", "0.00"})
	check.Len(t, detail.Net[1].Recv, 5)
	check.Len(t, detail.Net[1].Rx, 5)

	must.Len(t, detail.DiskIO, 1)
	check.DeepEqual(t, detail.DiskIO[0].WriteBytes, []string{"0.00", "0.00", "0.00", "0.00", "0.00"})
	check.DeepEqual(t, detail.DiskIO[0].WriteSpeed, []string{"0.00", "0.00", "0.00", "0.00", "0.00"})
}
