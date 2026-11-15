package io

import (
	"context"
	"os"
	"strings"

	"github.com/acepanel/panel/v3/pkg/shell"
)

type Entry struct {
	Path string
	Info os.FileInfo
}

// Search 文件名子串匹配，sub 为 true 时递归
func Search(ctx context.Context, path, keyword string, sub bool) ([]Entry, error) {
	depth := "-maxdepth 1"
	if sub {
		depth = ""
	}
	out, err := shell.Execf(
		ctx,
		"find %s -mindepth 1 %s -name %s -print0",
		shell.Quote(path),
		depth,
		shell.Quote("*"+globEscape(keyword)+"*"),
	)
	if err != nil {
		return nil, err
	}

	var entries []Entry
	for p := range strings.SplitSeq(out, "\x00") {
		if p == "" {
			continue
		}
		info, err := os.Lstat(p)
		if err != nil {
			continue
		}
		entries = append(entries, Entry{Path: p, Info: info})
	}
	return entries, nil
}

func globEscape(s string) string {
	return strings.NewReplacer(`\`, `\\`, `*`, `\*`, `?`, `\?`, `[`, `\[`).Replace(s)
}
