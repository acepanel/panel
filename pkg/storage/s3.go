package storage

import (
	"context"
	"io"
	"strings"

	"github.com/samber/lo"

	"github.com/acepanel/panel/v3/pkg/storage/s3sdk"
)

// S3AddressingStyle S3 地址模式
type S3AddressingStyle string

const (
	// S3AddressingStylePath Path 模式：https://s3.region.amazonaws.com/bucket/key
	S3AddressingStylePath S3AddressingStyle = "path"
	// S3AddressingStyleVirtualHosted Virtual Hosted 模式：https://bucket.s3.region.amazonaws.com/key
	S3AddressingStyleVirtualHosted S3AddressingStyle = "virtual-hosted"
)

type S3Config struct {
	Region          string            // AWS 区域
	Bucket          string            // S3 存储桶名称
	AccessKey       string            // 访问密钥 ID
	SecretKey       string            // 访问密钥
	Endpoint        string            // 自定义端点
	Scheme          string            // 协议 http 或 https
	BasePath        string            // 基础路径前缀
	AddressingStyle S3AddressingStyle // 地址模式
}

type s3Storage struct {
	client *s3sdk.S3
	config S3Config
}

func NewS3(cfg S3Config) Storage {
	cfg.BasePath = strings.Trim(cfg.BasePath, "/")

	client := s3sdk.New(s3sdk.Config{
		Region:      cfg.Region,
		Bucket:      cfg.Bucket,
		AccessKey:   cfg.AccessKey,
		SecretKey:   cfg.SecretKey,
		Endpoint:    cfg.Endpoint,
		Scheme:      cfg.Scheme,
		PathStyle:   cfg.AddressingStyle == S3AddressingStylePath,
		Concurrency: 5,
	})

	return &s3Storage{client: client, config: cfg}
}

// Delete 删除文件
func (s *s3Storage) Delete(ctx context.Context, files ...string) error {
	if len(files) == 0 {
		return nil
	}
	keys := lo.Map(files, func(file string, _ int) string {
		return joinPath(s.config.BasePath, file)
	})
	return s.client.Delete(ctx, keys...)
}

// List 列出目录下的所有文件
func (s *s3Storage) List(ctx context.Context, path string) ([]File, error) {
	prefix := joinPath(s.config.BasePath, path)
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}

	var files []File
	for obj, err := range s.client.List(ctx, prefix, "/") {
		if err != nil {
			return nil, err
		}
		// 提取文件名，跳过子目录
		name := strings.TrimPrefix(obj.Key, prefix)
		if name != "" && !strings.Contains(name, "/") {
			files = append(files, File{Name: name, Size: obj.Size, ModTime: obj.LastModified})
		}
	}

	return files, nil
}

// Put 写入文件内容
func (s *s3Storage) Put(ctx context.Context, file string, content io.Reader) error {
	return s.client.Put(ctx, joinPath(s.config.BasePath, file), content, "application/octet-stream")
}
