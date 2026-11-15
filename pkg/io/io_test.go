package io

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/libtnb/assert/check"
	"github.com/libtnb/assert/must"
)

// archiveExts 归档格式，支持多文件
var archiveExts = []string{".zip", ".tar", ".tar.gz", ".tgz", ".tar.bz2", ".tar.xz", ".tar.zst", ".7z"}

// singleExts 只能压缩单个文件的格式
var singleExts = []string{".gz", ".bz2", ".xz", ".zst"}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func writeTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for name, data := range files {
		path := filepath.Join(root, name)
		must.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
		must.NoError(t, Write(path, data, 0644))
	}
}

func checkTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for name, want := range files {
		got, err := Read(filepath.Join(root, name))
		check.NoError(t, err)
		check.Equal(t, got, want, check.Msgf("文件内容不符: %s", name))
	}
}

func TestWriteCreatesFileWithCorrectContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "write_test.txt")
	data := "Hello, World!"
	permission := os.FileMode(0644)

	must.NoError(t, Write(path, data, permission))

	content, err := Read(path)
	check.NoError(t, err)
	check.Equal(t, content, data)
}

func TestWriteCreatesTraversableParentDir(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a", "b", "c.txt")
	must.NoError(t, Write(path, "x", 0600))

	info, err := os.Stat(filepath.Dir(path))
	must.NoError(t, err)
	check.True(t, info.Mode().Perm()&0o111 != 0, check.Msgf("父目录应可进入，实际 %o", info.Mode().Perm()))
}

func TestCompress(t *testing.T) {
	abs := t.TempDir()
	src := []string{"compress_test1.txt", "compress_test2.txt"}
	must.NoError(t, Write(filepath.Join(abs, src[0]), "File 1", 0644))
	must.NoError(t, Write(filepath.Join(abs, src[1]), "File 2", 0644))

	for _, ext := range archiveExts {
		t.Run(ext, func(t *testing.T) {
			check.NoError(t, Compress(t.Context(), abs, src, filepath.Join(abs, "compress_test"+ext)))
		})
	}
	for _, ext := range singleExts {
		t.Run(ext, func(t *testing.T) {
			check.NoError(t, Compress(t.Context(), abs, src[:1], filepath.Join(abs, "compress_single"+ext)))
			// 单文件格式装不下多个文件
			check.Error(t, Compress(t.Context(), abs, src, filepath.Join(abs, "compress_multi"+ext)))
		})
	}
}

func TestUnCompress(t *testing.T) {
	abs := t.TempDir()
	src := []string{"uncompress_test1.txt", "uncompress_test2.txt"}
	must.NoError(t, Write(filepath.Join(abs, src[0]), "File 1", 0644))
	must.NoError(t, Write(filepath.Join(abs, src[1]), "File 2", 0644))

	for _, ext := range archiveExts {
		t.Run(ext, func(t *testing.T) {
			dst := filepath.Join(abs, "uncompressed"+strings.ReplaceAll(ext, ".", "_"))
			must.NoError(t, Compress(t.Context(), abs, src, filepath.Join(abs, "uncompress_test"+ext)))
			must.NoError(t, UnCompress(t.Context(), filepath.Join(abs, "uncompress_test"+ext), dst))

			data, err := Read(filepath.Join(dst, src[0]))
			check.NoError(t, err)
			check.Equal(t, data, "File 1")
			data, err = Read(filepath.Join(dst, src[1]))
			check.NoError(t, err)
			check.Equal(t, data, "File 2")
		})
	}
	// 单文件压缩格式解压后去掉压缩后缀恢复原文件名
	for _, ext := range singleExts {
		t.Run(ext, func(t *testing.T) {
			dst := filepath.Join(abs, "uncompressed_single"+strings.ReplaceAll(ext, ".", "_"))
			must.NoError(t, Compress(t.Context(), abs, src[:1], filepath.Join(abs, src[0]+ext)))
			must.NoError(t, UnCompress(t.Context(), filepath.Join(abs, src[0]+ext), dst))

			data, err := Read(filepath.Join(dst, src[0]))
			check.NoError(t, err)
			check.Equal(t, data, "File 1")
		})
	}
}

func TestCompressQuotesNames(t *testing.T) {
	abs := t.TempDir()
	names := []string{"my file.txt", "it's.txt", "-dash.txt", "a$b.txt", "中文 文件.txt"}
	for _, name := range names {
		must.NoError(t, Write(filepath.Join(abs, name), name, 0644))
	}

	for _, ext := range archiveExts {
		t.Run(ext, func(t *testing.T) {
			archive := filepath.Join(abs, "it's quoted"+ext)
			must.NoError(t, Compress(t.Context(), abs, names, archive))
			dst := filepath.Join(abs, "out "+strings.ReplaceAll(ext, ".", "_"))
			must.NoError(t, UnCompress(t.Context(), archive, dst))
			for _, name := range names {
				data, err := Read(filepath.Join(dst, name))
				check.NoError(t, err)
				check.Equal(t, data, name)
			}
		})
	}
}

func TestCompressOverwritesExistingArchive(t *testing.T) {
	abs := t.TempDir()
	must.NoError(t, Write(filepath.Join(abs, "old.txt"), "old", 0644))
	must.NoError(t, Write(filepath.Join(abs, "new.txt"), "new", 0644))

	for _, ext := range []string{".zip", ".7z"} {
		t.Run(ext, func(t *testing.T) {
			archive := filepath.Join(abs, "overwrite"+ext)
			must.NoError(t, Compress(t.Context(), abs, []string{"old.txt"}, archive))
			must.NoError(t, Compress(t.Context(), abs, []string{"new.txt"}, archive))
			dst := filepath.Join(abs, "overwrite_out"+strings.ReplaceAll(ext, ".", "_"))
			must.NoError(t, UnCompress(t.Context(), archive, dst))
			check.False(t, Exists(filepath.Join(dst, "old.txt")), check.Msgf("旧包里的条目不应残留"))
			check.True(t, Exists(filepath.Join(dst, "new.txt")), check.Msgf("新条目应存在"))
		})
	}
}

func TestCompressShellQuotesArguments(t *testing.T) {
	cmd, err := CompressShell("/data/site dir", []string{"it's.txt", "-dash.txt"}, "/data/out's.tar.gz", false)
	must.NoError(t, err)
	check.Equal(t, cmd, `mkdir -p '/data' && cd '/data/site dir' && rm -f '/data/out'\''s.tar.gz' && { tar -c -z -f '/data/out'\''s.tar.gz' -- 'it'\''s.txt' '-dash.txt' || [ $? -eq 1 ]; }`)

	cmd, err = UnCompressShell("/data/it's.zip", "/data/out dir", false)
	must.NoError(t, err)
	check.Equal(t, cmd, `mkdir -p '/data/out dir' && 7z x -y -snld '/data/it'\''s.zip' -o'/data/out dir'`)
}

func TestCompressShellVerbose(t *testing.T) {
	for dst, want := range map[string]string{
		"/data/out.zip":    "zip -r '/data/out.zip' -- 'a'",
		"/data/out.7z":     "{ 7z a -y -bb1 '/data/out.7z' -- 'a' || [ $? -eq 1 ]; }",
		"/data/out.tar.gz": "{ tar -c -v -z -f '/data/out.tar.gz' -- 'a' || [ $? -eq 1 ]; }",
		"/data/out.gz":     "gzip -v -c -- 'a' > '/data/out.gz'",
	} {
		cmd, err := CompressShell("/data", []string{"a"}, dst, true)
		must.NoError(t, err)
		check.True(t, strings.HasSuffix(cmd, want), check.Msgf("%s", cmd))
	}

	for src, want := range map[string]string{
		"/data/in.zip":    "7z x -y -bb1 -snld '/data/in.zip' -o'/data/out'",
		"/data/in.tar.xz": "tar -x -v -J -f '/data/in.tar.xz' -C '/data/out'",
		"/data/in.zst":    "zstd -v -dc -- '/data/in.zst' > '/data/out/in'",
	} {
		cmd, err := UnCompressShell(src, "/data/out", true)
		must.NoError(t, err)
		check.True(t, strings.HasSuffix(cmd, want), check.Msgf("%s", cmd))
	}
}

func TestCompressFailsWhenDirMissing(t *testing.T) {
	archive := filepath.Join(t.TempDir(), "x.tar.gz")
	check.Error(t, Compress(t.Context(), filepath.Join(t.TempDir(), "missing"), nil, archive))
	check.False(t, Exists(archive), check.Msgf("不应产出归档"))
}

func TestFormatArchiveByPath(t *testing.T) {
	for path, want := range map[string]ArchiveFormat{
		"a.ZIP": Zip, "a.7z": SevenZip, "a.tar": Tar, "a.TAR.GZ": TGz, "a.tgz": TGz,
		"a.tar.bz2": TBz2, "a.tbz2": TBz2, "a.tar.xz": TXz, "a.txz": TXz,
		"a.tar.zst": TZst, "a.tzst": TZst, "dump.sql.gz": Gz, "a.bz2": Bz2, "a.xz": Xz, "a.zst": Zst,
	} {
		got, err := formatArchiveByPath("/x/" + path)
		check.NoError(t, err, check.Msgf("%s", path))
		check.Equal(t, got, want, check.Msgf("%s", path))
	}
	_, err := formatArchiveByPath("/x/a.rar")
	check.Error(t, err)
}

func TestRemoveDeletesFileOrDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "remove_test")
	must.NoError(t, os.MkdirAll(path, 0755))
	must.True(t, dirExists(path), must.Msgf("前置目录未建成: %s", path))

	check.NoError(t, Remove(path))
	check.False(t, dirExists(path), check.Msgf("目录应已删除: %s", path))
}

func TestChmodChangesPermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "chmod_test.txt")
	must.NoError(t, Write(path, "test", 0644))

	check.NoError(t, Chmod(path, 0755))
	info, err := os.Stat(path)
	must.NoError(t, err)
	check.Equal(t, info.Mode().Perm(), os.FileMode(0755))
}

func TestChownChangesOwner(t *testing.T) {
	path := filepath.Join(t.TempDir(), "chown_test.txt")
	must.NoError(t, Write(path, "test", 0644))

	check.NoError(t, Chown(path, "root", "root"))
	check.NoError(t, Chown(path, "0", "0"))
	check.Error(t, Chown(path, "no-such-user-xyz", "root"))
}

func TestExistsReturnsTrueForExistingPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "exists_test.txt")
	must.NoError(t, Write(path, "test", 0644))
	check.True(t, Exists(path), check.Msgf("路径应存在: %s", path))
}

func TestExistsReturnsFalseForNonExistingPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent.txt")
	check.False(t, Exists(path), check.Msgf("路径不应存在: %s", path))
}

func TestEmptyReturnsTrueForEmptyDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty_test")
	must.NoError(t, os.MkdirAll(path, 0755))
	check.True(t, Empty(path), check.Msgf("目录应为空: %s", path))
}

func TestEmptyReturnsFalseForNonEmptyDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonempty_test")
	must.NoError(t, os.MkdirAll(path, 0755))
	must.NoError(t, Write(filepath.Join(path, "file.txt"), "test", 0644))
	check.False(t, Empty(path), check.Msgf("目录不应为空: %s", path))
}

func TestIsDirReturnsTrueForDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "isdir_test")
	must.NoError(t, os.MkdirAll(path, 0755))
	check.True(t, IsDir(path), check.Msgf("应识别为目录: %s", path))
}

func TestIsDirReturnsFalseForFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "isfile_test.txt")
	must.NoError(t, Write(path, "test", 0644))
	check.False(t, IsDir(path), check.Msgf("不应识别为目录: %s", path))
}

func TestMvMovesFile(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "mv_src.txt")
	dst := filepath.Join(dir, "mv_dst.txt")
	must.NoError(t, Write(src, "test", 0644))

	check.NoError(t, Mv(t.Context(), src, dst))
	check.True(t, fileExists(dst), check.Msgf("目标文件应存在: %s", dst))
	check.False(t, fileExists(src), check.Msgf("源文件应已移走: %s", src))
}

func TestMvMovesDirToNewPath(t *testing.T) {
	dir := t.TempDir()
	src, dst := filepath.Join(dir, "src", "app"), filepath.Join(dir, "dst", "app")
	writeTree(t, src, map[string]string{"sub/f": "new"})
	must.NoError(t, os.MkdirAll(filepath.Dir(dst), 0755))

	check.NoError(t, Mv(t.Context(), src, dst))
	check.False(t, dirExists(src), check.Msgf("源目录应已移走: %s", src))
	checkTree(t, dst, map[string]string{"sub/f": "new"})
}

func TestMvMergesIntoExistingDir(t *testing.T) {
	dir := t.TempDir()
	src, dst := filepath.Join(dir, "src", "app"), filepath.Join(dir, "dst", "app")
	writeTree(t, src, map[string]string{"sub/f": "new", "g": "new"})
	writeTree(t, dst, map[string]string{"sub/f": "old", "keep/k": "old"})

	check.NoError(t, Mv(t.Context(), src, dst))
	check.False(t, dirExists(src), check.Msgf("源目录应已移走: %s", src))
	check.False(t, dirExists(filepath.Join(dst, "app")), check.Msgf("不应嵌套进目标目录: %s", dst))
	checkTree(t, dst, map[string]string{"sub/f": "new", "g": "new", "keep/k": "old"})
}

func TestMvRefusesAncestorTarget(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{"a/x": "1", "a/b/x": "2", "a/b/b/x": "3"})

	check.Error(t, Mv(t.Context(), filepath.Join(dir, "a", "b"), filepath.Join(dir, "a")))
	checkTree(t, dir, map[string]string{"a/x": "1", "a/b/x": "2", "a/b/b/x": "3"})
}

func TestCpCopiesFile(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "cp_src.txt")
	dst := filepath.Join(dir, "cp_dst.txt")
	must.NoError(t, Write(src, "test", 0644))

	check.NoError(t, Cp(t.Context(), src, dst))
	check.True(t, fileExists(dst), check.Msgf("目标文件应存在: %s", dst))
	check.True(t, fileExists(src), check.Msgf("源文件应保留: %s", src))
}

func TestCpMergesIntoExistingDir(t *testing.T) {
	dir := t.TempDir()
	src, dst := filepath.Join(dir, "src", "app"), filepath.Join(dir, "dst", "app")
	writeTree(t, src, map[string]string{"sub/f": "new", "g": "new"})
	writeTree(t, dst, map[string]string{"sub/f": "old", "keep/k": "old"})

	check.NoError(t, Cp(t.Context(), src, dst))
	checkTree(t, src, map[string]string{"sub/f": "new", "g": "new"})
	check.False(t, dirExists(filepath.Join(dst, "app")), check.Msgf("不应嵌套进目标目录: %s", dst))
	checkTree(t, dst, map[string]string{"sub/f": "new", "g": "new", "keep/k": "old"})
}

func TestCpReplacesSymlinkInsteadOfTarget(t *testing.T) {
	dir := t.TempDir()
	target, link, src := filepath.Join(dir, "real.txt"), filepath.Join(dir, "link.txt"), filepath.Join(dir, "src.txt")
	must.NoError(t, Write(target, "real", 0644))
	must.NoError(t, Write(src, "new", 0644))
	must.NoError(t, os.Symlink(target, link))

	check.NoError(t, Cp(t.Context(), src, link))
	data, _ := Read(target)
	check.Equal(t, data, "real", check.Msgf("不应顺着链接改写目标文件"))
	info, err := os.Lstat(link)
	must.NoError(t, err)
	check.True(t, info.Mode().IsRegular(), check.Msgf("链接应被替换成普通文件"))
}

func TestSize(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{"a.txt": "12345", "sub/b.txt": "123"})

	size, err := Size(t.Context(), filepath.Join(dir, "a.txt"))
	check.NoError(t, err)
	check.Equal(t, size, int64(5))
}
