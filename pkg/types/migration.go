package types

import "time"

// MigrationStep 迁移步骤
type MigrationStep string

const (
	MigrationStepIdle     MigrationStep = "idle"     // 空闲
	MigrationStepConnect  MigrationStep = "connect"  // 连接信息
	MigrationStepPreCheck MigrationStep = "precheck" // 预检查
	MigrationStepSelect   MigrationStep = "select"   // 选择迁移项
	MigrationStepRunning  MigrationStep = "running"  // 迁移中
	MigrationStepDone     MigrationStep = "done"     // 迁移完成
)

// MigrationItemStatus 迁移项状态
type MigrationItemStatus string

const (
	MigrationItemPending MigrationItemStatus = "pending"
	MigrationItemRunning MigrationItemStatus = "running"
	MigrationItemSuccess MigrationItemStatus = "success"
	MigrationItemFailed  MigrationItemStatus = "failed"
	MigrationItemSkipped MigrationItemStatus = "skipped"
)

// MigrationItemResult 单个迁移项的结果
type MigrationItemResult struct {
	Type      string              `json:"type"`       // website / database / project
	Name      string              `json:"name"`       // 名称
	Status    MigrationItemStatus `json:"status"`     // 状态
	Error     string              `json:"error"`      // 失败原因
	StartedAt *time.Time          `json:"started_at"` // 开始时间
	EndedAt   *time.Time          `json:"ended_at"`   // 结束时间
	Duration  float64             `json:"duration"`   // 耗时（秒）
}
