package data

import (
	"archive/tar"
	"context"
	"errors"
	"fmt"
	stdio "io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/samber/lo"

	"github.com/acepanel/panel/v3/pkg/io"
)

// selectWebsiteBackup 逐层展开归档，从不同面板导出的目录结构中定位网站文件根目录。
func (r *backupRepo) selectWebsiteBackup(ctx context.Context, root string) (string, error) {
	current, onePanel := root, false
	for range 4 {
		entries, err := os.ReadDir(current)
		if err != nil {
			return "", err
		}
		// 1Panel 的备份是「元数据 + 内层归档」，website.json 是它的标记
		if lo.SomeBy(entries, func(entry os.DirEntry) bool { return entry.Name() == "website.json" }) {
			onePanel = true
		}

		archives := lo.Filter(entries, func(entry os.DirEntry, _ int) bool {
			return !entry.IsDir() && r.databaseArchiveExt(filepath.Join(current, entry.Name())) != ""
		})
		// PHP 站点的包里还有一份运行环境归档，网站文件在 .web 那个里面
		if onePanel && len(archives) > 1 {
			archives = lo.Filter(archives, func(entry os.DirEntry, _ int) bool {
				return strings.Contains(entry.Name(), ".web.")
			})
		}

		// 仅元数据旁或目录中只此一个文件时才当外层包装，否则可能是网站自带的压缩包
		wrapped := len(archives) == 1 && (onePanel || len(entries) == 1)
		if wrapped {
			stage, err := os.MkdirTemp(root, ".ace-nested-*")
			if err != nil {
				return "", err
			}
			if err = io.UnCompress(ctx, filepath.Join(current, archives[0].Name()), stage); err != nil {
				return "", err
			}
			current = stage
			continue
		}

		// 归档常带一层同名目录
		if len(entries) == 1 && entries[0].IsDir() {
			current = filepath.Join(current, entries[0].Name())
			continue
		}

		// 1Panel 站点目录下的 index 才是文件根，同级的 log、ssl 属于面板附属目录
		if index := filepath.Join(current, "index"); onePanel && io.IsDir(index) {
			return index, nil
		}

		return current, nil
	}

	return current, nil
}

// prepareDatabaseBackup 自动展开常见压缩格式，并从不同软件生成的目录结构中定位数据库备份。
func (r *backupRepo) prepareDatabaseBackup(ctx context.Context, backup, target string) (string, string, error) {
	if r.databaseArchiveExt(backup) == "" {
		return backup, "", nil
	}

	temp, err := r.tmpDir()
	if err != nil {
		return "", "", err
	}
	fail := func(err error) (string, string, error) {
		_ = os.RemoveAll(temp)
		return "", "", err
	}

	current := backup
	for depth := range 4 {
		ext := r.databaseArchiveExt(current)
		if ext == "" {
			return current, temp, nil
		}

		stage := filepath.Join(temp, fmt.Sprintf("stage-%d", depth))
		if err = os.MkdirAll(stage, 0755); err != nil {
			return fail(err)
		}
		source := filepath.Join(temp, fmt.Sprintf("archive-%d%s", depth, ext))
		if err = os.Symlink(current, source); err != nil {
			return fail(err)
		}
		if err = io.UnCompress(ctx, source, stage); err != nil {
			return fail(err)
		}
		current, err = r.selectDatabaseBackup(stage, target)
		if err != nil {
			return fail(err)
		}
	}

	return fail(errors.New(r.t.Get("database backup contains too many nested archives")))
}

func (r *backupRepo) selectDatabaseBackup(root, target string) (string, error) {
	files := make([]string, 0)
	var directoryDump string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if entry.Name() == "__MACOSX" {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Name() == "toc.dat" {
			directoryDump = filepath.Dir(path)
		}
		if entry.Type().IsRegular() && entry.Name() != ".DS_Store" {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if directoryDump != "" {
		return directoryDump, nil
	}
	if len(files) == 1 {
		return files[0], nil
	}

	candidates := make([]string, 0, len(files))
	for _, file := range files {
		lower := strings.ToLower(file)
		if strings.HasSuffix(lower, ".sql") || strings.HasSuffix(lower, ".dump") || strings.HasSuffix(lower, ".backup") || strings.HasSuffix(lower, ".bak") || r.databaseArchiveExt(file) != "" || r.postgresArchive(file) {
			candidates = append(candidates, file)
		}
	}
	if len(candidates) == 0 {
		return "", errors.New(r.t.Get("could not find database backup file"))
	}
	if len(candidates) == 1 {
		return candidates[0], nil
	}

	lowerTarget := strings.ToLower(target)
	matches := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		name := strings.ToLower(filepath.Base(candidate))
		if name == lowerTarget || strings.HasPrefix(name, lowerTarget+".") || strings.HasPrefix(name, lowerTarget+"_") || strings.HasPrefix(name, lowerTarget+"-") {
			matches = append(matches, candidate)
		}
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	if len(matches) > 1 {
		candidates = matches
	}
	for _, candidate := range candidates {
		if r.databaseArchiveExt(candidate) != "" || r.postgresArchive(candidate) {
			return "", errors.New(r.t.Get("database backup archive contains multiple dump files"))
		}
	}

	slices.Sort(candidates)
	combined := filepath.Join(root, "combined.sql")
	output, err := os.Create(combined)
	if err != nil {
		return "", err
	}
	for _, candidate := range candidates {
		input, openErr := os.Open(candidate)
		if openErr != nil {
			_ = output.Close()
			return "", openErr
		}
		_, copyErr := stdio.Copy(output, input)
		_ = input.Close()
		if copyErr != nil {
			_ = output.Close()
			return "", copyErr
		}
		if _, err = output.WriteString("\n"); err != nil {
			_ = output.Close()
			return "", err
		}
	}
	if err = output.Close(); err != nil {
		return "", err
	}
	return combined, nil
}

func (r *backupRepo) databaseArchiveExt(path string) string {
	file, err := os.Open(path)
	if err == nil {
		defer func() { _ = file.Close() }()
		header := make([]byte, 512)
		n, _ := file.Read(header)
		header = header[:n]
		switch {
		case len(header) >= 2 && header[0] == 0x1f && header[1] == 0x8b:
			return ".gz"
		case len(header) >= 4 && string(header[:4]) == "PK\x03\x04":
			return ".zip"
		case len(header) >= 6 && string(header[:6]) == "7z\xbc\xaf'\x1c":
			return ".7z"
		case len(header) >= 7 && string(header[:7]) == "Rar!\x1a\x07":
			return ".7z"
		case len(header) >= 3 && string(header[:3]) == "BZh":
			return ".bz2"
		case len(header) >= 6 && string(header[:6]) == "\xfd7zXZ\x00":
			return ".xz"
		case len(header) >= 4 && header[0] == 0x28 && header[1] == 0xb5 && header[2] == 0x2f && header[3] == 0xfd:
			return ".zst"
		case len(header) >= 262 && string(header[257:262]) == "ustar":
			return ".tar"
		}
	}
	for _, suffix := range []string{".tar.gz", ".tar.bz2", ".tar.xz", ".tar.zst", ".tgz", ".tbz2", ".txz", ".zip", ".7z", ".tar", ".gz", ".bz2", ".xz", ".zst"} {
		if strings.HasSuffix(strings.ToLower(path), suffix) {
			return filepath.Ext(path)
		}
	}
	return ""
}

func (r *backupRepo) postgresArchive(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	if info.IsDir() {
		_, err = os.Stat(filepath.Join(path, "toc.dat"))
		return err == nil
	}

	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer func() { _ = file.Close() }()
	header := make([]byte, 5)
	if n, _ := file.Read(header); n == len(header) && string(header) == "PGDMP" {
		return true
	}
	if _, err = file.Seek(0, 0); err != nil {
		return false
	}
	reader := tar.NewReader(file)
	for range 8 {
		entry, readErr := reader.Next()
		if readErr != nil {
			return false
		}
		if filepath.Base(entry.Name) == "toc.dat" {
			return true
		}
		if entry.Size > 0 {
			return false
		}
	}
	return false
}
