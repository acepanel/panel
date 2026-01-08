package request

// ProcessKill 结束进程请求
type ProcessKill struct {
	PID int32 `json:"pid" validate:"required"`
}

// ProcessDetail 获取进程详情请求
type ProcessDetail struct {
	PID int32 `json:"pid" form:"pid" query:"pid" validate:"required"`
}

// ProcessSignal 发送信号请求
// 支持的信号: SIGHUP(1), SIGINT(2), SIGKILL(9), SIGUSR1(10), SIGUSR2(12), SIGTERM(15), SIGCONT(18), SIGSTOP(19)
type ProcessSignal struct {
	PID    int32 `json:"pid" validate:"required"`
	Signal int   `json:"signal" validate:"required|in:1,2,9,10,12,15,18,19"`
}

// ProcessList 进程列表请求
type ProcessList struct {
	Page    uint   `json:"page" form:"page" query:"page"`
	Limit   uint   `json:"limit" form:"limit" query:"limit"`
	Sort    string `json:"sort" form:"sort" query:"sort" validate:"in:pid,name,cpu,rss,start_time,ppid,num_threads"`
	Order   string `json:"order" form:"order" query:"order" validate:"in:asc,desc"`
	Status  string `json:"status" form:"status" query:"status" validate:"in:R,S,T,I,Z,W,L"`
	Keyword string `json:"keyword" form:"keyword" query:"keyword"`
}
