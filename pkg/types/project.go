package types

type ProjectType string

const (
	ProjectTypeGeneral ProjectType = "general"
	ProjectTypePHP     ProjectType = "php"
	ProjectTypeJava    ProjectType = "java"
	ProjectTypeGo      ProjectType = "go"
	ProjectTypePython  ProjectType = "python"
	ProjectTypeNodejs  ProjectType = "nodejs"
)

type ProjectDetail struct {
	ID              uint        `json:"id"`                // 项目 ID
	Name            string      `json:"name"`              // 项目名称
	Type            ProjectType `json:"type"`              // 项目类型
	Description     string      `json:"description"`       // 项目描述
	RootDir         string      `json:"root_dir"`          // 项目路径
	WorkingDir      string      `json:"working_dir"`       // 运行目录
	ExecStartPre    string      `json:"exec_start_pre"`    // 启动前命令
	ExecStartPost   string      `json:"exec_start_post"`   // 启动后命令
	ExecStart       string      `json:"exec_start"`        // 启动命令
	ExecStop        string      `json:"exec_stop"`         // 停止命令
	ExecReload      string      `json:"exec_reload"`       // 重载命令
	User            string      `json:"user"`              // 运行用户
	Restart         string      `json:"restart"`           // 重启策略
	RestartSec      string      `json:"restart_sec"`       // 重启间隔
	RestartMax      int         `json:"restart_max"`       // 最大重启次数
	TimeoutStartSec int         `json:"timeout_start_sec"` // 启动超时（秒）
	TimeoutStopSec  int         `json:"timeout_stop_sec"`  // 停止超时（秒）
	Environments    []KV        `json:"environments"`      // 环境变量
	StandardOutput  string      `json:"standard_output"`   // 标准输出 journal/file:/path
	StandardError   string      `json:"standard_error"`    // 标准错误 journal/file:/path
	Requires        []string    `json:"requires"`          // 依赖服务（强依赖）
	Wants           []string    `json:"wants"`             // 依赖服务（弱依赖）
	After           []string    `json:"after"`             // 启动顺序（在...之后启动）
	Before          []string    `json:"before"`            // 启动顺序（在...之前启动）

	// 运行状态
	Status      string  `json:"status"`       // 运行状态
	Enabled     bool    `json:"enabled"`      // 是否自启动
	PID         int     `json:"pid"`          // 进程ID
	Memory      int64   `json:"memory"`       // 内存使用（字节）
	CPU         float64 `json:"cpu"`          // CPU使用率
	Uptime      string  `json:"uptime"`       // 运行时间
	MemoryLimit float64 `json:"memory_limit"` // 内存限制（字节）
	CPUQuota    float64 `json:"cpu_quota"`    // CPU限制（百分比）

	// 安全相关
	NoNewPrivileges bool     `json:"no_new_privileges"` // 无新特权
	ProtectTmp      bool     `json:"protect_tmp"`       // 保护临时目录
	ProtectHome     bool     `json:"protect_home"`      // 保护主目录
	ProtectSystem   string   `json:"protect_system"`    // 保护系统 full/strict
	ReadWritePaths  []string `json:"read_write_paths"`  // 读写路径
	ReadOnlyPaths   []string `json:"read_only_paths"`   // 只读路径
}
