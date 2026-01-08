package request

// ToolboxDiskDevice 磁盘设备请求
type ToolboxDiskDevice struct {
	Device string `form:"device" json:"device" validate:"required"`
}

// ToolboxDiskMount 挂载请求
type ToolboxDiskMount struct {
	Device string `form:"device" json:"device" validate:"required"`
	Path   string `form:"path" json:"path" validate:"required"`
}

// ToolboxDiskUmount 卸载请求
type ToolboxDiskUmount struct {
	Path string `form:"path" json:"path" validate:"required"`
}

// ToolboxDiskFormat 格式化请求
type ToolboxDiskFormat struct {
	Device string `form:"device" json:"device" validate:"required"`
	FsType string `form:"fs_type" json:"fs_type" validate:"required|in:ext4,ext3,xfs,btrfs"`
}

// ToolboxDiskVG 卷组请求
type ToolboxDiskVG struct {
	Name    string   `form:"name" json:"name" validate:"required"`
	Devices []string `form:"devices" json:"devices" validate:"required"`
}

// ToolboxDiskLV 逻辑卷请求
type ToolboxDiskLV struct {
	Name   string `form:"name" json:"name" validate:"required"`
	VGName string `form:"vg_name" json:"vg_name" validate:"required"`
	Size   int    `form:"size" json:"size" validate:"required|min:1"`
}

// ToolboxDiskVGName 卷组名称请求
type ToolboxDiskVGName struct {
	Name string `form:"name" json:"name" validate:"required"`
}

// ToolboxDiskLVPath 逻辑卷路径请求
type ToolboxDiskLVPath struct {
	Path string `form:"path" json:"path" validate:"required"`
}

// ToolboxDiskExtendLV 扩容逻辑卷请求
type ToolboxDiskExtendLV struct {
	Path   string `form:"path" json:"path" validate:"required"`
	Size   int    `form:"size" json:"size" validate:"required|min:1"`
	Resize bool   `form:"resize" json:"resize"`
}
