package request

import (
	"mime/multipart"
	"net/http"

	"github.com/spf13/cast"
)

type FileList struct {
	Path          string `json:"path" form:"path" validate:"required && unix_path"`
	Sort          string `json:"sort" form:"sort"`
	Keyword       string `form:"keyword" json:"keyword"`
	Sub           bool   `form:"sub" json:"sub"`
	ExcludeHidden bool   `form:"exclude_hidden" json:"exclude_hidden"`
}

func (r *FileList) Prepare(req *http.Request) error {
	r.Sub = cast.ToBool(req.FormValue("sub"))
	r.ExcludeHidden = cast.ToBool(req.FormValue("exclude_hidden"))
	return nil
}

type FilePath struct {
	Path string `json:"path" form:"path" validate:"required && unix_path"`
}

type FileContent struct {
	Path     string `json:"path" form:"path" validate:"required && unix_path"`
	Encoding string `json:"encoding" form:"encoding"`
}

type FileTail struct {
	Path      string `json:"path" form:"path"`
	Service   string `json:"service" form:"service"`
	Container string `json:"container" form:"container"`
	Offset    int    `json:"offset" form:"offset"`
	Limit     int    `json:"limit" form:"limit"`
	Cursor    string `json:"cursor" form:"cursor"`
	// Size 为首屏返回的文件大小，翻页时回传作为反向分页锚点，避免日志持续写入导致错位
	Size int64 `json:"size" form:"size"`
}

type FileFollow struct {
	Path      string `json:"path" form:"path"`
	Service   string `json:"service" form:"service"`
	Container string `json:"container" form:"container"`
	// Offset 为首屏锚点字节位置，从该处开始跟踪以衔接首屏与实时流，避免中间写入的日志丢失
	Offset int64 `json:"offset" form:"offset"`
}

type FileCreate struct {
	Dir  bool   `json:"dir" form:"dir"`
	Path string `json:"path" form:"path" validate:"required && unix_path"`
}

type FileSave struct {
	Path     string `form:"path" json:"path" validate:"required && unix_path"`
	Content  string `form:"content" json:"content"`
	Encoding string `form:"encoding" json:"encoding"`
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
	Path      string `form:"path" json:"path" validate:"required && unix_path"`
	Mode      string `form:"mode" json:"mode" validate:"required"`
	Owner     string `form:"owner" json:"owner" validate:"required"`
	Group     string `form:"group" json:"group" validate:"required"`
	Recursive bool   `form:"recursive" json:"recursive"`
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

// FileUpload 上传文件请求
type FileUpload struct {
	Path  string                `form:"path" validate:"required && unix_path"` // 目标路径
	Force bool                  `form:"force"`                                 // 是否覆盖已存在文件
	File  *multipart.FileHeader `form:"file" validate:"required"`              // 文件
}

// ChunkUploadFile 分块上传的文件标识，各接口共用
type ChunkUploadFile struct {
	Path     string `json:"path" form:"path" validate:"required && unix_path"`        // 目标目录
	FileName string `json:"file_name" form:"file_name" validate:"required"`           // 文件名
	FileHash string `json:"file_hash" form:"file_hash" validate:"required && len:64"` // 文件SHA256
}

// ChunkUploadStart 分块上传开始请求
type ChunkUploadStart struct {
	ChunkUploadFile
	ChunkCount int  `json:"chunk_count" validate:"required && min:1"` // 分块总数
	ChunkSize  int  `json:"chunk_size" validate:"required && min:1"`  // 分块大小（字节）
	Force      bool `json:"force"`                                    // 是否覆盖已存在文件
}

// ChunkUpload 上传分块请求
type ChunkUpload struct {
	ChunkUploadFile
	ChunkIndex int                   `form:"chunk_index" validate:"min:0"` // 分块下标
	ChunkHash  string                `form:"chunk_hash"`                   // 分块SHA256，为空则不校验
	File       *multipart.FileHeader `form:"file" validate:"required"`     // 分块数据
}

// ChunkUploadFinish 分块上传完成请求
type ChunkUploadFinish struct {
	ChunkUploadFile
	ChunkCount int  `json:"chunk_count" validate:"required && min:1"` // 分块总数
	Force      bool `json:"force"`                                    // 是否覆盖已存在文件
}
