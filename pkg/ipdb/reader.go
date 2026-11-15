package ipdb

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"strings"
	"sync"
)

var (
	ErrInvalidFile = errors.New("ipdb: invalid database file")
	ErrClosed      = errors.New("ipdb: reader is closed")
	ErrNotFound    = errors.New("ipdb: record not found")
	ErrInvalidIP   = errors.New("ipdb: invalid IP address")
	ErrNoLanguage  = errors.New("ipdb: unsupported language")
)

// Meta IPDB 元数据
type Meta struct {
	Build     int64          `json:"build"`
	IPVersion uint16         `json:"ip_version"`
	Languages map[string]int `json:"languages"`
	NodeCount int            `json:"node_count"`
	TotalSize int            `json:"total_size"`
	Fields    []string       `json:"fields"`
}

// Reader IPDB 读取器
type Reader struct {
	mu        sync.RWMutex
	mmap      *mmapFile
	body      []byte // 节点树 + 数据记录区
	meta      Meta
	nodeCount int
	v4offset  int
	fieldLen  int
	closed    bool
}

// Open 打开 IPDB 文件
func Open(path string) (*Reader, error) {
	m, err := mmapOpen(path)
	if err != nil {
		return nil, err
	}

	r := &Reader{}
	if err = r.load(m); err != nil {
		_ = m.Close()
		return nil, err
	}
	return r, nil
}

// Find 查询 IP 的记录
func (r *Reader) Find(ip, language string) ([]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.closed {
		return nil, ErrClosed
	}

	langOff, ok := r.meta.Languages[language]
	if !ok {
		return nil, ErrNoLanguage
	}

	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return nil, ErrInvalidIP
	}

	var rawIP []byte
	var bitCount int
	if addr.Is4() {
		ip4 := addr.As4()
		rawIP = ip4[:]
		bitCount = 32
	} else {
		ip6 := addr.As16()
		rawIP = ip6[:]
		bitCount = 128
	}

	node, err := r.search(rawIP, bitCount)
	if err != nil {
		return nil, err
	}

	record, err := r.resolve(node)
	if err != nil {
		return nil, err
	}

	fields := strings.Split(record, "\t")
	start := langOff
	end := start + r.fieldLen
	if end > len(fields) {
		return nil, ErrInvalidFile
	}

	// 拷贝切片，不持有原始 record 引用
	result := make([]string, r.fieldLen)
	copy(result, fields[start:end])
	return result, nil
}

// Fields 返回字段名列表
func (r *Reader) Fields() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.meta.Fields
}

// Reload 加载新文件替换当前数据
func (r *Reader) Reload(path string) error {
	m, err := mmapOpen(path)
	if err != nil {
		return err
	}

	// 先加载验证新文件
	tmp := &Reader{}
	if err = tmp.load(m); err != nil {
		_ = m.Close()
		return err
	}

	r.mu.Lock()
	old := r.mmap
	r.mmap = tmp.mmap
	r.body = tmp.body
	r.meta = tmp.meta
	r.nodeCount = tmp.nodeCount
	r.v4offset = tmp.v4offset
	r.fieldLen = tmp.fieldLen
	r.mu.Unlock()

	// 关闭旧映射
	if old != nil {
		_ = old.Close()
	}
	return nil
}

// Close 释放资源
func (r *Reader) Close() error {
	if r == nil {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		return nil
	}
	r.closed = true
	r.body = nil

	if r.mmap != nil {
		return r.mmap.Close()
	}
	return nil
}

// load 解析 mmap 数据
func (r *Reader) load(m *mmapFile) error {
	data := m.data
	if len(data) < 4 {
		return ErrInvalidFile
	}

	metaLen := int(binary.BigEndian.Uint32(data[:4]))
	if len(data) < 4+metaLen {
		return ErrInvalidFile
	}

	var meta Meta
	if err := json.Unmarshal(data[4:4+metaLen], &meta); err != nil {
		return fmt.Errorf("ipdb: failed to parse metadata: %w", err)
	}
	if meta.NodeCount <= 0 || len(meta.Fields) == 0 {
		return ErrInvalidFile
	}

	body := data[4+metaLen:]
	nodeSize := meta.NodeCount * 8
	if len(body) < nodeSize {
		return ErrInvalidFile
	}

	r.mmap = m
	r.body = body
	r.meta = meta
	r.nodeCount = meta.NodeCount
	r.fieldLen = len(meta.Fields)

	// 计算 IPv4 在 IPv6 trie 中的偏移
	r.v4offset = r.calcV4Offset()

	return nil
}

// calcV4Offset 在 IPv6 trie 中找到 IPv4 的起始节点
// IPv4 映射地址: 80 个 0 bit + 16 个 1 bit + 32 bit IPv4
func (r *Reader) calcV4Offset() int {
	node := 0
	for i := 0; i < 96 && node < r.nodeCount; i++ {
		if i >= 80 {
			node = r.readNode(node, 1)
		} else {
			node = r.readNode(node, 0)
		}
	}
	return node
}

// readNode 读取节点的子节点索引
func (r *Reader) readNode(node, bit int) int {
	off := node*8 + bit*4
	return int(binary.BigEndian.Uint32(r.body[off : off+4]))
}

// search 在 trie 中搜索 IP
func (r *Reader) search(ip []byte, bitCount int) (int, error) {
	node := 0
	if bitCount == 32 {
		node = r.v4offset
	}

	for i := range bitCount {
		if node > r.nodeCount {
			break
		}
		bit := int(ip[i>>3]>>(7-uint(i&7))) & 1
		node = r.readNode(node, bit)
	}

	if node <= r.nodeCount {
		return 0, ErrNotFound
	}

	return node, nil
}

// resolve 从数据区提取记录
func (r *Reader) resolve(node int) (string, error) {
	off := (node - r.nodeCount) + r.nodeCount*8
	if off+2 > len(r.body) {
		return "", ErrInvalidFile
	}

	size := int(binary.BigEndian.Uint16(r.body[off : off+2]))
	off += 2
	if off+size > len(r.body) {
		return "", ErrInvalidFile
	}

	// 拷贝到独立字符串，不持有 mmap 引用
	return string(r.body[off : off+size]), nil
}
