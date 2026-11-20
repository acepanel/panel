package types

import (
	"time"
)

// MigrationStep 迁移流程所处步骤
type MigrationStep string

const (
	MigrationStepIdle     MigrationStep = "idle"     // 空闲
	MigrationStepPreCheck MigrationStep = "precheck" // 已连接来源
	MigrationStepSelect   MigrationStep = "select"   // 选择迁移项
	MigrationStepRunning  MigrationStep = "running"  // 迁移中
	MigrationStepDone     MigrationStep = "done"     // 迁移结束
)

// MigrationStatus 单个迁移项的状态
type MigrationStatus string

const (
	MigrationPending MigrationStatus = "pending"
	MigrationRunning MigrationStatus = "running"
	MigrationSuccess MigrationStatus = "success"
	MigrationPartial MigrationStatus = "partial" // 迁移完成但有告警
	MigrationFailed  MigrationStatus = "failed"
	MigrationSkipped MigrationStatus = "skipped"
)

// MigrationStage 单个迁移项的当前阶段
type MigrationStage string

const (
	MigrationStageBackup   MigrationStage = "backup"   // 在来源侧生成备份
	MigrationStageTransfer MigrationStage = "transfer" // 传输备份
	MigrationStageImport   MigrationStage = "import"   // 导入到目标
	MigrationStageDone     MigrationStage = "done"
)

// MigrationProgress 传输进度回调
type MigrationProgress func(transferred, total int64)

// MigrationSource 来源面板信息
type MigrationSource struct {
	Panel   string `json:"panel"`
	Version string `json:"version"`
}

// MigrationItem 来源面板上的一个可迁移资源
type MigrationItem struct {
	Key       string   `json:"key"`      // 前端选择用的唯一标识
	Type      string   `json:"type"`     // website / database / database_user / project
	Subtype   string   `json:"subtype"`  // 网站为 static/php/proxy，数据库为 mysql/postgresql 等
	Name      string   `json:"name"`     // 来源名称
	Status    string   `json:"status"`   // running / stopped
	Size      int64    `json:"size"`     // 占用空间，0 为未知
	Blockers  []string `json:"blockers"` // 非空则该项无法迁移
	Warnings  []string `json:"warnings"` // 迁移前提示
	DependsOn []string `json:"depends_on"`

	TargetName string `json:"target_name"`
	TargetPath string `json:"target_path"`
	TargetUser string `json:"-"` // 项目运行用户，由前端指定

	// 以下字段仅服务端内部使用
	SourceID    string `json:"-"` // 来源主键
	SourcePath  string `json:"-"` // 来源目录
	SourceGroup string `json:"-"` // 来源分组：项目或网站所属的模块名 / 数据库服务名
	Version     string `json:"-"` // 运行时版本
}

// MigrationDetail 迁移执行前重新读取的资源详情
type MigrationDetail struct {
	Item         MigrationItem
	Website      *MigrationWebsite
	Database     *MigrationDatabase
	DatabaseUser *MigrationDatabaseUser
	Project      *MigrationProject
}

// MigrationWebsite 网站详情
type MigrationWebsite struct {
	Type         string // static / php / proxy
	Path         string // 来源网站根目录
	Root         string // 运行目录
	Domains      []string
	Listens      []string
	SSLListens   []string
	Index        []string
	PHP          uint
	Rewrite      string
	OpenBasedir  bool
	Remark       string
	ExpireAt     *time.Time
	Enabled      bool
	SSL          bool
	SSLCert      string
	SSLKey       string
	SSLProtocols []string
	HSTS         bool
	OCSP         bool
	HTTPRedirect bool
	Proxies      []MigrationProxy
	Redirects    []MigrationRedirect
}

// MigrationProxy 网站反向代理规则
type MigrationProxy struct {
	Location string
	Pass     string
	Host     string
	Replaces map[string]string
}

// MigrationRedirect 网站重定向规则
type MigrationRedirect struct {
	Type       string // host / url
	From       string
	To         string
	KeepURI    bool
	StatusCode int
}

// MigrationDatabase 数据库详情
type MigrationDatabase struct {
	Type     string // mysql / postgresql
	Version  string
	Name     string
	Username string
	Password string
	Host     string
}

// MigrationDatabaseUser 数据库用户详情
type MigrationDatabaseUser struct {
	Type     string // mysql / postgresql
	Username string
	Password string
	Host     string
}

// MigrationProject 项目详情
type MigrationProject struct {
	Type         ProjectType
	Version      string
	Path         string
	WorkingDir   string
	ExecStart    string
	User         string
	Port         uint
	Domains      []string
	Listens      []string
	Environments []KV
	Enabled      bool
	Running      bool
}

// MigrationResult 单个迁移项的执行结果
type MigrationResult struct {
	Key       string          `json:"key"`
	Type      string          `json:"type"`
	Name      string          `json:"name"`
	Status    MigrationStatus `json:"status"`
	Stage     MigrationStage  `json:"stage"`
	Error     string          `json:"error"`
	Warnings  []string        `json:"warnings"`
	StartedAt *time.Time      `json:"started_at"`
	EndedAt   *time.Time      `json:"ended_at"`
	Duration  float64         `json:"duration"` // 耗时（秒）
}
