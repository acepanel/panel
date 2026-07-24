package request

import (
	"net/http"

	"github.com/spf13/cast"
)

type FileList struct {
	Path    string `json:"path" form:"path" validate:"required && unix_path"`
	Sort    string `json:"sort" form:"sort"`
	Keyword string `form:"keyword" json:"keyword"`
	Sub     bool   `form:"sub" json:"sub"`
}

func (r *FileList) Prepare(req *http.Request) error {
	r.Sub = cast.ToBool(req.FormValue("sub"))
	return nil
}

type FilePath struct {
	Path string `json:"path" form:"path" validate:"required && unix_path"`
}

type FileTail struct {
	Path      string `json:"path" form:"path"`
	Service   string `json:"service" form:"service"`
	Container string `json:"container" form:"container"`
	Offset    int    `json:"offset" form:"offset"`
	Limit     int    `json:"limit" form:"limit"`
	Cursor    string `json:"cursor" form:"cursor"`
}

type FileFollow struct {
	Path      string `json:"path" form:"path"`
	Service   string `json:"service" form:"service"`
	Container string `json:"container" form:"container"`
}

type FileCreate struct {
	Dir  bool   `json:"dir" form:"dir"`
	Path string `json:"path" form:"path" validate:"required && unix_path"`
}

type FileSave struct {
	Path    string `form:"path" json:"path" validate:"required && unix_path"`
	Content string `form:"content" json:"content"`
}

type FileControl struct {
	Source string `form:"source" json:"source" validate:"required && unix_path"`
	Target string `form:"target" json:"target" validate:"required && unix_path"`
	Force  bool   `form:"force" json:"force"`
}

type FileRemoteDownload struct {
	Path string `form:"path" json:"path" validate:"required && unix_path"`
	URL  string `form:"url" json:"url" validate:"required && url"`
}

type FilePermission struct {
	Path  string `form:"path" json:"path" validate:"required && unix_path"`
	Mode  string `form:"mode" json:"mode" validate:"required"`
	Owner string `form:"owner" json:"owner" validate:"required"`
	Group string `form:"group" json:"group" validate:"required"`
}

type FileCompress struct {
	Dir   string   `form:"dir" json:"dir" validate:"required && unix_path"`
	Paths []string `form:"paths" json:"paths" validate:"required && unique && dive && required"`
	File  string   `form:"file" json:"file" validate:"required && unix_path"`
}

type FileUnCompress struct {
	File string `form:"file" json:"file" validate:"required && unix_path"`
	Path string `form:"path" json:"path" validate:"required && unix_path"`
}

type FileShareCreate struct {
	Path         string `form:"path" json:"path" validate:"required && unix_path"`
	MaxDownloads uint   `form:"max_downloads" json:"max_downloads"`                                        // 最大下载次数（0 不限）
	ExpireHours  uint   `form:"expire_hours" json:"expire_hours" validate:"required && min:1 && max:8760"` // 有效期（小时）
}

type FileShareToken struct {
	Token string `json:"token" form:"token" uri:"token" validate:"required"`
}

// ChunkUploadStart 分块上传开始请求
type ChunkUploadStart struct {
	Path       string `json:"path" validate:"required && unix_path"`    // 目标目录
	FileName   string `json:"file_name" validate:"required"`            // 文件名
	FileHash   string `json:"file_hash" validate:"required && len:64"`  // 文件SHA256
	ChunkCount int    `json:"chunk_count" validate:"required && min:1"` // 分块总数
	Force      bool   `json:"force"`                                    // 是否覆盖已存在文件
}

// ChunkUploadFinish 分块上传完成请求
type ChunkUploadFinish struct {
	Path       string `json:"path" validate:"required && unix_path"`    // 目标目录
	FileName   string `json:"file_name" validate:"required"`            // 文件名
	FileHash   string `json:"file_hash" validate:"required && len:64"`  // 文件SHA256
	ChunkCount int    `json:"chunk_count" validate:"required && min:1"` // 分块总数
	Force      bool   `json:"force"`                                    // 是否覆盖已存在文件
}
