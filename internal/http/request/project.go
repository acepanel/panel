package request

import "github.com/acepanel/panel/pkg/types"

type ProjectCreate struct {
	Name         string            `form:"name" json:"name" validate:"required|regex:^[a-zA-Z0-9_-]+$"`
	Type         types.ProjectType `form:"type" json:"type" validate:"required|in:general,php,java,go,python,nodejs"`
	Description  string            `form:"description" json:"description"`
	RootDir      string            `form:"root_dir" json:"root_dir"`
	WorkingDir   string            `form:"working_dir" json:"working_dir"`
	ExecStart    string            `form:"exec_start" json:"exec_start"`
	User         string            `form:"user" json:"user"`
	Restart      string            `json:"restart"`
	Environments []types.KV
}

type ProjectUpdate struct {
	ID              uint       `form:"id" json:"id" validate:"required|exists:projects,id"`
	Name            string     `form:"name" json:"name" validate:"required|regex:^[a-zA-Z0-9_-]+$"`
	Description     string     `form:"description" json:"description"`
	RootDir         string     `form:"root_dir" json:"root_dir" validate:"required"`
	WorkingDir      string     `form:"working_dir" json:"working_dir"`
	ExecStartPre    string     `form:"exec_start_pre" json:"exec_start_pre"`
	ExecStartPost   string     `form:"exec_start_post" json:"exec_start_post"`
	ExecStart       string     `form:"exec_start" json:"exec_start"`
	ExecStop        string     `form:"exec_stop" json:"exec_stop"`
	ExecReload      string     `form:"exec_reload" json:"exec_reload"`
	User            string     `form:"user" json:"user"`
	Restart         string     `json:"restart"`
	RestartSec      string     `json:"restart_sec"`
	RestartMax      int        `json:"restart_max"`
	TimeoutStartSec int        `json:"timeout_start_sec"`
	TimeoutStopSec  int        `json:"timeout_stop_sec"`
	Environments    []types.KV `form:"environments" json:"environments"`
	StandardOutput  string     `form:"standard_output" json:"standard_output"`
	StandardError   string     `form:"standard_error" json:"standard_error"`
	Requires        []string   `form:"requires" json:"requires"`
	Wants           []string   `form:"wants" json:"wants"`
	After           []string   `form:"after" json:"after"`
	Before          []string   `form:"before" json:"before"`

	MemoryLimit float64 `form:"memory_limit" json:"memory_limit"`
	CPUQuota    string  `form:"cpu_quota" json:"cpu_quota"`

	NoNewPrivileges bool     `form:"no_new_privileges" json:"no_new_privileges"`
	ProtectTmp      bool     `form:"protect_tmp" json:"protect_tmp"`
	ProtectHome     bool     `form:"protect_home" json:"protect_home"`
	ProtectSystem   string   `form:"protect_system" json:"protect_system"`
	ReadWritePaths  []string `form:"read_write_paths" json:"read_write_paths"`
	ReadOnlyPaths   []string `form:"read_only_paths" json:"read_only_paths"`
}
