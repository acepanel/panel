package app

import (
	"sort"
	"sync"
	"time"
)

const (
	// HealthLevelError 需要用户立即处理
	HealthLevelError = "error"
	// HealthLevelWarning 提示性问题，通常已被系统自动降级处理
	HealthLevelWarning = "warning"
)

// HealthIssue 单条健康问题
// Key 为稳定标识符（如 database:stat），前端据此选择翻译文案
// Message 为原始错误详情，供诊断参考
type HealthIssue struct {
	Key     string    `json:"key"`
	Level   string    `json:"level"`
	Message string    `json:"message"`
	Since   time.Time `json:"since"`
}

type healthRegistry struct {
	mu     sync.RWMutex
	issues map[string]HealthIssue
}

// Health 全局健康状态注册表，供各后台任务上报/清除故障
var Health = &healthRegistry{issues: make(map[string]HealthIssue)}

// Report 上报或更新一条健康问题
// 同 key 重复上报时保留首次上报时间，仅更新 message 和 level
func (h *healthRegistry) Report(key, level, message string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if existing, ok := h.issues[key]; ok {
		existing.Level = level
		existing.Message = message
		h.issues[key] = existing
		return
	}
	h.issues[key] = HealthIssue{
		Key:     key,
		Level:   level,
		Message: message,
		Since:   time.Now(),
	}
}

// Clear 清除指定 key 的健康问题
func (h *healthRegistry) Clear(key string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.issues, key)
}

// Snapshot 返回当前所有健康问题，按 Since 升序（越早发生越靠前）
func (h *healthRegistry) Snapshot() []HealthIssue {
	h.mu.RLock()
	defer h.mu.RUnlock()

	result := make([]HealthIssue, 0, len(h.issues))
	for _, issue := range h.issues {
		result = append(result, issue)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Since.Before(result[j].Since)
	})
	return result
}
