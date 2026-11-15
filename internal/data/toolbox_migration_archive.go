package data

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/pkg/io"
	"github.com/acepanel/panel/v3/pkg/shell"
)

type migrationArchiveRepo struct{}

func NewMigrationArchiveRepo() biz.MigrationArchiveRepo {
	return &migrationArchiveRepo{}
}

// TempDir 在面板目录下创建临时目录，避免大文件撑爆 /tmp（常为内存盘）
func (r *migrationArchiveRepo) TempDir() (string, error) {
	base := filepath.Join(app.Root, "tmp")
	if err := os.MkdirAll(base, 0755); err != nil {
		return "", err
	}
	return os.MkdirTemp(base, "migration-*")
}

// Extract 解包归档，返回真实内容根目录（归档常带一层同名目录）
func (r *migrationArchiveRepo) Extract(ctx context.Context, archive, target string) (string, error) {
	if err := os.MkdirAll(target, 0755); err != nil {
		return "", err
	}
	untar := shell.Command(ctx, "tar", "-xf", archive, "-C", target)
	if output, err := untar.CombinedOutput(); err != nil {
		return "", fmt.Errorf("extract archive: %s", strings.TrimSpace(string(output)))
	}
	entries, err := os.ReadDir(target)
	if err != nil || len(entries) != 1 || !entries[0].IsDir() {
		return target, nil //nolint:nilerr
	}
	return filepath.Join(target, entries[0].Name()), nil
}

// Compress 打包目录为 tar.gz
func (r *migrationArchiveRepo) Compress(ctx context.Context, source, archive string) error {
	if err := os.MkdirAll(filepath.Dir(archive), 0755); err != nil {
		return err
	}
	tar := shell.Command(ctx, "tar", "-czf", archive, "-C", source, ".")
	output, err := tar.CombinedOutput()
	if err != nil {
		return fmt.Errorf("compress directory: %s", strings.TrimSpace(string(output)))
	}
	return nil
}

// CopyTree 复制目录内容，保留权限与属主
func (r *migrationArchiveRepo) CopyTree(ctx context.Context, source, target string) error {
	if err := os.MkdirAll(target, 0755); err != nil {
		return err
	}
	return io.Cp(ctx, source, target)
}

// IsEmpty 判断目录为空或不存在
func (r *migrationArchiveRepo) IsEmpty(path string) bool {
	return io.Empty(path)
}
