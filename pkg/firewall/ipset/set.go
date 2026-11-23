// Package ipset 解析 IP 列表，并把它们下发成面板独占的 nftables 过滤表
package ipset

import (
	"bufio"
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
// 每行只看首个字段，可以是地址、CIDR 或 起-止 形式的地址段，注释和认不出的行跳过
func Parse(r io.Reader) (Set, error) {
	var ranges []Range
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		field := strings.TrimSpace(scanner.Text())
		// 行内可能带注释（# 或 ;）或额外的列
		if i := strings.IndexAny(field, " \t#;,"); i >= 0 {
			field = field[:i]
		}
		if rg, ok := parseRange(field); ok {
			ranges = append(ranges, rg)
		}
	}
	if err := scanner.Err(); err != nil {
		return Set{}, err
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

// newSet 按地址族分开后排序，并合并重叠或首尾相接的地址段
func newSet(ranges []Range) Set {
	slices.SortFunc(ranges, func(a, b Range) int {
		return a.From.Compare(b.From)
	})

	var set Set
	for _, rg := range ranges {
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

	return set
}

// parseRange 解析地址、CIDR 或 起-止 形式的地址段
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
		rg := Range{From: start.Unmap(), To: end.Unmap()}
		valid := rg.From.Zone() == "" && rg.To.Zone() == "" && rg.From.Is4() == rg.To.Is4() && rg.From.Compare(rg.To) <= 0
		return rg, valid
	}

	if strings.Contains(s, "/") {
		prefix, err := netip.ParsePrefix(s)
		if err != nil {
			return Range{}, false
		}
		// ::ffff:1.2.3.0/120 这类映射地址还原成 IPv4 前缀
		if addr := prefix.Addr(); addr.Is4In6() && prefix.Bits() >= 96 {
			prefix = netip.PrefixFrom(addr.Unmap(), prefix.Bits()-96)
		}
		prefix = prefix.Masked()
		// 把主机位全部置 1 得到末尾地址
		last := prefix.Addr().AsSlice()
		for i := prefix.Bits(); i < len(last)*8; i++ {
			last[i/8] |= 1 << (7 - i%8)
		}
		to, _ := netip.AddrFromSlice(last)
		return Range{From: prefix.Addr(), To: to}, true
	}

	addr, err := netip.ParseAddr(s)
	if err != nil || addr.Zone() != "" {
		return Range{}, false
	}
	addr = addr.Unmap()
	return Range{From: addr, To: addr}, true
}
