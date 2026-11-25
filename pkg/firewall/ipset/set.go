// Package ipset 解析 IP 列表，并下发面板独占的 nftables 过滤表
package ipset

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net/netip"
	"slices"
	"strings"
)

// Range 一段连续的同族地址，From 不大于 To
type Range struct {
	From netip.Addr
	To   netip.Addr
}

// IsAll 是否覆盖整个 IPv4 或 IPv6 地址空间
func (r Range) IsAll() bool {
	return !r.From.Prev().IsValid() && !r.To.Next().IsValid()
}

// String 单个地址写成地址本身，其余写成 起-止，nft 和 Parse 都认这两种写法
func (r Range) String() string {
	if r.From == r.To {
		return r.From.String()
	}
	return r.From.String() + "-" + r.To.String()
}

// Set 一组互不重叠、互不相邻的地址段，IPv4 与 IPv6 分开，各自按起始地址升序
type Set struct {
	v4 []Range
	v6 []Range
}

// Parse 从文本列表读出集合
// 每行可写地址、CIDR 或 起-止 地址段（支持短横线两侧带空格），注释、超长行和认不出的行跳过
func Parse(r io.Reader) (Set, error) {
	var ranges []Range
	br := bufio.NewReader(r)
	for {
		line, isPrefix, err := br.ReadLine()
		if isPrefix {
			// 跳过超过缓冲区长度的行（如压缩成一行的报错页面），合法 IP 行远小于 4KB
			for isPrefix && err == nil {
				_, isPrefix, err = br.ReadLine()
			}
		} else if len(line) > 0 {
			if rg, ok := parseLine(string(line)); ok {
				ranges = append(ranges, rg)
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return Set{}, err
		}
	}

	return newSet(ranges), nil
}

// Of 由若干地址、CIDR 或地址段组成集合，认不出的跳过
func Of(entries ...string) Set {
	ranges := make([]Range, 0, len(entries))
	for _, entry := range entries {
		if rg, ok := parseRange(entry); ok {
			ranges = append(ranges, rg)
		}
	}

	return newSet(ranges)
}

// Union 合并多个集合
func Union(sets ...Set) Set {
	var ranges []Range
	for _, set := range sets {
		ranges = append(ranges, set.v4...)
		ranges = append(ranges, set.v6...)
	}

	return newSet(ranges)
}

// Len 合并后的地址段数
func (s Set) Len() int {
	return len(s.v4) + len(s.v6)
}

// WriteTo 每行写一个地址段，写出的内容可以交给 Parse 读回
func (s Set) WriteTo(w io.Writer) (int64, error) {
	bw := bufio.NewWriter(w)
	var n int64
	for _, rg := range slices.Concat(s.v4, s.v6) {
		written, err := fmt.Fprintln(bw, rg)
		n += int64(written)
		if err != nil {
			return n, err
		}
	}

	return n, bw.Flush()
}

// newSet 按地址族分开后排序，并合并重叠或首尾相接的地址段，丢弃覆盖全地址空间的段
func newSet(ranges []Range) Set {
	slices.SortFunc(ranges, func(a, b Range) int {
		return a.From.Compare(b.From)
	})

	var set Set
	for _, rg := range ranges {
		if rg.IsAll() {
			continue
		}
		family := &set.v6
		if rg.From.Is4() {
			family = &set.v4
		}
		if n := len(*family); n > 0 {
			last := &(*family)[n-1]
			// 上一段已到地址空间末尾时 Next 无效，后面的段必然落在它里面
			if next := last.To.Next(); !next.IsValid() || rg.From.Compare(next) <= 0 {
				if rg.To.Compare(last.To) > 0 {
					last.To = rg.To
				}
				continue
			}
		}
		*family = append(*family, rg)
	}

	set.v4 = slices.DeleteFunc(set.v4, Range.IsAll)
	set.v6 = slices.DeleteFunc(set.v6, Range.IsAll)
	return set
}

// parseLine 从单行文本提取地址、CIDR 或 起-止 地址段
func parseLine(line string) (Range, bool) {
	line = strings.TrimPrefix(line, "\uFEFF")
	if i := strings.IndexAny(line, "#;,"); i >= 0 {
		line = line[:i]
	}
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return Range{}, false
	}
	// 兼容 1.2.3.4 - 1.2.3.10 写法；若短横线右侧不是地址（如 1.2.3.4 - note）则回退按首字段解析
	if len(fields) >= 3 && fields[1] == "-" {
		if rg, ok := parseRange(fields[0] + "-" + fields[2]); ok {
			return rg, true
		}
	}
	return parseRange(fields[0])
}

// parseRange 解析地址、CIDR 或 起-止 形式的地址段，拒绝覆盖整个地址空间的条目
func parseRange(s string) (Range, bool) {
	if from, to, ok := strings.Cut(s, "-"); ok {
		start, err := netip.ParseAddr(from)
		if err != nil {
			return Range{}, false
		}
		end, err := netip.ParseAddr(to)
		if err != nil {
			return Range{}, false
		}
		if start.Zone() != "" || end.Zone() != "" {
			return Range{}, false
		}
		// ::ffff:0.0.0.0-::ffff:255.255.255.255 对应 IPv6 保留段 ::ffff:0:0/96，不能还原成全部 IPv4
		if !isFullMappedRange(start, end) {
			start, end = start.Unmap(), end.Unmap()
		}
		rg := Range{From: start, To: end}
		return rg, rg.From.Is4() == rg.To.Is4() && rg.From.Compare(rg.To) <= 0 && !rg.IsAll()
	}

	if strings.Contains(s, "/") {
		prefix, err := netip.ParsePrefix(s)
		if err != nil {
			return Range{}, false
		}
		// ::ffff:1.2.3.0/120 这类映射子网还原成 IPv4 前缀；::ffff:0:0/96 是常见 IPv6 bogon，保留为 IPv6
		if addr := prefix.Addr(); addr.Is4In6() && prefix.Bits() > 96 {
			prefix = netip.PrefixFrom(addr.Unmap(), prefix.Bits()-96)
		}
		prefix = prefix.Masked()
		// 把主机位全部置 1 得到末尾地址
		last := prefix.Addr().AsSlice()
		for i := prefix.Bits(); i < len(last)*8; i++ {
			last[i/8] |= 1 << (7 - i%8)
		}
		to, _ := netip.AddrFromSlice(last)
		rg := Range{From: prefix.Addr(), To: to}
		return rg, !rg.IsAll()
	}

	addr, err := netip.ParseAddr(s)
	if err != nil || addr.Zone() != "" {
		return Range{}, false
	}
	addr = addr.Unmap()
	return Range{From: addr, To: addr}, true
}

func isFullMappedRange(start, end netip.Addr) bool {
	if !start.Is4In6() || !end.Is4In6() {
		return false
	}
	return !start.Unmap().Prev().IsValid() && !end.Unmap().Next().IsValid()
}
