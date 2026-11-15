// Package disk 查询块设备、fstab、LVM、SMART 与 RAID 的状态
package disk

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/acepanel/panel/v3/pkg/shell"
)

// Usage 挂载点的容量，数值是 df 输出的字节数
type Usage struct {
	Size    string `json:"size"`
	Used    string `json:"used"`
	Avail   string `json:"avail"`
	Percent string `json:"percent"`
}

// BlockDevices 返回 lsblk 的设备树
func BlockDevices(ctx context.Context) ([]any, error) {
	output, err := shell.Execf(ctx, "lsblk -J -b -o NAME,SIZE,TYPE,MOUNTPOINT,FSTYPE,UUID,LABEL,MODEL")
	if err != nil {
		return nil, err
	}

	var data struct {
		BlockDevices []any `json:"blockdevices"`
	}
	if err = json.Unmarshal([]byte(output), &data); err != nil {
		return nil, err
	}

	return data.BlockDevices, nil
}

// Usages 按挂载点返回 df 统计的容量
func Usages(ctx context.Context) map[string]Usage {
	output, _ := shell.Execf(ctx, "df -B1 --output=source,size,used,avail,pcent,target 2>/dev/null | tail -n +2")
	return parseDF(output)
}

// PartitionName 返回磁盘上第 n 个分区的设备名
func PartitionName(device string, n int) string {
	// 磁盘名以数字结尾时（nvme0n1、mmcblk0、md0、nbd0、loop0）内核用 p 隔开分区号
	if l := len(device); l > 0 && device[l-1] >= '0' && device[l-1] <= '9' {
		device += "p"
	}

	return device + strconv.Itoa(n)
}

func parseDF(output string) map[string]Usage {
	usages := make(map[string]Usage)
	for line := range strings.SplitSeq(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 6 {
			continue
		}
		usages[fields[5]] = Usage{
			Size:    fields[1],
			Used:    fields[2],
			Avail:   fields[3],
			Percent: strings.TrimSuffix(fields[4], "%"),
		}
	}

	return usages
}
