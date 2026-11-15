package disk

import (
	"os"
	"slices"
	"strings"
)

// FstabEntry fstab 条目
type FstabEntry struct {
	Device     string `json:"device"`      // 设备（UUID=xxx 或 /dev/xxx）
	MountPoint string `json:"mount_point"` // 挂载点
	FsType     string `json:"fs_type"`     // 文件系统类型
	Options    string `json:"options"`     // 挂载选项
	Dump       string `json:"dump"`        // 备份标志
	Pass       string `json:"pass"`        // 检查顺序
}

// Fstab 读取 fstab 中的有效条目
func Fstab() ([]FstabEntry, error) {
	content, err := os.ReadFile("/etc/fstab")
	if err != nil {
		return nil, err
	}

	return parseFstab(string(content)), nil
}

// FstabHas 判断挂载点是否已写入 fstab
func FstabHas(mountPoint string) bool {
	entries, _ := Fstab()
	return slices.ContainsFunc(entries, func(entry FstabEntry) bool {
		return entry.MountPoint == mountPoint
	})
}

func parseFstab(content string) []FstabEntry {
	var entries []FstabEntry
	for line := range strings.SplitSeq(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		entry := FstabEntry{
			Device:     fields[0],
			MountPoint: fields[1],
			FsType:     fields[2],
			Options:    fields[3],
		}
		if len(fields) >= 5 {
			entry.Dump = fields[4]
		}
		if len(fields) >= 6 {
			entry.Pass = fields[5]
		}
		entries = append(entries, entry)
	}

	return entries
}
