package request

type ContainerImageID struct {
	ID string `json:"id" form:"id"`
}

type ContainerImageExport struct {
	ID   string `uri:"id" validate:"required"`
	Path string `form:"path" json:"path" validate:"unix_path"` // 保存目录，留空用备份目录下的 image
}

type ContainerImageImport struct {
	Path string `form:"path" json:"path" validate:"required && unix_path"` // 镜像归档文件
}

type ContainerImagePull struct {
	Name       string `form:"name" json:"name" validate:"required"`
	Background bool   `form:"background" json:"background"`
	Auth       bool   `form:"auth" json:"auth"`
	Username   string `form:"username" json:"username" validate:"required_if:Auth,true"`
	Password   string `form:"password" json:"password" validate:"required_if:Auth,true"`
}
