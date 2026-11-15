package disk

import (
	"context"
	"encoding/json"
	"os/exec"
	"strings"
	"time"

	"github.com/acepanel/panel/v3/pkg/shell"
)

// SMARTDisk 支持 SMART 的磁盘
type SMARTDisk struct {
	Name  string `json:"name"`
	Model string `json:"model"`
	Type  string `json:"type"`
}

// SMARTInstalled 判断 smartctl 是否已安装
func SMARTInstalled() bool {
	_, err := exec.LookPath("smartctl")
	return err == nil
}

// SMARTDisks 列出 smartctl 扫描到的磁盘，扫描失败按没有磁盘处理
func SMARTDisks(ctx context.Context) ([]SMARTDisk, error) {
	scan, err := shell.ExecfWithTimeout(ctx, 10*time.Second, "smartctl --scan -j")
	if err != nil {
		return []SMARTDisk{}, nil //nolint:nilerr
	}
	models, _ := shell.ExecfWithTimeout(ctx, 5*time.Second, "lsblk -J -d -o NAME,MODEL 2>/dev/null")

	return parseSMARTScan(scan, models)
}

// SMARTInfo 返回 smartctl -j -a 输出的磁盘 SMART 详情
func SMARTInfo(ctx context.Context, device string) (any, error) {
	// smartctl 在磁盘有预警时返回非零退出码，但仍有有效 JSON 输出
	output, _ := shell.ExecfWithTimeout(ctx, 30*time.Second, "smartctl -j -a '/dev/%s'", device)

	var info any
	if err := json.Unmarshal([]byte(output), &info); err != nil {
		return nil, err
	}

	return info, nil
}

// parseSMARTScan 解析 smartctl --scan -j 的输出，型号取自 lsblk -J -d -o NAME,MODEL
func parseSMARTScan(scan, models string) ([]SMARTDisk, error) {
	var scanData struct {
		Devices []struct {
			Name string `json:"name"`
			Type string `json:"type"`
		} `json:"devices"`
	}
	if err := json.Unmarshal([]byte(scan), &scanData); err != nil {
		return nil, err
	}

	var lsblkData struct {
		BlockDevices []struct {
			Name  string `json:"name"`
			Model string `json:"model"`
		} `json:"blockdevices"`
	}
	_ = json.Unmarshal([]byte(models), &lsblkData)
	model := make(map[string]string, len(lsblkData.BlockDevices))
	for _, dev := range lsblkData.BlockDevices {
		model[dev.Name] = strings.TrimSpace(dev.Model)
	}

	disks := make([]SMARTDisk, 0, len(scanData.Devices))
	for _, dev := range scanData.Devices {
		name := strings.TrimPrefix(dev.Name, "/dev/")
		disks = append(disks, SMARTDisk{
			Name:  name,
			Model: model[name],
			Type:  dev.Type,
		})
	}

	return disks, nil
}
