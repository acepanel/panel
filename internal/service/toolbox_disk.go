package service

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/chix/v2"
	"github.com/samber/do/v2"

	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/pkg/shell"
)

type ToolboxDiskService struct {
	t *gotext.Locale
}

func NewToolboxDiskService(i do.Injector) (*ToolboxDiskService, error) {
	return &ToolboxDiskService{
		t: do.MustInvoke[*gotext.Locale](i),
	}, nil
}

// List 获取磁盘列表
func (s *ToolboxDiskService) List(w http.ResponseWriter, r *http.Request) {
	// 获取磁盘基本信息
	lsblkOutput, err := shell.Execf("lsblk -J -b -o NAME,SIZE,TYPE,MOUNTPOINT,FSTYPE,UUID,LABEL,MODEL")
	if err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("failed to get disk list: %v", err))
		return
	}

	// 解析 lsblk JSON
	var lsblkData struct {
		BlockDevices []any `json:"blockdevices"`
	}
	if err = json.Unmarshal([]byte(lsblkOutput), &lsblkData); err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("failed to parse disk list: %v", err))
		return
	}

	// 获取磁盘使用情况
	dfOutput, _ := shell.Execf("df -B1 --output=source,size,used,avail,pcent,target 2>/dev/null | tail -n +2")

	// 解析 df 输出为 map
	dfMap := make(map[string]map[string]string)
	lines := strings.SplitSeq(strings.TrimSpace(dfOutput), "\n")
	for line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 6 {
			mountpoint := fields[5]
			dfMap[mountpoint] = map[string]string{
				"size":    fields[1],
				"used":    fields[2],
				"avail":   fields[3],
				"percent": strings.TrimSuffix(fields[4], "%"),
			}
		}
	}

	Success(w, chix.M{
		"disks": lsblkData.BlockDevices,
		"df":    dfMap,
	})
}

// GetPartitions 获取分区列表
func (s *ToolboxDiskService) GetPartitions(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ToolboxDiskDevice](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	output, err := shell.Execf("lsblk -J -b -o NAME,SIZE,TYPE,MOUNTPOINT,FSTYPE,UUID,LABEL '/dev/%s'", req.Device)
	if err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("failed to get partitions: %v", err))
		return
	}

	Success(w, output)
}

// Mount 挂载分区
func (s *ToolboxDiskService) Mount(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ToolboxDiskMount](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if _, err = shell.Execf("test -d '%s' || mkdir -p '%s'", req.Path, req.Path); err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("failed to create mount point: %v", err))
		return
	}

	if _, err = shell.Execf("mount '/dev/%s' '%s'", req.Device, req.Path); err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("failed to mount partition: %v", err))
		return
	}

	// 如果需要写入 fstab
	if req.WriteFstab {
		// 获取分区的 UUID
		uuid, err := shell.Execf("blkid -s UUID -o value '/dev/%s'", req.Device)
		if err != nil {
			Error(w, http.StatusInternalServerError, s.t.Get("failed to get partition UUID: %v", err))
			return
		}
		uuid = strings.TrimSpace(uuid)
		if uuid == "" {
			Error(w, http.StatusInternalServerError, s.t.Get("partition has no UUID"))
			return
		}

		// 获取文件系统类型
		fsType, err := shell.Execf("blkid -s TYPE -o value '/dev/%s'", req.Device)
		if err != nil {
			Error(w, http.StatusInternalServerError, s.t.Get("failed to get filesystem type: %v", err))
			return
		}
		fsType = strings.TrimSpace(fsType)
		if fsType == "" {
			fsType = "auto"
		}

		// 挂载选项
		mountOption := req.MountOption
		if mountOption == "" {
			mountOption = "defaults"
		}

		// 检查 fstab 中是否已存在该挂载点
		existCheck, _ := shell.Execf("grep -E '^[^#].*\\s+%s\\s+' /etc/fstab", req.Path)
		if strings.TrimSpace(existCheck) != "" {
			Error(w, http.StatusBadRequest, s.t.Get("mount point %s already exists in fstab", req.Path))
			return
		}

		// 写入 fstab
		fstabEntry := fmt.Sprintf("UUID=%s %s %s %s 0 2", uuid, req.Path, fsType, mountOption)
		if _, err = shell.Execf("echo '%s' >> /etc/fstab", fstabEntry); err != nil {
			Error(w, http.StatusInternalServerError, s.t.Get("failed to write fstab: %v", err))
			return
		}
	}

	Success(w, nil)
}

// Umount 卸载分区
func (s *ToolboxDiskService) Umount(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ToolboxDiskUmount](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if _, err = shell.Execf("umount '%s'", req.Path); err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("failed to umount partition: %v", err))
		return
	}

	Success(w, nil)
}

// Format 格式化分区
func (s *ToolboxDiskService) Format(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ToolboxDiskFormat](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	var formatCmd string
	switch req.FsType {
	case "ext4":
		formatCmd = fmt.Sprintf("mkfs.ext4 -F '/dev/%s'", req.Device)
	case "ext3":
		formatCmd = fmt.Sprintf("mkfs.ext3 -F '/dev/%s'", req.Device)
	case "xfs":
		formatCmd = fmt.Sprintf("mkfs.xfs -f '/dev/%s'", req.Device)
	case "btrfs":
		formatCmd = fmt.Sprintf("mkfs.btrfs -f '/dev/%s'", req.Device)
	default:
		Error(w, http.StatusUnprocessableEntity, s.t.Get("unsupported filesystem type: %s", req.FsType))
		return
	}

	if _, err = shell.Execf(formatCmd); err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("failed to format partition: %v", err))
		return
	}

	Success(w, nil)
}

// Init 初始化磁盘（删除所有分区，创建单个分区并格式化）
func (s *ToolboxDiskService) Init(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ToolboxDiskInit](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	device := "/dev/" + req.Device

	// 检查设备是否存在
	if _, err = shell.Execf("test -b '%s'", device); err != nil {
		Error(w, http.StatusBadRequest, s.t.Get("device not found: %s", device))
		return
	}

	// 检查是否为系统盘（检查是否有分区挂载在 /）
	mountInfo, _ := shell.Execf("lsblk -no MOUNTPOINT '%s' 2>/dev/null", device)
	if strings.Contains(mountInfo, "/\n") || strings.TrimSpace(mountInfo) == "/" {
		Error(w, http.StatusBadRequest, s.t.Get("cannot initialize system disk"))
		return
	}

	// 卸载该磁盘的所有分区
	_, _ = shell.Execf("umount '%s'* 2>/dev/null || true", device)

	// 先清除分区表
	if _, err = shell.Execf("wipefs -a '%s'", device); err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("failed to wipe disk: %v", err))
		return
	}

	// sfdisk 创建 GPT 分区表和单个分区
	if _, err = shell.Execf("echo 'type=linux' | sfdisk '%s'", device); err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("failed to create partition: %v", err))
		return
	}

	// 等待内核更新分区表
	_, _ = shell.Execf("partprobe '%s' 2>/dev/null || true", device)
	_, _ = shell.Execf("sleep 1")

	// 确定新分区的设备名（device + "1"，如 sdb1 或 nvme0n1p1）
	var partDevice string
	if strings.Contains(req.Device, "nvme") || strings.Contains(req.Device, "loop") {
		partDevice = device + "p1"
	} else {
		partDevice = device + "1"
	}

	// 格式化新分区
	var formatCmd string
	switch req.FsType {
	case "ext4":
		formatCmd = fmt.Sprintf("mkfs.ext4 -F '%s'", partDevice)
	case "ext3":
		formatCmd = fmt.Sprintf("mkfs.ext3 -F '%s'", partDevice)
	case "xfs":
		formatCmd = fmt.Sprintf("mkfs.xfs -f '%s'", partDevice)
	case "btrfs":
		formatCmd = fmt.Sprintf("mkfs.btrfs -f '%s'", partDevice)
	default:
		Error(w, http.StatusUnprocessableEntity, s.t.Get("unsupported filesystem type: %s", req.FsType))
		return
	}

	if _, err = shell.Execf(formatCmd); err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("failed to format partition: %v", err))
		return
	}

	Success(w, nil)
}

// GetLVMInfo 获取LVM信息
func (s *ToolboxDiskService) GetLVMInfo(w http.ResponseWriter, r *http.Request) {
	// 获取物理卷信息
	pvOutput, _ := shell.Execf("pvdisplay -C --noheadings --separator '|' -o pv_name,vg_name,pv_size,pv_free 2>/dev/null || echo ''")
	// 获取卷组信息
	vgOutput, _ := shell.Execf("vgdisplay -C --noheadings --separator '|' -o vg_name,pv_count,lv_count,vg_size,vg_free 2>/dev/null || echo ''")
	// 获取逻辑卷信息
	lvOutput, _ := shell.Execf("lvdisplay -C --noheadings --separator '|' -o lv_name,vg_name,lv_size,lv_path 2>/dev/null || echo ''")

	pvs := s.parseLVMOutput(pvOutput)
	vgs := s.parseLVMOutput(vgOutput)
	lvs := s.parseLVMOutput(lvOutput)

	Success(w, chix.M{
		"pvs": pvs,
		"vgs": vgs,
		"lvs": lvs,
	})
}

// CreatePV 创建物理卷
func (s *ToolboxDiskService) CreatePV(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ToolboxDiskDevice](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if _, err = shell.Execf("pvcreate '/dev/%s'", req.Device); err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("failed to create physical volume: %v", err))
		return
	}

	Success(w, nil)
}

// CreateVG 创建卷组
func (s *ToolboxDiskService) CreateVG(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ToolboxDiskVG](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	// 构建设备列表，每个设备单独引用
	var deviceArgs []string
	for _, dev := range req.Devices {
		deviceArgs = append(deviceArgs, fmt.Sprintf("'%s'", dev))
	}

	if _, err = shell.Execf("vgcreate '%s' %s", req.Name, strings.Join(deviceArgs, " ")); err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("failed to create volume group: %v", err))
		return
	}

	Success(w, nil)
}

// CreateLV 创建逻辑卷
func (s *ToolboxDiskService) CreateLV(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ToolboxDiskLV](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	// 验证逻辑卷大小（必须为正数）
	if req.Size <= 0 {
		Error(w, http.StatusUnprocessableEntity, s.t.Get("invalid logical volume size"))
		return
	}

	// 创建逻辑卷
	if _, err = shell.Execf("lvcreate -L '%dG' -n '%s' '%s'", req.Size, req.Name, req.VGName); err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("failed to create logical volume: %v", err))
		return
	}

	Success(w, nil)
}

// RemovePV 删除物理卷
func (s *ToolboxDiskService) RemovePV(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ToolboxDiskDevice](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if _, err = shell.Execf("pvremove '%s'", req.Device); err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("failed to remove physical volume: %v", err))
		return
	}

	Success(w, nil)
}

// RemoveVG 删除卷组
func (s *ToolboxDiskService) RemoveVG(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ToolboxDiskVGName](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if _, err = shell.Execf("vgremove -f '%s'", req.Name); err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("failed to remove volume group: %v", err))
		return
	}

	Success(w, nil)
}

// RemoveLV 删除逻辑卷
func (s *ToolboxDiskService) RemoveLV(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ToolboxDiskLVPath](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if _, err = shell.Execf("lvremove -f '%s'", req.Path); err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("failed to remove logical volume: %v", err))
		return
	}

	Success(w, nil)
}

// ExtendLV 扩容逻辑卷
func (s *ToolboxDiskService) ExtendLV(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ToolboxDiskExtendLV](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	// 验证扩容大小为正整数
	if req.Size <= 0 {
		Error(w, http.StatusUnprocessableEntity, s.t.Get("invalid size"))
		return
	}

	// 扩容逻辑卷
	if _, err = shell.Execf("lvextend -L +%dG '%s'", req.Size, req.Path); err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("failed to extend logical volume: %v", err))
		return
	}

	// 扩展文件系统
	if req.Resize {
		// 检测文件系统类型并扩展
		fsType, _ := shell.Execf("blkid -o value -s TYPE '%s'", req.Path)
		fsType = strings.TrimSpace(fsType)

		switch fsType {
		case "ext4", "ext3":
			if _, err = shell.Execf("resize2fs '%s'", req.Path); err != nil {
				Error(w, http.StatusInternalServerError, s.t.Get("failed to resize filesystem: %v", err))
				return
			}
		case "xfs":
			// XFS需要挂载后才能扩展
			mountPoint, _ := shell.Execf("findmnt -n -o TARGET '%s'", req.Path)
			mountPoint = strings.TrimSpace(mountPoint)
			if mountPoint != "" {
				if _, err = shell.Execf("xfs_growfs '%s'", mountPoint); err != nil {
					Error(w, http.StatusInternalServerError, s.t.Get("failed to resize filesystem: %v", err))
					return
				}
			} else {
				// XFS未挂载时，返回错误信息
				Error(w, http.StatusInternalServerError, s.t.Get("xfs filesystem is not mounted, logical volume has been extended but filesystem was not resized"))
				return
			}
		case "btrfs":
			// btrfs需要挂载后才能扩展
			mountPoint, _ := shell.Execf("findmnt -n -o TARGET '%s'", req.Path)
			mountPoint = strings.TrimSpace(mountPoint)
			if mountPoint != "" {
				// 扩展到当前可用的最大空间
				if _, err = shell.Execf("btrfs filesystem resize max '%s'", mountPoint); err != nil {
					Error(w, http.StatusInternalServerError, s.t.Get("failed to resize filesystem: %v", err))
					return
				}
			} else {
				// btrfs未挂载时，返回错误信息
				Error(w, http.StatusInternalServerError, s.t.Get("btrfs filesystem is not mounted, logical volume has been extended but filesystem was not resized"))
				return
			}
		}
	}

	Success(w, nil)
}

// parseLVMOutput 解析LVM命令输出
// 将LVM命令的表格输出解析为map数组，每行数据的字段以field_0, field_1...命名
var spaceRegex = regexp.MustCompile(`\s+`)

func (s *ToolboxDiskService) parseLVMOutput(output string) []map[string]string {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	var result []map[string]string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		line = spaceRegex.ReplaceAllString(line, " ")

		fields := strings.Split(line, "|")
		item := make(map[string]string)

		for i, field := range fields {
			item[fmt.Sprintf("field_%d", i)] = strings.TrimSpace(field)
		}

		result = append(result, item)
	}

	return result
}

// GetFstab 获取 fstab 列表
func (s *ToolboxDiskService) GetFstab(w http.ResponseWriter, r *http.Request) {
	content, err := shell.Execf("cat /etc/fstab")
	if err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("failed to read fstab: %v", err))
		return
	}

	var entries []request.ToolboxDiskFstabEntry
	lines := strings.SplitSeq(content, "\n")
	for line := range lines {
		line = strings.TrimSpace(line)
		// 跳过空行和注释
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) >= 4 {
			entry := request.ToolboxDiskFstabEntry{
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
	}

	Success(w, entries)
}

// DeleteFstab 删除 fstab 条目
func (s *ToolboxDiskService) DeleteFstab(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ToolboxDiskFstabDelete](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	// 不允许删除根目录挂载
	if req.MountPoint == "/" {
		Error(w, http.StatusBadRequest, s.t.Get("cannot delete root mount point"))
		return
	}

	if _, err = shell.Execf(`sed -i 's@^[^#].*\s%s\s.*$@@g' /etc/fstab`, req.MountPoint); err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("failed to delete fstab entry: %v", err))
		return
	}

	if _, err = shell.Execf("mount -a"); err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("failed to remount filesystems: %v", err))
		return
	}

	Success(w, nil)
}

// GetSmartDisks 获取支持 SMART 的磁盘列表
func (s *ToolboxDiskService) GetSmartDisks(w http.ResponseWriter, r *http.Request) {
	// 检查 smartctl 是否安装
	if _, err := shell.ExecfWithTimeout(5*time.Second, "which smartctl"); err != nil {
		Success(w, chix.M{
			"available": false,
			"message":   s.t.Get("smartmontools is not installed, please install it first (e.g., apt install smartmontools or dnf install smartmontools)"),
			"disks":     []any{},
		})
		return
	}

	// 获取磁盘列表
	scanOutput, err := shell.ExecfWithTimeout(10*time.Second, "smartctl --scan -j")
	if err != nil {
		Success(w, chix.M{
			"available": true,
			"message":   "",
			"disks":     []any{},
		})
		return
	}

	var scanData struct {
		Devices []struct {
			Name     string `json:"name"`
			InfoName string `json:"info_name"`
			Type     string `json:"type"`
			Protocol string `json:"protocol"`
		} `json:"devices"`
	}
	if err = json.Unmarshal([]byte(scanOutput), &scanData); err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("failed to parse smartctl output: %v", err))
		return
	}

	type smartDisk struct {
		Name  string `json:"name"`
		Model string `json:"model"`
		Type  string `json:"type"`
	}

	disks := make([]smartDisk, 0)
	for _, dev := range scanData.Devices {
		// 获取设备名（去掉 /dev/ 前缀）
		name := strings.TrimPrefix(dev.Name, "/dev/")
		// 获取 model 信息
		model, _ := shell.ExecfWithTimeout(5*time.Second, "lsblk -ndo MODEL '/dev/%s' 2>/dev/null", name)
		disks = append(disks, smartDisk{
			Name:  name,
			Model: strings.TrimSpace(model),
			Type:  dev.Type,
		})
	}

	Success(w, chix.M{
		"available": true,
		"message":   "",
		"disks":     disks,
	})
}

// GetSmartInfo 获取指定磁盘的 SMART 详细信息
func (s *ToolboxDiskService) GetSmartInfo(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ToolboxDiskDevice](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	// smartctl 在磁盘有预警时返回非零退出码，但仍有有效 JSON 输出
	output, _ := shell.ExecfWithTimeout(30*time.Second, "smartctl -j -a '/dev/%s'", req.Device)
	if output == "" {
		Error(w, http.StatusInternalServerError, s.t.Get("failed to get SMART info for device %s", req.Device))
		return
	}

	// 解析为结构化数据
	var result any
	if err = json.Unmarshal([]byte(output), &result); err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("failed to parse SMART info: %v", err))
		return
	}

	Success(w, result)
}

// GetRaidInfo 获取 RAID 阵列状态
func (s *ToolboxDiskService) GetRaidInfo(w http.ResponseWriter, r *http.Request) {
	// 按优先级检测 RAID 类型
	// 1. 软件 RAID (mdadm)
	if info := s.detectMdadm(); info != nil {
		Success(w, info)
		return
	}
	// 2. MegaRAID (LSI/Broadcom)
	if info := s.detectMegaRAID(); info != nil {
		Success(w, info)
		return
	}
	// 3. HP Smart Array
	if info := s.detectHPSA(); info != nil {
		Success(w, info)
		return
	}
	// 4. Adaptec
	if info := s.detectAdaptec(); info != nil {
		Success(w, info)
		return
	}

	// 未检测到任何 RAID
	Success(w, chix.M{
		"available":   false,
		"message":     s.t.Get("no RAID configuration detected"),
		"type":        "",
		"controllers": []any{},
		"arrays":      []any{},
	})
}

// raidArray RAID 阵列信息
type raidArray struct {
	Name          string       `json:"name"`
	RaidLevel     string       `json:"raid_level"`
	Size          string       `json:"size"`
	State         string       `json:"state"`
	StripSize     string       `json:"strip_size"`
	ActiveDevices int          `json:"active_devices"`
	TotalDevices  int          `json:"total_devices"`
	RebuildPct    string       `json:"rebuild_pct,omitempty"`
	Devices       []raidDevice `json:"devices"`
}

// raidDevice RAID 物理磁盘信息
type raidDevice struct {
	Name   string `json:"name"`
	Slot   string `json:"slot"`
	Size   string `json:"size"`
	State  string `json:"state"`
	Model  string `json:"model"`
	Serial string `json:"serial"`
}

// raidController RAID 控制器信息
type raidController struct {
	Model    string `json:"model"`
	Serial   string `json:"serial"`
	Firmware string `json:"firmware"`
	Cache    string `json:"cache_size"`
}

// detectMdadm 检测软件 RAID (mdadm)
func (s *ToolboxDiskService) detectMdadm() chix.M {
	mdstat, err := shell.ExecfWithTimeout(5*time.Second, "cat /proc/mdstat 2>/dev/null")
	if err != nil || !strings.Contains(mdstat, " : ") {
		return nil
	}

	// 获取 md 设备列表
	var mdDevices []string
	for line := range strings.SplitSeq(mdstat, "\n") {
		line = strings.TrimSpace(line)
		if strings.Contains(line, " : ") {
			parts := strings.SplitN(line, " ", 2)
			if len(parts) > 0 {
				mdDevices = append(mdDevices, parts[0])
			}
		}
	}

	if len(mdDevices) == 0 {
		return nil
	}

	var arrays []raidArray
	for _, md := range mdDevices {
		detail, _ := shell.ExecfWithTimeout(10*time.Second, "mdadm --detail '/dev/%s' 2>/dev/null", md)
		if detail == "" {
			continue
		}
		arrays = append(arrays, s.parseMdadm(md, detail))
	}

	return chix.M{
		"available":   true,
		"message":     "",
		"type":        "mdadm",
		"controllers": []any{},
		"arrays":      arrays,
	}
}

// parseMdadm 解析 mdadm --detail 输出
func (s *ToolboxDiskService) parseMdadm(name, detail string) raidArray {
	arr := raidArray{Name: name}

	for line := range strings.SplitSeq(detail, "\n") {
		line = strings.TrimSpace(line)
		if after, ok := strings.CutPrefix(line, "Raid Level :"); ok {
			arr.RaidLevel = strings.TrimSpace(after)
		} else if after, ok := strings.CutPrefix(line, "Array Size :"); ok {
			arr.Size = strings.TrimSpace(after)
		} else if after, ok := strings.CutPrefix(line, "State :"); ok {
			arr.State = strings.TrimSpace(after)
		} else if after, ok := strings.CutPrefix(line, "Active Devices :"); ok {
			_, _ = fmt.Sscanf(after, "%d", &arr.ActiveDevices)
		} else if after, ok := strings.CutPrefix(line, "Total Devices :"); ok {
			_, _ = fmt.Sscanf(after, "%d", &arr.TotalDevices)
		} else if after, ok := strings.CutPrefix(line, "Chunk Size :"); ok {
			arr.StripSize = strings.TrimSpace(after)
		} else if after, ok := strings.CutPrefix(line, "Rebuild Status :"); ok {
			arr.RebuildPct = strings.TrimSpace(after)
		}
	}

	// 解析磁盘列表（在 Number Major Minor RaidDevice State 之后的行）
	inDevSection := false
	for line := range strings.SplitSeq(detail, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Number") && strings.Contains(line, "RaidDevice") {
			inDevSection = true
			continue
		}
		if !inDevSection {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 7 {
			state := fields[4]
			// 有时状态由多个词组成（如 "active sync"）
			if len(fields) >= 8 && (fields[4] == "active" || fields[4] == "spare") {
				state = fields[4] + " " + fields[5]
				// 设备路径在最后一个字段
				arr.Devices = append(arr.Devices, raidDevice{
					Name:  fields[len(fields)-1],
					Slot:  fields[3],
					State: state,
				})
			} else {
				arr.Devices = append(arr.Devices, raidDevice{
					Name:  fields[len(fields)-1],
					Slot:  fields[3],
					State: state,
				})
			}
		}
	}

	return arr
}

// detectMegaRAID 检测 MegaRAID (LSI/Broadcom)
func (s *ToolboxDiskService) detectMegaRAID() chix.M {
	// 检测 storcli64 或 storcli
	storcli := ""
	if _, err := shell.ExecfWithTimeout(5*time.Second, "which storcli64"); err == nil {
		storcli = "storcli64"
	} else if _, err := shell.ExecfWithTimeout(5*time.Second, "which storcli"); err == nil {
		storcli = "storcli"
	}
	if storcli == "" {
		return nil
	}

	output, err := shell.ExecfWithTimeout(30*time.Second, "%s /cALL show all J", storcli)
	if err != nil || output == "" {
		return nil
	}

	controllers, arrays := s.parseMegaRAID(output)
	if len(controllers) == 0 && len(arrays) == 0 {
		return nil
	}

	return chix.M{
		"available":   true,
		"message":     "",
		"type":        "megaraid",
		"controllers": controllers,
		"arrays":      arrays,
	}
}

// parseMegaRAID 解析 storcli JSON 输出
func (s *ToolboxDiskService) parseMegaRAID(output string) ([]raidController, []raidArray) {
	var data map[string]any
	if err := json.Unmarshal([]byte(output), &data); err != nil {
		return nil, nil
	}

	var controllers []raidController
	var arrays []raidArray

	// storcli JSON 结构: Controllers[].Response Data
	ctrlList, _ := data["Controllers"].([]any)
	for _, ctrl := range ctrlList {
		ctrlMap, _ := ctrl.(map[string]any)
		respData, _ := ctrlMap["Response Data"].(map[string]any)
		if respData == nil {
			continue
		}

		// 控制器基本信息
		if basics, ok := respData["Basics"].(map[string]any); ok {
			controllers = append(controllers, raidController{
				Model:    fmt.Sprintf("%v", basics["Model"]),
				Serial:   fmt.Sprintf("%v", basics["Serial Number"]),
				Firmware: fmt.Sprintf("%v", basics["FW Package Build"]),
			})
		}

		// 虚拟磁盘（阵列）
		if vdList, ok := respData["VD LIST"].([]any); ok {
			for _, vd := range vdList {
				vdMap, _ := vd.(map[string]any)
				if vdMap == nil {
					continue
				}
				arr := raidArray{
					Name:      fmt.Sprintf("%v", vdMap["DG/VD"]),
					RaidLevel: fmt.Sprintf("%v", vdMap["TYPE"]),
					Size:      fmt.Sprintf("%v", vdMap["Size"]),
					State:     fmt.Sprintf("%v", vdMap["State"]),
				}
				arrays = append(arrays, arr)
			}
		}

		// 物理磁盘
		if pdList, ok := respData["PD LIST"].([]any); ok {
			for _, pd := range pdList {
				pdMap, _ := pd.(map[string]any)
				if pdMap == nil {
					continue
				}
				dev := raidDevice{
					Slot:   fmt.Sprintf("%v", pdMap["EID:Slt"]),
					Size:   fmt.Sprintf("%v", pdMap["Size"]),
					State:  fmt.Sprintf("%v", pdMap["State"]),
					Model:  fmt.Sprintf("%v", pdMap["Model"]),
					Serial: fmt.Sprintf("%v", pdMap["SN"]),
				}
				// 将物理磁盘分配到对应的阵列
				if dgStr, ok := pdMap["DG"].(float64); ok && int(dgStr) < len(arrays) {
					arrays[int(dgStr)].Devices = append(arrays[int(dgStr)].Devices, dev)
				}
			}
		}
	}

	return controllers, arrays
}

// detectHPSA 检测 HP Smart Array
func (s *ToolboxDiskService) detectHPSA() chix.M {
	// 检测 ssacli 或 hpssacli
	ssacli := ""
	if _, err := shell.ExecfWithTimeout(5*time.Second, "which ssacli"); err == nil {
		ssacli = "ssacli"
	} else if _, err := shell.ExecfWithTimeout(5*time.Second, "which hpssacli"); err == nil {
		ssacli = "hpssacli"
	}
	if ssacli == "" {
		return nil
	}

	output, err := shell.ExecfWithTimeout(30*time.Second, "%s ctrl all show config detail", ssacli)
	if err != nil || output == "" {
		return nil
	}

	controllers, arrays := s.parseHPSA(output)
	if len(controllers) == 0 && len(arrays) == 0 {
		return nil
	}

	return chix.M{
		"available":   true,
		"message":     "",
		"type":        "hpsa",
		"controllers": controllers,
		"arrays":      arrays,
	}
}

// parseHPSA 解析 ssacli 文本输出
func (s *ToolboxDiskService) parseHPSA(output string) ([]raidController, []raidArray) {
	var controllers []raidController
	var arrays []raidArray

	var currentCtrl *raidController
	var currentArray *raidArray
	var currentDev *raidDevice

	for line := range strings.SplitSeq(output, "\n") {
		trimmed := strings.TrimSpace(line)

		// 控制器
		if strings.Contains(trimmed, "Smart Array") || strings.Contains(trimmed, "Smart HBA") {
			if currentCtrl != nil {
				controllers = append(controllers, *currentCtrl)
			}
			currentCtrl = &raidController{Model: trimmed}
		}

		if currentCtrl != nil {
			if after, ok := strings.CutPrefix(trimmed, "Serial Number:"); ok {
				currentCtrl.Serial = strings.TrimSpace(after)
			} else if after, ok := strings.CutPrefix(trimmed, "Firmware Version:"); ok {
				currentCtrl.Firmware = strings.TrimSpace(after)
			} else if strings.HasPrefix(trimmed, "Cache Board Present:") || strings.HasPrefix(trimmed, "Total Cache Size:") {
				currentCtrl.Cache = strings.TrimSpace(strings.SplitN(trimmed, ":", 2)[1])
			}
		}

		// 阵列
		if strings.HasPrefix(trimmed, "Array:") || (strings.HasPrefix(trimmed, "array") && strings.Contains(trimmed, "Array")) {
			if currentArray != nil {
				if currentDev != nil {
					currentArray.Devices = append(currentArray.Devices, *currentDev)
					currentDev = nil
				}
				arrays = append(arrays, *currentArray)
			}
			currentArray = &raidArray{Name: trimmed}
		}

		if currentArray != nil {
			if after, ok := strings.CutPrefix(trimmed, "Fault Tolerance:"); ok {
				currentArray.RaidLevel = strings.TrimSpace(after)
			} else if after, ok := strings.CutPrefix(trimmed, "Size:"); ok {
				currentArray.Size = strings.TrimSpace(after)
			} else if after, ok := strings.CutPrefix(trimmed, "Status:"); ok {
				currentArray.State = strings.TrimSpace(after)
			} else if after, ok := strings.CutPrefix(trimmed, "Strip Size:"); ok {
				currentArray.StripSize = strings.TrimSpace(after)
			}

			// 物理磁盘
			if strings.HasPrefix(trimmed, "physicaldrive") {
				if currentDev != nil {
					currentArray.Devices = append(currentArray.Devices, *currentDev)
				}
				currentDev = &raidDevice{Name: trimmed}
			}
			if currentDev != nil {
				if strings.HasPrefix(trimmed, "Port:") || strings.HasPrefix(trimmed, "Bay:") {
					currentDev.Slot = strings.TrimSpace(strings.SplitN(trimmed, ":", 2)[1])
				} else if after, ok := strings.CutPrefix(trimmed, "Size:"); ok {
					currentDev.Size = strings.TrimSpace(after)
				} else if after, ok := strings.CutPrefix(trimmed, "Status:"); ok {
					currentDev.State = strings.TrimSpace(after)
				} else if after, ok := strings.CutPrefix(trimmed, "Model:"); ok {
					currentDev.Model = strings.TrimSpace(after)
				} else if after, ok := strings.CutPrefix(trimmed, "Serial Number:"); ok {
					currentDev.Serial = strings.TrimSpace(after)
				}
			}
		}
	}

	// 收尾
	if currentDev != nil && currentArray != nil {
		currentArray.Devices = append(currentArray.Devices, *currentDev)
	}
	if currentArray != nil {
		arrays = append(arrays, *currentArray)
	}
	if currentCtrl != nil {
		controllers = append(controllers, *currentCtrl)
	}

	return controllers, arrays
}

// detectAdaptec 检测 Adaptec RAID
func (s *ToolboxDiskService) detectAdaptec() chix.M {
	if _, err := shell.ExecfWithTimeout(5*time.Second, "which arcconf"); err != nil {
		return nil
	}

	output, err := shell.ExecfWithTimeout(30*time.Second, "arcconf GETCONFIG 1")
	if err != nil || output == "" {
		return nil
	}

	controllers, arrays := s.parseAdaptec(output)
	if len(controllers) == 0 && len(arrays) == 0 {
		return nil
	}

	return chix.M{
		"available":   true,
		"message":     "",
		"type":        "adaptec",
		"controllers": controllers,
		"arrays":      arrays,
	}
}

// parseAdaptec 解析 arcconf GETCONFIG 输出
func (s *ToolboxDiskService) parseAdaptec(output string) ([]raidController, []raidArray) {
	var controllers []raidController
	var arrays []raidArray

	var currentCtrl *raidController
	var currentArray *raidArray
	var currentDev *raidDevice
	inLogicalDev := false
	inPhysicalDev := false

	for line := range strings.SplitSeq(output, "\n") {
		trimmed := strings.TrimSpace(line)

		// 控制器信息
		if strings.Contains(trimmed, "Controller Model") {
			val := s.extractAdaptecValue(trimmed)
			currentCtrl = &raidController{Model: val}
		}
		if currentCtrl != nil {
			if strings.Contains(trimmed, "Controller Serial Number") {
				currentCtrl.Serial = s.extractAdaptecValue(trimmed)
			} else if strings.Contains(trimmed, "Firmware") && strings.Contains(trimmed, "Version") {
				currentCtrl.Firmware = s.extractAdaptecValue(trimmed)
			}
		}

		// 逻辑设备段
		if strings.HasPrefix(trimmed, "Logical Device number") || strings.HasPrefix(trimmed, "Logical device number") {
			if currentArray != nil {
				if currentDev != nil {
					currentArray.Devices = append(currentArray.Devices, *currentDev)
					currentDev = nil
				}
				arrays = append(arrays, *currentArray)
			}
			currentArray = &raidArray{Name: trimmed}
			inLogicalDev = true
			inPhysicalDev = false
			continue
		}

		if inLogicalDev && currentArray != nil {
			if strings.HasPrefix(trimmed, "RAID level") {
				currentArray.RaidLevel = s.extractAdaptecValue(trimmed)
			} else if strings.HasPrefix(trimmed, "Size") {
				currentArray.Size = s.extractAdaptecValue(trimmed)
			} else if strings.HasPrefix(trimmed, "Status of Logical Device") || strings.HasPrefix(trimmed, "Status of logical device") {
				currentArray.State = s.extractAdaptecValue(trimmed)
			} else if strings.HasPrefix(trimmed, "Stripe-size") || strings.HasPrefix(trimmed, "Strip Size") {
				currentArray.StripSize = s.extractAdaptecValue(trimmed)
			}
		}

		// 物理设备段
		if strings.Contains(trimmed, "Device #") || strings.HasPrefix(trimmed, "Physical Device") {
			if currentDev != nil && currentArray != nil {
				currentArray.Devices = append(currentArray.Devices, *currentDev)
			}
			currentDev = &raidDevice{Name: trimmed}
			inPhysicalDev = true
		}

		if inPhysicalDev && currentDev != nil {
			if strings.HasPrefix(trimmed, "State") {
				currentDev.State = s.extractAdaptecValue(trimmed)
			} else if strings.HasPrefix(trimmed, "Size") {
				currentDev.Size = s.extractAdaptecValue(trimmed)
			} else if strings.HasPrefix(trimmed, "Model") {
				currentDev.Model = s.extractAdaptecValue(trimmed)
			} else if strings.HasPrefix(trimmed, "Serial number") || strings.HasPrefix(trimmed, "Serial Number") {
				currentDev.Serial = s.extractAdaptecValue(trimmed)
			} else if strings.HasPrefix(trimmed, "Reported Channel,Device") {
				currentDev.Slot = s.extractAdaptecValue(trimmed)
			}
		}
	}

	// 收尾
	if currentDev != nil && currentArray != nil {
		currentArray.Devices = append(currentArray.Devices, *currentDev)
	}
	if currentArray != nil {
		arrays = append(arrays, *currentArray)
	}
	if currentCtrl != nil {
		controllers = append(controllers, *currentCtrl)
	}

	return controllers, arrays
}

// extractAdaptecValue 从 "Key : Value" 格式中提取值
func (s *ToolboxDiskService) extractAdaptecValue(line string) string {
	parts := strings.SplitN(line, ":", 2)
	if len(parts) < 2 {
		return ""
	}
	return strings.TrimSpace(parts[1])
}
