package io

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/acepanel/panel/pkg/shell"
)

type FormatArchive string

const (
	Zip      FormatArchive = "zip"
	Gz       FormatArchive = "gz"
	Bz2      FormatArchive = "bz2"
	Tar      FormatArchive = "tar"
	TGz      FormatArchive = "tgz"
	Xz       FormatArchive = "xz"
	SevenZip FormatArchive = "7z"
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
	case TGz:
		_, err = shell.ExecfWithDir(dir, "tar -czf %s %s", dst, strings.Join(src, " "))
	case Bz2:
		_, err = shell.ExecfWithDir(dir, "tar -cjf %s %s", dst, strings.Join(src, " "))
	case Tar:
		_, err = shell.ExecfWithDir(dir, "tar -cf %s %s", dst, strings.Join(src, " "))
	case Xz:
		_, err = shell.ExecfWithDir(dir, "tar -cJf %s %s", dst, strings.Join(src, " "))
	case SevenZip:
		_, err = shell.ExecfWithDir(dir, "7z a -y %s %s", dst, strings.Join(src, " "))
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
		_, err = shell.Execf("unzip -qo '%s' -d '%s'", src, dst)
	case Gz:
		// 单独的 gzip 文件（如 .sql.gz），解压到目标目录
		// gunzip -k 保留原文件，-c 输出到标准输出，重定向到目标文件
		baseName := strings.TrimSuffix(filepath.Base(src), ".gz")
		_, err = shell.Execf("gunzip -c '%s' > '%s'", src, filepath.Join(dst, baseName))
	case TGz:
		_, err = shell.Execf("tar -xzf '%s' -C '%s'", src, dst)
	case Bz2:
		_, err = shell.Execf("tar -xjf '%s' -C '%s'", src, dst)
	case Tar:
		_, err = shell.Execf("tar -xf '%s' -C '%s'", src, dst)
	case Xz:
		_, err = shell.Execf("tar -xJf '%s' -C '%s'", src, dst)
	case SevenZip:
		_, err = shell.Execf("7z x -y '%s' -o'%s'", src, dst)
	default:
		return errors.New("unsupported format")
	}

	return err
}

// ListCompress 获取压缩包内文件列表
func ListCompress(src string) ([]string, error) {
	format, err := formatArchiveByPath(src)
	if err != nil {
		return nil, err
	}

	var out string
	switch format {
	case Zip:
		out, err = shell.Execf("unzip -Z1 '%s'", src)
	case Gz:
		// 单独的 gzip 文件只包含一个文件，返回去除 .gz 后缀的文件名
		baseName := strings.TrimSuffix(filepath.Base(src), ".gz")
		return []string{baseName}, nil
	case TGz, Bz2, Tar, Xz:
		out, err = shell.Execf("tar -tf '%s'", src)
	case SevenZip:
		out, err = shell.Execf(`7z l -ba -slt '%s' | grep "^Path = " | sed 's/^Path = //'`, src)
	default:
		return nil, errors.New("unsupported format")
	}
	if err != nil {
		return nil, err
	}

	return strings.Split(out, "\n"), nil
}

// formatArchiveByPath 根据文件后缀获取压缩格式
func formatArchiveByPath(path string) (FormatArchive, error) {
	switch filepath.Ext(path) {
	case ".zip":
		return Zip, nil
	case ".bz2":
		// .bz2 后缀的文件使用 tar -xjf 解压，适用于 .tar.bz2 格式
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
		// .xz 后缀的文件使用 tar -xJf 解压，适用于 .tar.xz 格式
		return Xz, nil
	case ".7z":
		return SevenZip, nil
	}

	return "", errors.New("unknown format")
}
