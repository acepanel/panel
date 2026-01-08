package request

// ProcessKill 结束进程请求
type ProcessKill struct {
	PID int32 `json:"pid" validate:"required"`
}

// ProcessSignal 发送信号请求
type ProcessSignal struct {
	PID    int32 `json:"pid" validate:"required"`
	Signal int   `json:"signal" validate:"required|in:1,2,9,15,17,18,19,23,10,12"`
}

// ProcessList 进程列表请求
type ProcessList struct {
	Page    uint   `json:"page" form:"page" query:"page"`
	Limit   uint   `json:"limit" form:"limit" query:"limit"`
	Sort    string `json:"sort" form:"sort" query:"sort"`       // pid, name, cpu, rss, start_time
	Order   string `json:"order" form:"order" query:"order"`    // asc, desc
	Status  string `json:"status" form:"status" query:"status"` // R, S, T, I, Z, W, L
	Keyword string `json:"keyword" form:"keyword" query:"keyword"`
}
