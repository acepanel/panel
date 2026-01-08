package service

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/chix"
	"github.com/spf13/cast"

	"github.com/acepanel/panel/internal/http/request"
	"github.com/acepanel/panel/pkg/shell"
)

type ToolboxDiskService struct {
	t *gotext.Locale
}

func NewToolboxDiskService(t *gotext.Locale) *ToolboxDiskService {
	return &ToolboxDiskService{
		t: t,
	}
}

// List 获取磁盘列表
func (s *ToolboxDiskService) List(w http.ResponseWriter, r *http.Request) {
	// 使用 lsblk 获取磁盘信息
	output, err := shell.Execf("lsblk -J -b -o NAME,SIZE,TYPE,MOUNTPOINT,FSTYPE,UUID,LABEL,MODEL")
	if err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("failed to get disk list: %v", err))
		return
	}

	Success(w, output)
}

// GetPartitions 获取分区列表
func (s *ToolboxDiskService) GetPartitions(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ToolboxDiskDevice](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	// 获取指定磁盘的分区信息
	output, err := shell.Execf("lsblk -J -b -o NAME,SIZE,TYPE,MOUNTPOINT,FSTYPE,UUID,LABEL /dev/%s", req.Device)
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

	// 检查挂载点是否存在，不存在则创建
	if _, err = shell.Execf("test -d '%s' || mkdir -p '%s'", req.Path, req.Path); err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("failed to create mount point: %v", err))
		return
	}

	// 挂载分区
	mountCmd := fmt.Sprintf("mount /dev/%s '%s'", req.Device, req.Path)
	if _, err = shell.Execf(mountCmd); err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("failed to mount partition: %v", err))
		return
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

	// 卸载分区
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

	// 根据文件系统类型格式化
	var formatCmd string
	switch req.FsType {
	case "ext4":
		formatCmd = fmt.Sprintf("mkfs.ext4 -F /dev/%s", req.Device)
	case "ext3":
		formatCmd = fmt.Sprintf("mkfs.ext3 -F /dev/%s", req.Device)
	case "xfs":
		formatCmd = fmt.Sprintf("mkfs.xfs -f /dev/%s", req.Device)
	case "btrfs":
		formatCmd = fmt.Sprintf("mkfs.btrfs -f /dev/%s", req.Device)
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

	if _, err = shell.Execf("pvcreate /dev/%s", req.Device); err != nil {
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

	// 构建设备列表
	devices := make([]string, len(req.Devices))
	for i, dev := range req.Devices {
		devices[i] = "/dev/" + dev
	}

	if _, err = shell.Execf("vgcreate %s %s", req.Name, strings.Join(devices, " ")); err != nil {
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

	// 创建逻辑卷
	if _, err = shell.Execf("lvcreate -L %sG -n %s %s", cast.ToString(req.Size), req.Name, req.VGName); err != nil {
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

	if _, err = shell.Execf("pvremove /dev/%s", req.Device); err != nil {
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

	if _, err = shell.Execf("vgremove -f %s", req.Name); err != nil {
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

	if _, err = shell.Execf("lvremove -f %s", req.Path); err != nil {
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

	// 扩容逻辑卷
	if _, err = shell.Execf("lvextend -L +%sG %s", cast.ToString(req.Size), req.Path); err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("failed to extend logical volume: %v", err))
		return
	}

	// 扩展文件系统
	if req.Resize {
		// 检测文件系统类型并扩展
		fsType, _ := shell.Execf("blkid -o value -s TYPE %s", req.Path)
		fsType = strings.TrimSpace(fsType)

		switch fsType {
		case "ext4", "ext3":
			if _, err = shell.Execf("resize2fs %s", req.Path); err != nil {
				Error(w, http.StatusInternalServerError, s.t.Get("failed to resize filesystem: %v", err))
				return
			}
		case "xfs":
			// XFS需要挂载后才能扩展
			mountPoint, _ := shell.Execf("findmnt -n -o TARGET %s", req.Path)
			mountPoint = strings.TrimSpace(mountPoint)
			if mountPoint != "" {
				if _, err = shell.Execf("xfs_growfs %s", mountPoint); err != nil {
					Error(w, http.StatusInternalServerError, s.t.Get("failed to resize filesystem: %v", err))
					return
				}
			}
		}
	}

	Success(w, nil)
}

// parseLVMOutput 解析LVM命令输出
func (s *ToolboxDiskService) parseLVMOutput(output string) []map[string]string {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	var result []map[string]string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// 使用正则去除多余空格
		re := regexp.MustCompile(`\s+`)
		line = re.ReplaceAllString(line, " ")

		fields := strings.Split(line, "|")
		item := make(map[string]string)

		for i, field := range fields {
			item[fmt.Sprintf("field_%d", i)] = strings.TrimSpace(field)
		}

		result = append(result, item)
	}

	return result
}
