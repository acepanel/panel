package io

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/acepanel/panel/v3/pkg/chattr"
	"github.com/acepanel/panel/v3/pkg/shell"
)

type applyFunc func(path string, typ fs.FileMode) error

type lockedFile struct {
	path  string
	attrs uint32
}

// Remove 撞上 +i/+a 属性时解锁子树重试
func Remove(path string) error {
	err := os.RemoveAll(path)
	if !errors.Is(err, fs.ErrPermission) {
		return err
	}
	unlockTree(path)
	return os.RemoveAll(path)
}

// Chmod 只改 path 自身，符号链接跟随到目标；mode 是 chmod 命令的原始八进制值
func Chmod(path string, mode os.FileMode) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	return applyEntry(path, info.Mode().Type(), chmodFn(mode))
}

// ChmodR 同 chmod -R：起点的符号链接跟随，遍历中遇到的跳过
func ChmodR(ctx context.Context, path string, mode os.FileMode) error {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}
	return walkApply(ctx, path, chmodFn(mode))
}

// Chown 只改 path 自身，owner/group 可为名称或数字 id
func Chown(path, owner, group string) error {
	fn, err := chownFn(owner, group)
	if err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	return applyEntry(path, info.Mode().Type(), fn)
}

func ChownR(ctx context.Context, path, owner, group string) error {
	fn, err := chownFn(owner, group)
	if err != nil {
		return err
	}
	return walkApply(ctx, path, fn)
}

// Exists 判断路径是否存在
func Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// Empty 判断路径是否为空
func Empty(path string) bool {
	files, err := os.ReadDir(path)
	if err != nil {
		return true
	}

	return len(files) == 0
}

func IsDir(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// Mv 目标是最终落点而非父目录，已存在的目录会被合并
func Mv(ctx context.Context, src, dst string) error {
	if err := rename(src, dst); err == nil {
		return nil
	}
	if isRealDir(src) && isRealDir(dst) && !strings.HasPrefix(src, dst+"/") && !strings.HasPrefix(dst, src+"/") {
		return mergeDir(ctx, src, dst)
	}
	_, err := shell.Execf(ctx, "mv -fT %s %s", shell.Quote(src), shell.Quote(dst))
	return err
}

// Cp 语义同 Mv
func Cp(ctx context.Context, src, dst string) error {
	_, err := shell.Execf(ctx, "cp -a --remove-destination -T %s %s", shell.Quote(src), shell.Quote(dst))
	return err
}

// Size 用 du 统计，硬链接只计一次
func Size(ctx context.Context, path string) (int64, error) {
	out, err := shell.Execf(ctx, "du -sb %s", shell.Quote(path))
	if err != nil {
		return 0, err
	}

	parts := strings.Fields(out)
	if len(parts) == 0 {
		return 0, fmt.Errorf("failed to parse du output: %s", out)
	}

	return strconv.ParseInt(parts[0], 10, 64)
}

// applyEntry 被 +i/+a 属性挡住时解锁重试并恢复
func applyEntry(path string, typ fs.FileMode, fn applyFunc) error {
	err := fn(path, typ)
	if !errors.Is(err, fs.ErrPermission) {
		return err
	}
	lf, ok := unlockEntry(path)
	if !ok {
		return err
	}
	defer relockAttr([]lockedFile{lf})
	return fn(path, typ)
}

func chmodFn(mode os.FileMode) applyFunc {
	return func(path string, typ fs.FileMode) error {
		if typ&os.ModeSymlink != 0 {
			return nil
		}
		if err := syscall.Chmod(path, uint32(mode)); err != nil {
			return &os.PathError{Op: "chmod", Path: path, Err: err}
		}
		return nil
	}
}

func walkApply(ctx context.Context, path string, fn applyFunc) error {
	return filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err = ctx.Err(); err != nil {
			return err
		}
		return applyEntry(p, d.Type(), fn)
	})
}

func chownFn(owner, group string) (applyFunc, error) {
	uid, err := lookupID(owner, func(name string) (string, error) {
		u, err := user.Lookup(name)
		if err != nil {
			return "", err
		}
		return u.Uid, nil
	})
	if err != nil {
		return nil, err
	}
	gid, err := lookupID(group, func(name string) (string, error) {
		g, err := user.LookupGroup(name)
		if err != nil {
			return "", err
		}
		return g.Gid, nil
	})
	if err != nil {
		return nil, err
	}
	return func(path string, _ fs.FileMode) error {
		return os.Lchown(path, uid, gid)
	}, nil
}

func lookupID(name string, lookup func(string) (string, error)) (int, error) {
	if id, err := strconv.Atoi(name); err == nil {
		return id, nil
	}
	id, err := lookup(name)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(id)
}

// rename 源或目标带 +i/+a（如 .user.ini）时 EPERM，解锁重试后把属性落到新路径
func rename(src, dst string) error {
	err := os.Rename(src, dst)
	if !errors.Is(err, fs.ErrPermission) {
		return err
	}
	var locked []lockedFile
	for _, p := range []string{src, dst} {
		if lf, ok := unlockEntry(p); ok {
			locked = append(locked, lf)
		}
	}
	if len(locked) == 0 {
		return err
	}
	if err = os.Rename(src, dst); err != nil {
		relockAttr(locked)
		return err
	}
	var attrs uint32
	for _, lf := range locked {
		attrs |= lf.attrs
	}
	relockAttr([]lockedFile{{path: dst, attrs: attrs}})
	return nil
}

// isRealDir 不跟随符号链接的 IsDir
func isRealDir(path string) bool {
	info, err := os.Lstat(path)
	return err == nil && info.IsDir()
}

// mergeDir 同名目录递归合并，其余条目直接覆盖
func mergeDir(ctx context.Context, src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if err = ctx.Err(); err != nil {
			return err
		}
		if err = Mv(ctx, filepath.Join(src, e.Name()), filepath.Join(dst, e.Name())); err != nil {
			return err
		}
	}
	return os.Remove(src)
}

func unlockTree(path string) []lockedFile {
	var locked []lockedFile
	_ = filepath.WalkDir(path, func(p string, _ fs.DirEntry, err error) error {
		if err != nil {
			return nil //nolint:nilerr
		}
		if lf, ok := unlockEntry(p); ok {
			locked = append(locked, lf)
		}
		return nil
	})
	return locked
}

func unlockEntry(path string) (lockedFile, bool) {
	info, err := os.Lstat(path)
	if err != nil || (!info.Mode().IsRegular() && !info.IsDir()) {
		return lockedFile{}, false
	}
	file, err := os.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		return lockedFile{}, false
	}
	defer func() { _ = file.Close() }()
	attrs, err := chattr.GetAttrs(file)
	if err != nil {
		return lockedFile{}, false
	}
	attrs &= chattr.FS_IMMUTABLE_FL | chattr.FS_APPEND_FL
	if attrs == 0 || chattr.UnsetAttr(file, attrs) != nil {
		return lockedFile{}, false
	}
	return lockedFile{path: path, attrs: attrs}, true
}

func relockAttr(files []lockedFile) {
	for _, f := range files {
		file, err := os.OpenFile(f.path, os.O_RDONLY, 0)
		if err != nil {
			continue
		}
		_ = chattr.SetAttr(file, f.attrs)
		_ = file.Close()
	}
}
