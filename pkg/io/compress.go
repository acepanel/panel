package io

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/acepanel/panel/v3/pkg/shell"
)

type FormatArchive string

const (
	Zip      FormatArchive = "zip"
	Gz       FormatArchive = "gz"
	Bz2      FormatArchive = "bz2"
	Tar      FormatArchive = "tar"
	TGz      FormatArchive = "tgz"
	TXz      FormatArchive = "txz"
	TBz2     FormatArchive = "tbz2"
	TZst     FormatArchive = "tzst"
	Xz       FormatArchive = "xz"
	SevenZip FormatArchive = "7z"
	Zst      FormatArchive = "zst"
)

// Compress 压缩文件
func Compress(dir string, src []string, dst string) error {
	if !filepath.IsAbs(dir) || !filepath.IsAbs(dst) {
		return errors.New("dir and dst must be absolute path")
	}
	if len(src) == 0 {
		src = append(src, ".")
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	format, err := formatArchiveByPath(dst)
	if err != nil {
		return err
	}

	switch format {
	case Zip:
		_, err = shell.ExecfWithDir(dir, "zip -qr -o %s %s", dst, strings.Join(src, " "))
	case Gz, Bz2, Xz, Zst:
		// 单文件压缩格式仅支持压缩单个文件
		if len(src) != 1 {
			return fmt.Errorf("%s format only supports compressing a single file", format)
		}
		_, err = shell.ExecfWithDir(dir, "%s -c %s > %s", compressorByFormat(format), src[0], dst)
	case TGz:
		_, err = shell.ExecfWithDir(dir, "tar -czf %s %s", dst, strings.Join(src, " "))
	case TBz2:
		_, err = shell.ExecfWithDir(dir, "tar -cjf %s %s", dst, strings.Join(src, " "))
	case Tar:
		_, err = shell.ExecfWithDir(dir, "tar -cf %s %s", dst, strings.Join(src, " "))
	case TXz:
		_, err = shell.ExecfWithDir(dir, "tar -cJf %s %s", dst, strings.Join(src, " "))
	case SevenZip:
		_, err = shell.ExecfWithDir(dir, "7z a -y %s %s", dst, strings.Join(src, " "))
	case TZst:
		_, err = shell.ExecfWithDir(dir, "tar --zstd -cf %s %s", dst, strings.Join(src, " "))
	default:
		return errors.New("unsupported format")
	}

	return err
}

// UnCompress 解压文件
func UnCompress(src string, dst string) error {
	if !filepath.IsAbs(src) || !filepath.IsAbs(dst) {
		return errors.New("src and dst must be absolute path")
	}
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}

	format, err := formatArchiveByPath(src)
	if err != nil {
		return err
	}

	switch format {
	case Zip:
		// 用 7z 解压 zip,自动检测文件名编码,避免中文文件名变成 #Uxxxx
		_, err = shell.Execf("7z x -y '%s' -o'%s'", src, dst)
	case Gz, Bz2, Xz, Zst:
		// 单独压缩的文件（如 .sql.gz），解压到目标目录
		baseName := strings.TrimSuffix(filepath.Base(src), filepath.Ext(src))
		_, err = shell.Execf("%s -dc '%s' > '%s'", compressorByFormat(format), src, filepath.Join(dst, baseName))
	case TGz:
		_, err = shell.Execf("tar -xzf '%s' -C '%s'", src, dst)
	case TBz2:
		_, err = shell.Execf("tar -xjf '%s' -C '%s'", src, dst)
	case Tar:
		_, err = shell.Execf("tar -xf '%s' -C '%s'", src, dst)
	case TXz:
		_, err = shell.Execf("tar -xJf '%s' -C '%s'", src, dst)
	case SevenZip:
		_, err = shell.Execf("7z x -y '%s' -o'%s'", src, dst)
	case TZst:
		_, err = shell.Execf("tar --zstd -xf '%s' -C '%s'", src, dst)
	default:
		return errors.New("unsupported format")
	}

	return err
}

// CompressShell 生成压缩命令的 shell 字符串, 便于作为后台任务执行
func CompressShell(dir string, src []string, dst string) (string, error) {
	if !filepath.IsAbs(dir) || !filepath.IsAbs(dst) {
		return "", errors.New("dir and dst must be absolute path")
	}
	if len(src) == 0 {
		src = append(src, ".")
	}

	format, err := formatArchiveByPath(dst)
	if err != nil {
		return "", err
	}

	sources := strings.Join(src, " ")
	var cmd string
	switch format {
	case Zip:
		cmd = fmt.Sprintf("zip -qr -o '%s' %s", dst, sources)
	case Gz, Bz2, Xz, Zst:
		// 单文件压缩格式仅支持压缩单个文件
		if len(src) != 1 {
			return "", fmt.Errorf("%s format only supports compressing a single file", format)
		}
		cmd = fmt.Sprintf("%s -c %s > '%s'", compressorByFormat(format), sources, dst)
	case TGz:
		cmd = fmt.Sprintf("tar -czf '%s' %s", dst, sources)
	case TBz2:
		cmd = fmt.Sprintf("tar -cjf '%s' %s", dst, sources)
	case Tar:
		cmd = fmt.Sprintf("tar -cf '%s' %s", dst, sources)
	case TXz:
		cmd = fmt.Sprintf("tar -cJf '%s' %s", dst, sources)
	case SevenZip:
		cmd = fmt.Sprintf("7z a -y '%s' %s", dst, sources)
	case TZst:
		cmd = fmt.Sprintf("tar --zstd -cf '%s' %s", dst, sources)
	default:
		return "", errors.New("unsupported format")
	}

	return fmt.Sprintf("mkdir -p '%s' && cd '%s' && %s", filepath.Dir(dst), dir, cmd), nil
}

// UnCompressShell 生成解压命令的 shell 字符串, 便于作为后台任务执行
func UnCompressShell(src, dst string) (string, error) {
	if !filepath.IsAbs(src) || !filepath.IsAbs(dst) {
		return "", errors.New("src and dst must be absolute path")
	}

	format, err := formatArchiveByPath(src)
	if err != nil {
		return "", err
	}

	var cmd string
	switch format {
	case Zip:
		cmd = fmt.Sprintf("7z x -y '%s' -o'%s'", src, dst)
	case Gz:
		baseName := strings.TrimSuffix(filepath.Base(src), ".gz")
		cmd = fmt.Sprintf("gunzip -c '%s' > '%s'", src, filepath.Join(dst, baseName))
	case TGz:
		cmd = fmt.Sprintf("tar -xzf '%s' -C '%s'", src, dst)
	case TBz2:
		cmd = fmt.Sprintf("tar -xjf '%s' -C '%s'", src, dst)
	case Tar:
		cmd = fmt.Sprintf("tar -xf '%s' -C '%s'", src, dst)
	case TXz:
		cmd = fmt.Sprintf("tar -xJf '%s' -C '%s'", src, dst)
	case Xz:
		baseName := strings.TrimSuffix(filepath.Base(src), ".xz")
		cmd = fmt.Sprintf("xz -dc '%s' > '%s'", src, filepath.Join(dst, baseName))
	case Bz2:
		baseName := strings.TrimSuffix(filepath.Base(src), ".bz2")
		cmd = fmt.Sprintf("bzip2 -dc '%s' > '%s'", src, filepath.Join(dst, baseName))
	case SevenZip:
		cmd = fmt.Sprintf("7z x -y '%s' -o'%s'", src, dst)
	case TZst:
		cmd = fmt.Sprintf("tar --zstd -xf '%s' -C '%s'", src, dst)
	case Zst:
		baseName := strings.TrimSuffix(filepath.Base(src), ".zst")
		cmd = fmt.Sprintf("zstd -dc '%s' > '%s'", src, filepath.Join(dst, baseName))
	default:
		return "", errors.New("unsupported format")
	}

	// 部分归档条目失败不应阻断整个任务
	return fmt.Sprintf("mkdir -p '%s' && (%s || true)", dst, cmd), nil
}

// ListCompress 获取压缩包内文件列表
func ListCompress(src string) ([]string, error) {
	format, err := formatArchiveByPath(src)
	if err != nil {
		return nil, err
	}

	var out string
	switch format {
	case Zip, SevenZip:
		out, err = shell.Execf(`7z l -ba -slt '%s' | grep "^Path = " | sed 's/^Path = //'`, src)
	case Gz, Xz, Bz2, Zst:
		// 单独压缩的文件只包含一个文件，返回去除压缩后缀的文件名
		baseName := strings.TrimSuffix(filepath.Base(src), filepath.Ext(src))
		return []string{baseName}, nil
	case TGz, TBz2, Tar, TXz:
		out, err = shell.Execf("tar -tf '%s'", src)
	case TZst:
		out, err = shell.Execf("tar --zstd -tf '%s'", src)
	default:
		return nil, errors.New("unsupported format")
	}
	if err != nil {
		return nil, err
	}

	return strings.Split(out, "\n"), nil
}

// compressorByFormat 单文件压缩格式对应的压缩工具，压缩用 -c，解压用 -dc
func compressorByFormat(format FormatArchive) string {
	switch format {
	case Gz:
		return "gzip"
	case Bz2:
		return "bzip2"
	case Xz:
		return "xz"
	case Zst:
		return "zstd"
	}
	return ""
}

// formatArchiveByPath 根据文件后缀获取压缩格式
func formatArchiveByPath(path string) (FormatArchive, error) {
	switch filepath.Ext(path) {
	case ".zip":
		return Zip, nil
	case ".bz2":
		// 支持 .tar.bz2 和单独的 .bz2 格式（如 .sql.bz2）
		if strings.HasSuffix(path, ".tar.bz2") {
			return TBz2, nil
		}
		return Bz2, nil
	case ".tar":
		return Tar, nil
	case ".tgz":
		return TGz, nil
	case ".gz":
		// 支持 .tar.gz 和单独的 .gz 格式（如 .sql.gz）
		if strings.HasSuffix(path, ".tar.gz") {
			return TGz, nil
		}
		return Gz, nil
	case ".xz":
		// 支持 .tar.xz 和单独的 .xz 格式（如 .sql.xz）
		if strings.HasSuffix(path, ".tar.xz") {
			return TXz, nil
		}
		return Xz, nil
	case ".7z":
		return SevenZip, nil
	case ".zst":
		// 支持 .tar.zst 和单独的 .zst 格式（如 .sql.zst）
		if strings.HasSuffix(path, ".tar.zst") {
			return TZst, nil
		}
		return Zst, nil
	}

	return "", errors.New("unknown format")
}
