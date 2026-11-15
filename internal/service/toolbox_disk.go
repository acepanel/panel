package service

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/chix/v2"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/pkg/disk"
	"github.com/acepanel/panel/v3/pkg/shell"
)

type ToolboxDiskService struct {
	t   *gotext.Locale
	log *slog.Logger
}

func NewToolboxDiskService(t *gotext.Locale, log *slog.Logger) *ToolboxDiskService {
	return &ToolboxDiskService{
		t:   t,
		log: log,
	}
}

// List 获取磁盘列表
func (s *ToolboxDiskService) List(w http.ResponseWriter, r *http.Request) {
	disks, err := disk.BlockDevices(r.Context())
	if err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("failed to get disk list: %v", err))
		return
	}
	usages := disk.Usages(r.Context())

	s.log.Info(s.t.Get("disk list viewed"), slog.String("type", biz.OperationTypeToolbox), slog.Uint64("operator_id", biz.OperatorID(r.Context())))
	Success(w, chix.M{
		"disks": disks,
		"df":    usages,
	})
}

// GetPartitions 获取分区列表
func (s *ToolboxDiskService) GetPartitions(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ToolboxDiskDevice](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	output, err := shell.Execf(r.Context(), "lsblk -J -b -o NAME,SIZE,TYPE,MOUNTPOINT,FSTYPE,UUID,LABEL '/dev/%s'", req.Device)
	if err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("failed to get partitions: %v", err))
		return
	}

	s.log.Info(s.t.Get("partition list viewed"), slog.String("type", biz.OperationTypeToolbox), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("device", req.Device))
	Success(w, output)
}

// Mount 挂载分区
func (s *ToolboxDiskService) Mount(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ToolboxDiskMount](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if req.WriteFstab && disk.FstabHas(req.Path) {
		Error(w, http.StatusBadRequest, s.t.Get("mount point %s already exists in fstab", req.Path))
		return
	}

	// 挂载成功后必须把 fstab 写完，否则重复挂载会直接失败
	ctx := context.WithoutCancel(r.Context())

	if _, err = shell.Execf(ctx, "test -d '%s' || mkdir -p '%s'", req.Path, req.Path); err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("failed to create mount point: %v", err))
		return
	}

	if _, err = shell.Execf(ctx, "mount '/dev/%s' '%s'", req.Device, req.Path); err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("failed to mount partition: %v", err))
		return
	}

	// 如果需要写入 fstab
	if req.WriteFstab {
		// 获取分区的 UUID
		uuid, err := shell.Execf(ctx, "blkid -s UUID -o value '/dev/%s'", req.Device)
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
		fsType, err := shell.Execf(ctx, "blkid -s TYPE -o value '/dev/%s'", req.Device)
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

		// 写入 fstab
		fstabEntry := fmt.Sprintf("UUID=%s %s %s %s 0 2", uuid, req.Path, fsType, mountOption)
		if _, err = shell.Execf(ctx, "echo '%s' >> /etc/fstab", fstabEntry); err != nil {
			Error(w, http.StatusInternalServerError, s.t.Get("failed to write fstab: %v", err))
			return
		}
	}

	s.log.Info(s.t.Get("partition mounted"), slog.String("type", biz.OperationTypeToolbox), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("device", req.Device), slog.String("path", req.Path), slog.Bool("write_fstab", req.WriteFstab), slog.String("mount_option", req.MountOption))
	Success(w, nil)
}

// Umount 卸载分区
func (s *ToolboxDiskService) Umount(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ToolboxDiskUmount](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if _, err = shell.Execf(r.Context(), "umount '%s'", req.Path); err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("failed to umount partition: %v", err))
		return
	}

	s.log.Warn(s.t.Get("partition unmounted"), slog.String("type", biz.OperationTypeToolbox), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("path", req.Path))
	Success(w, nil)
}

// Format 格式化分区
func (s *ToolboxDiskService) Format(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ToolboxDiskFormat](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	formatCmd := s.mkfsCommand(req.FsType, "/dev/"+req.Device)
	if formatCmd == "" {
		Error(w, http.StatusUnprocessableEntity, s.t.Get("unsupported filesystem type: %s", req.FsType))
		return
	}

	// mkfs 写到一半被杀会留下无法挂载的分区，同 Init 一样不跟随请求取消
	if _, err = shell.Exec(context.WithoutCancel(r.Context()), formatCmd); err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("failed to format partition: %v", err))
		return
	}

	s.log.Warn(s.t.Get("partition formatted"), slog.String("type", biz.OperationTypeToolbox), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("device", req.Device), slog.String("fs_type", req.FsType))
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
	partDevice := disk.PartitionName(device, 1)

	formatCmd := s.mkfsCommand(req.FsType, partDevice)
	if formatCmd == "" {
		Error(w, http.StatusUnprocessableEntity, s.t.Get("unsupported filesystem type: %s", req.FsType))
		return
	}

	// 擦除、分区、格式化是一条链，中途取消会留下无法使用的裸盘
	ctx := context.WithoutCancel(r.Context())

	// 检查设备是否存在
	if _, err = shell.Execf(ctx, "test -b '%s'", device); err != nil {
		Error(w, http.StatusBadRequest, s.t.Get("device not found: %s", device))
		return
	}

	// 检查是否为系统盘（检查是否有分区挂载在 /）
	mountInfo, _ := shell.Execf(ctx, "lsblk -no MOUNTPOINT '%s' 2>/dev/null", device)
	if slices.Contains(strings.Fields(mountInfo), "/") {
		Error(w, http.StatusBadRequest, s.t.Get("cannot initialize system disk"))
		return
	}

	// 卸载该磁盘的所有分区
	_, _ = shell.Execf(ctx, "umount '%s'* 2>/dev/null || true", device)

	// 先清除分区表
	if _, err = shell.Execf(ctx, "wipefs -a '%s'", device); err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("failed to wipe disk: %v", err))
		return
	}

	// sfdisk 创建 GPT 分区表和单个分区
	if _, err = shell.Execf(ctx, "echo 'type=linux' | sfdisk '%s'", device); err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("failed to create partition: %v", err))
		return
	}

	// 等待内核更新分区表
	_, _ = shell.Execf(ctx, "partprobe '%s' 2>/dev/null || true", device)
	time.Sleep(time.Second)

	if _, err = shell.Exec(ctx, formatCmd); err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("failed to format partition: %v", err))
		return
	}

	s.log.Warn(s.t.Get("disk initialized"), slog.String("type", biz.OperationTypeToolbox), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("device", req.Device), slog.String("partition", partDevice), slog.String("fs_type", req.FsType))
	Success(w, nil)
}

// GetLVMInfo 获取LVM信息
func (s *ToolboxDiskService) GetLVMInfo(w http.ResponseWriter, r *http.Request) {
	info := disk.LVM(r.Context())

	s.log.Info(s.t.Get("lvm info viewed"), slog.String("type", biz.OperationTypeToolbox), slog.Uint64("operator_id", biz.OperatorID(r.Context())))
	Success(w, info)
}

// CreatePV 创建物理卷
func (s *ToolboxDiskService) CreatePV(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ToolboxDiskDevice](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if _, err = shell.Execf(r.Context(), "pvcreate '/dev/%s'", req.Device); err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("failed to create physical volume: %v", err))
		return
	}

	s.log.Info(s.t.Get("physical volume created"), slog.String("type", biz.OperationTypeToolbox), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("device", req.Device))
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

	if _, err = shell.Execf(r.Context(), "vgcreate '%s' %s", req.Name, strings.Join(deviceArgs, " ")); err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("failed to create volume group: %v", err))
		return
	}

	s.log.Info(s.t.Get("volume group created"), slog.String("type", biz.OperationTypeToolbox), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("name", req.Name), slog.Any("devices", req.Devices))
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
	if _, err = shell.Execf(r.Context(), "lvcreate -L '%dG' -n '%s' '%s'", req.Size, req.Name, req.VGName); err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("failed to create logical volume: %v", err))
		return
	}

	s.log.Info(s.t.Get("logical volume created"), slog.String("type", biz.OperationTypeToolbox), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("name", req.Name), slog.String("vg_name", req.VGName), slog.Int("size_gb", req.Size))
	Success(w, nil)
}

// RemovePV 删除物理卷
func (s *ToolboxDiskService) RemovePV(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ToolboxDiskDevice](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if _, err = shell.Execf(r.Context(), "pvremove '%s'", req.Device); err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("failed to remove physical volume: %v", err))
		return
	}

	s.log.Warn(s.t.Get("physical volume deleted"), slog.String("type", biz.OperationTypeToolbox), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("device", req.Device))
	Success(w, nil)
}

// RemoveVG 删除卷组
func (s *ToolboxDiskService) RemoveVG(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ToolboxDiskVGName](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if _, err = shell.Execf(r.Context(), "vgremove -f '%s'", req.Name); err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("failed to remove volume group: %v", err))
		return
	}

	s.log.Warn(s.t.Get("volume group deleted"), slog.String("type", biz.OperationTypeToolbox), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("name", req.Name))
	Success(w, nil)
}

// RemoveLV 删除逻辑卷
func (s *ToolboxDiskService) RemoveLV(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ToolboxDiskLVPath](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if _, err = shell.Execf(r.Context(), "lvremove -f '%s'", req.Path); err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("failed to remove logical volume: %v", err))
		return
	}

	s.log.Warn(s.t.Get("logical volume deleted"), slog.String("type", biz.OperationTypeToolbox), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("path", req.Path))
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

	// 逻辑卷扩容后必须接着扩展文件系统，中断会让新增空间不可用
	ctx := context.WithoutCancel(r.Context())

	// 扩容逻辑卷
	if _, err = shell.Execf(ctx, "lvextend -L +%dG '%s'", req.Size, req.Path); err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("failed to extend logical volume: %v", err))
		return
	}

	// 扩展文件系统
	if req.Resize {
		// 检测文件系统类型并扩展
		fsType, _ := shell.Execf(ctx, "blkid -o value -s TYPE '%s'", req.Path)
		fsType = strings.TrimSpace(fsType)

		switch fsType {
		case "ext4", "ext3":
			if _, err = shell.Execf(ctx, "resize2fs '%s'", req.Path); err != nil {
				Error(w, http.StatusInternalServerError, s.t.Get("failed to resize filesystem: %v", err))
				return
			}
		case "xfs":
			// XFS需要挂载后才能扩展
			mountPoint, _ := shell.Execf(ctx, "findmnt -n -o TARGET '%s'", req.Path)
			mountPoint = strings.TrimSpace(mountPoint)
			if mountPoint == "" {
				// XFS未挂载时，返回错误信息
				Error(w, http.StatusInternalServerError, s.t.Get("xfs filesystem is not mounted, logical volume has been extended but filesystem was not resized"))
				return
			}
			if _, err = shell.Execf(ctx, "xfs_growfs '%s'", mountPoint); err != nil {
				Error(w, http.StatusInternalServerError, s.t.Get("failed to resize filesystem: %v", err))
				return
			}
		case "btrfs":
			// btrfs需要挂载后才能扩展
			mountPoint, _ := shell.Execf(ctx, "findmnt -n -o TARGET '%s'", req.Path)
			mountPoint = strings.TrimSpace(mountPoint)
			if mountPoint == "" {
				// btrfs未挂载时，返回错误信息
				Error(w, http.StatusInternalServerError, s.t.Get("btrfs filesystem is not mounted, logical volume has been extended but filesystem was not resized"))
				return
			}
			// 扩展到当前可用的最大空间
			if _, err = shell.Execf(ctx, "btrfs filesystem resize max '%s'", mountPoint); err != nil {
				Error(w, http.StatusInternalServerError, s.t.Get("failed to resize filesystem: %v", err))
				return
			}
		}
	}

	s.log.Info(s.t.Get("logical volume extended"), slog.String("type", biz.OperationTypeToolbox), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("path", req.Path), slog.Int("size_gb", req.Size), slog.Bool("resize", req.Resize))
	Success(w, nil)
}

// GetFstab 获取 fstab 列表
func (s *ToolboxDiskService) GetFstab(w http.ResponseWriter, r *http.Request) {
	entries, err := disk.Fstab()
	if err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("failed to read fstab: %v", err))
		return
	}

	s.log.Info(s.t.Get("fstab list viewed"), slog.String("type", biz.OperationTypeToolbox), slog.Uint64("operator_id", biz.OperatorID(r.Context())))
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

	// 改完 fstab 必须接着 mount -a 生效，中断会让挂载状态和 fstab 对不上
	ctx := context.WithoutCancel(r.Context())

	if _, err = shell.Execf(ctx, `sed -E -i 's@^[^#].*\s%s\s.*$@@g' /etc/fstab`, regexp.QuoteMeta(req.MountPoint)); err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("failed to delete fstab entry: %v", err))
		return
	}

	if _, err = shell.Execf(ctx, "mount -a"); err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("failed to remount filesystems: %v", err))
		return
	}

	s.log.Warn(s.t.Get("fstab entry deleted"), slog.String("type", biz.OperationTypeToolbox), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("mount_point", req.MountPoint))
	Success(w, nil)
}

// GetSmartDisks 获取支持 SMART 的磁盘列表
func (s *ToolboxDiskService) GetSmartDisks(w http.ResponseWriter, r *http.Request) {
	if !disk.SMARTInstalled() {
		Success(w, chix.M{
			"available": false,
			"message":   s.t.Get("smartmontools is not installed, please install it first (e.g., apt install smartmontools or dnf install smartmontools)"),
			"disks":     []any{},
		})
		return
	}

	disks, err := disk.SMARTDisks(r.Context())
	if err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("failed to parse smartctl output: %v", err))
		return
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

	info, err := disk.SMARTInfo(r.Context(), req.Device)
	if err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("failed to get SMART info for device %s", req.Device))
		return
	}

	s.log.Info(s.t.Get("smart info viewed"), slog.String("type", biz.OperationTypeToolbox), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("device", req.Device))
	Success(w, info)
}

// GetRaidInfo 获取 RAID 阵列状态
func (s *ToolboxDiskService) GetRaidInfo(w http.ResponseWriter, r *http.Request) {
	info := chix.M{
		"available":   false,
		"message":     s.t.Get("no RAID configuration detected"),
		"type":        "",
		"controllers": []any{},
		"arrays":      []any{},
	}
	if raid := disk.DetectRAID(r.Context()); raid != nil {
		info = chix.M{
			"available":   true,
			"message":     "",
			"type":        raid.Type,
			"controllers": raid.Controllers,
			"arrays":      raid.Arrays,
		}
	}

	s.log.Info(s.t.Get("raid info viewed"), slog.String("type", biz.OperationTypeToolbox), slog.Uint64("operator_id", biz.OperatorID(r.Context())))
	Success(w, info)
}

// mkfsCommand 返回格式化命令，不支持的文件系统返回空串
func (s *ToolboxDiskService) mkfsCommand(fsType, device string) string {
	switch fsType {
	case "ext4", "ext3":
		return fmt.Sprintf("mkfs.%s -F '%s'", fsType, device)
	case "xfs", "btrfs":
		return fmt.Sprintf("mkfs.%s -f '%s'", fsType, device)
	default:
		return ""
	}
}
