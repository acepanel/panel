package service

import (
	"sync"
	"time"

	"github.com/acepanel/panel/v3/pkg/clientip"
)

const (
	// bruteforceWindow 失败计数的统计窗口
	bruteforceWindow = 10 * time.Minute
	// bruteforceThreshold 窗口内触发告警的失败次数
	bruteforceThreshold = 5
	// bruteforceSilence 同一来源两次告警的最小间隔
	bruteforceSilence = 30 * time.Minute
	// bruteforceMaxEntries 计数条目上限，防伪造来源耗尽内存
	bruteforceMaxEntries = 10000
)

type loginFailEntry struct {
	count    uint
	since    time.Time
	notified time.Time
}

// loginGuard 按来源 IP 统计登录失败，用于验证码判定与爆破告警
type loginGuard struct {
	mu      sync.Mutex
	entries map[string]*loginFailEntry
}

func newLoginGuard() *loginGuard {
	return &loginGuard{entries: make(map[string]*loginFailEntry)}
}

// Fail 记录一次失败，返回窗口内失败次数与是否应当告警
func (g *loginGuard) Fail(ip string) (uint, bool) {
	ip = clientip.Bucket(ip)
	now := time.Now()

	g.mu.Lock()
	defer g.mu.Unlock()

	g.sweep(now)

	entry, ok := g.entries[ip]
	if !ok {
		if len(g.entries) >= bruteforceMaxEntries {
			return 1, false
		}
		g.entries[ip] = &loginFailEntry{count: 1, since: now}
		return 1, false
	}

	// 超出窗口重新计数
	if now.Sub(entry.since) > bruteforceWindow {
		entry.count, entry.since = 1, now
		return 1, false
	}

	entry.count++
	if entry.count < bruteforceThreshold {
		return entry.count, false
	}
	if !entry.notified.IsZero() && now.Sub(entry.notified) < bruteforceSilence {
		return entry.count, false
	}
	entry.notified = now

	return entry.count, true
}

// Count 返回该来源在窗口内的失败次数
func (g *loginGuard) Count(ip string) uint {
	g.mu.Lock()
	defer g.mu.Unlock()

	entry, ok := g.entries[clientip.Bucket(ip)]
	if !ok || time.Since(entry.since) > bruteforceWindow {
		return 0
	}

	return entry.count
}

// Reset 登录成功后清除该来源的失败计数
func (g *loginGuard) Reset(ip string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.entries, clientip.Bucket(ip))
}

// sweep 清理超出窗口且未在静默期内的条目，调用方需持有锁
func (g *loginGuard) sweep(now time.Time) {
	for ip, entry := range g.entries {
		if now.Sub(entry.since) > bruteforceWindow && now.Sub(entry.notified) > bruteforceSilence {
			delete(g.entries, ip)
		}
	}
}
