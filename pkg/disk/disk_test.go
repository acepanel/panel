package disk

import (
	"testing"

	"github.com/libtnb/assert/check"
)

func TestPartitionName(t *testing.T) {
	tests := []struct {
		device string
		want   string
	}{
		{device: "/dev/sdb", want: "/dev/sdb1"},
		{device: "/dev/vda", want: "/dev/vda1"},
		{device: "/dev/xvdf", want: "/dev/xvdf1"},
		{device: "/dev/nvme0n1", want: "/dev/nvme0n1p1"},
		{device: "/dev/mmcblk0", want: "/dev/mmcblk0p1"},
		{device: "/dev/md0", want: "/dev/md0p1"},
		{device: "/dev/nbd0", want: "/dev/nbd0p1"},
		{device: "/dev/loop0", want: "/dev/loop0p1"},
	}
	for _, tt := range tests {
		t.Run(tt.device, func(t *testing.T) {
			check.Equal(t, PartitionName(tt.device, 1), tt.want)
		})
	}
}

func TestParseDF(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   map[string]Usage
	}{
		{
			name:   "没有输出",
			output: "",
			want:   map[string]Usage{},
		},
		{
			name: "按挂载点汇总并去掉百分号",
			output: `/dev/vda2       42006183936 9214304256 30626725888  24% /
tmpfs            2063134720          0  2063134720   0% /dev/shm
/dev/vda1         535805952    6451200   529354752   2% /boot/efi`,
			want: map[string]Usage{
				"/":         {Size: "42006183936", Used: "9214304256", Avail: "30626725888", Percent: "24"},
				"/dev/shm":  {Size: "2063134720", Used: "0", Avail: "2063134720", Percent: "0"},
				"/boot/efi": {Size: "535805952", Used: "6451200", Avail: "529354752", Percent: "2"},
			},
		},
		{
			name:   "跳过字段不全的行",
			output: "/dev/vda2 42006183936 9214304256\n",
			want:   map[string]Usage{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			check.DeepEqual(t, parseDF(tt.output), tt.want)
		})
	}
}
