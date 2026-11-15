package disk

import (
	"testing"

	"github.com/libtnb/assert/check"
)

func TestParseFstab(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    []FstabEntry
	}{
		{
			name:    "只有注释和空行",
			content: "# /etc/fstab: static file system information.\n\n   # indented comment\n",
		},
		{
			name: "完整条目",
			content: `# <file system> <mount point> <type> <options> <dump> <pass>
UUID=0a1b2c3d-4e5f-6071-8293-a4b5c6d7e8f9 /         ext4 errors=remount-ro 0 1
/dev/sdb1	/data	xfs	defaults,noatime	0	2
`,
			want: []FstabEntry{
				{Device: "UUID=0a1b2c3d-4e5f-6071-8293-a4b5c6d7e8f9", MountPoint: "/", FsType: "ext4", Options: "errors=remount-ro", Dump: "0", Pass: "1"},
				{Device: "/dev/sdb1", MountPoint: "/data", FsType: "xfs", Options: "defaults,noatime", Dump: "0", Pass: "2"},
			},
		},
		{
			name:    "省略 dump 和 pass",
			content: "/swapfile none swap sw\ntmpfs /tmp tmpfs defaults 0\n",
			want: []FstabEntry{
				{Device: "/swapfile", MountPoint: "none", FsType: "swap", Options: "sw"},
				{Device: "tmpfs", MountPoint: "/tmp", FsType: "tmpfs", Options: "defaults", Dump: "0"},
			},
		},
		{
			name:    "跳过字段不足四个的行",
			content: "/dev/sdc1 /backup ext4\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			check.DeepEqual(t, parseFstab(tt.content), tt.want)
		})
	}
}
