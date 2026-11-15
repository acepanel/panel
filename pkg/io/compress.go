package io

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/samber/lo"

	"github.com/acepanel/panel/v3/pkg/shell"
)

type ArchiveFormat string

const (
	Zip      ArchiveFormat = "zip"
	SevenZip ArchiveFormat = "7z"
	Tar      ArchiveFormat = "tar"
	TGz      ArchiveFormat = "tgz"
	TBz2     ArchiveFormat = "tbz2"
	TXz      ArchiveFormat = "txz"
	TZst     ArchiveFormat = "tzst"
	Gz       ArchiveFormat = "gz"
	Bz2      ArchiveFormat = "bz2"
	Xz       ArchiveFormat = "xz"
	Zst      ArchiveFormat = "zst"
)

// Compress src 为空时压缩整个 dir
func Compress(ctx context.Context, dir string, src []string, dst string) error {
	cmd, err := CompressShell(dir, src, dst, false)
	if err != nil {
		return err
	}
	_, err = shell.Exec(ctx, cmd)
	return err
}

func UnCompress(ctx context.Context, src, dst string) error {
	cmd, err := UnCompressShell(src, dst, false)
	if err != nil {
		return err
	}
	_, err = shell.Exec(ctx, cmd)
	return err
}

// CompressShell verbose 时逐文件输出，供后台任务日志查看进度
func CompressShell(dir string, src []string, dst string, verbose bool) (string, error) {
	if !filepath.IsAbs(dir) || !filepath.IsAbs(dst) {
		return "", errors.New("dir and dst must be absolute path")
	}
	format, err := formatArchiveByPath(dst)
	if err != nil {
		return "", err
	}
	if len(src) == 0 {
		src = []string{"."}
	}
	target := shell.Quote(dst)
	sources := strings.Join(lo.Map(src, func(s string, _ int) string { return shell.Quote(s) }), " ")

	var cmd string
	switch format {
	case Zip:
		cmd = fmt.Sprintf("zip -%sr %s -- %s", lo.Ternary(verbose, "", "q"), target, sources)
	case SevenZip:
		cmd = fmt.Sprintf("{ 7z a -y%s %s -- %s || [ $? -eq 1 ]; }", lo.Ternary(verbose, " -bb1", ""), target, sources)
	case Tar, TGz, TBz2, TXz, TZst:
		cmd = fmt.Sprintf(
			"{ tar -c%s %s -f %s -- %s || [ $? -eq 1 ]; }",
			lo.Ternary(verbose, " -v", ""),
			tarFilter(format),
			target,
			sources,
		)
	case Gz, Bz2, Xz, Zst:
		// 单文件压缩格式仅支持压缩单个文件
		if len(src) != 1 {
			return "", fmt.Errorf("%s format only supports compressing a single file", format)
		}
		cmd = fmt.Sprintf("%s%s -c -- %s > %s", compressor(format), lo.Ternary(verbose, " -v", ""), sources, target)
	default:
		return "", errors.New("unsupported format")
	}

	return fmt.Sprintf(
		"mkdir -p %s && cd %s && rm -f %s && %s",
		shell.Quote(filepath.Dir(dst)),
		shell.Quote(dir),
		target,
		cmd,
	), nil
}

func UnCompressShell(src, dst string, verbose bool) (string, error) {
	if !filepath.IsAbs(src) || !filepath.IsAbs(dst) {
		return "", errors.New("src and dst must be absolute path")
	}
	format, err := formatArchiveByPath(src)
	if err != nil {
		return "", err
	}
	source, target := shell.Quote(src), shell.Quote(dst)

	var cmd string
	switch format {
	case Zip, SevenZip:
		cmd = fmt.Sprintf("7z x -y%s -snld %s -o%s", lo.Ternary(verbose, " -bb1", ""), source, target)
	case Tar, TGz, TBz2, TXz, TZst:
		cmd = fmt.Sprintf("tar -x%s %s -f %s -C %s", lo.Ternary(verbose, " -v", ""), tarFilter(format), source, target)
	case Gz, Bz2, Xz, Zst:
		name := strings.TrimSuffix(filepath.Base(src), filepath.Ext(src))
		cmd = fmt.Sprintf(
			"%s%s -dc -- %s > %s",
			compressor(format),
			lo.Ternary(verbose, " -v", ""),
			source,
			shell.Quote(filepath.Join(dst, name)),
		)
	default:
		return "", errors.New("unsupported format")
	}

	return fmt.Sprintf("mkdir -p %s && %s", target, cmd), nil
}

func tarFilter(format ArchiveFormat) string {
	switch format {
	case TGz:
		return "-z"
	case TBz2:
		return "-j"
	case TXz:
		return "-J"
	case TZst:
		return "--zstd"
	default:
		return ""
	}
}

func compressor(format ArchiveFormat) string {
	switch format {
	case Gz:
		return "gzip"
	case Bz2:
		return "bzip2"
	case Xz:
		return "xz"
	case Zst:
		return "zstd"
	default:
		return ""
	}
}

func formatArchiveByPath(path string) (ArchiveFormat, error) {
	name := strings.ToLower(filepath.Base(path))
	switch {
	case strings.HasSuffix(name, ".tar.gz"), strings.HasSuffix(name, ".tgz"):
		return TGz, nil
	case strings.HasSuffix(name, ".tar.bz2"), strings.HasSuffix(name, ".tbz2"):
		return TBz2, nil
	case strings.HasSuffix(name, ".tar.xz"), strings.HasSuffix(name, ".txz"):
		return TXz, nil
	case strings.HasSuffix(name, ".tar.zst"), strings.HasSuffix(name, ".tzst"):
		return TZst, nil
	}
	switch filepath.Ext(name) {
	case ".zip":
		return Zip, nil
	case ".7z":
		return SevenZip, nil
	case ".tar":
		return Tar, nil
	case ".gz":
		return Gz, nil
	case ".bz2":
		return Bz2, nil
	case ".xz":
		return Xz, nil
	case ".zst":
		return Zst, nil
	}

	return "", errors.New("unknown format")
}
