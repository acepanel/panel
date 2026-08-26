package route

import (
	"net/http"

	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/internal/service"
)

// FileRoutes 文件管理路由
func FileRoutes(fileService *service.FileService) Endpoints {
	file := fileService

	return Endpoints{
		{Method: http.MethodPost, Path: "/api/file/create", Handler: file.Create,
			Summary: "创建文件或目录", Tags: []string{"文件"},
			Document: DescribeReq[request.FileCreate]()},
		{Method: http.MethodGet, Path: "/api/file/content", Handler: file.Content,
			Summary: "读取文件内容", Tags: []string{"文件"},
			Document: DescribeReq[request.FilePath]()},
		{Method: http.MethodGet, Path: "/api/file/tail", Handler: file.Tail,
			Summary: "反向分页读取日志", Tags: []string{"文件"},
			Document: DescribeReq[request.FileTail]()},
		{Method: http.MethodPost, Path: "/api/file/save", Handler: file.Save,
			Summary: "保存文件", Tags: []string{"文件"},
			Document: DescribeReq[request.FileSave]()},
		{Method: http.MethodPost, Path: "/api/file/truncate", Handler: file.Truncate,
			Summary: "截断文件", Tags: []string{"文件"},
			Document: DescribeReq[request.FilePath]()},
		{Method: http.MethodPost, Path: "/api/file/delete", Handler: file.Delete,
			Summary: "删除文件", Tags: []string{"文件"},
			Document: DescribeReq[request.FilePath]()},
		{Method: http.MethodPost, Path: "/api/file/upload", Handler: file.Upload,
			Summary: "上传文件", Tags: []string{"文件"}},
		{Method: http.MethodPost, Path: "/api/file/exist", Handler: file.Exist,
			Summary: "批量检查文件是否存在", Tags: []string{"文件"}},
		{Method: http.MethodPost, Path: "/api/file/move", Handler: file.Move,
			Summary: "移动文件", Tags: []string{"文件"}},
		{Method: http.MethodPost, Path: "/api/file/copy", Handler: file.Copy,
			Summary: "复制文件", Tags: []string{"文件"}},
		{Method: http.MethodGet, Path: "/api/file/download", Handler: file.Download,
			Summary: "下载文件", Tags: []string{"文件"},
			Document: DescribeReq[request.FilePath]()},
		{Method: http.MethodPost, Path: "/api/file/remote_download", Handler: file.RemoteDownload,
			Summary: "远程下载文件", Tags: []string{"文件"},
			Document: DescribeReq[request.FileRemoteDownload]()},
		{Method: http.MethodGet, Path: "/api/file/info", Handler: file.Info,
			Summary: "获取文件信息", Tags: []string{"文件"},
			Document: DescribeReq[request.FilePath]()},
		{Method: http.MethodGet, Path: "/api/file/size", Handler: file.Size,
			Summary: "计算文件或目录大小", Tags: []string{"文件"},
			Document: DescribeReq[request.FilePath]()},
		{Method: http.MethodPost, Path: "/api/file/permission", Handler: file.Permission,
			Summary: "设置文件权限", Tags: []string{"文件"},
			Document: DescribeReq[request.FilePermission]()},
		{Method: http.MethodPost, Path: "/api/file/compress", Handler: file.Compress,
			Summary: "压缩文件", Tags: []string{"文件"},
			Document: DescribeReq[request.FileCompress]()},
		{Method: http.MethodPost, Path: "/api/file/un_compress", Handler: file.UnCompress,
			Summary: "解压文件", Tags: []string{"文件"},
			Document: DescribeReq[request.FileUnCompress]()},
		{Method: http.MethodGet, Path: "/api/file/list", Handler: file.List,
			Summary: "文件列表", Tags: []string{"文件"},
			Document: DescribeReq[request.FileList]()},
		{Method: http.MethodPost, Path: "/api/file/chunk/start", Handler: file.ChunkUploadStart,
			Summary: "开始分块上传", Tags: []string{"文件"},
			Document: DescribeReq[request.ChunkUploadStart]()},
		{Method: http.MethodPost, Path: "/api/file/chunk/upload", Handler: file.ChunkUploadChunk,
			Summary: "上传分块", Tags: []string{"文件"}},
		{Method: http.MethodPost, Path: "/api/file/chunk/finish", Handler: file.ChunkUploadFinish,
			Summary: "完成分块上传", Tags: []string{"文件"},
			Document: DescribeReq[request.ChunkUploadFinish]()},
	}
}
