package request

// LogList 日志列表请求
type LogList struct {
	Type  string `json:"type" form:"type" query:"type" validate:"required && in:app,db,http"`
	Limit int    `json:"limit" form:"limit" query:"limit" validate:"min:1 && max:1000"`
	Date  string `json:"date" form:"date" query:"date" validate:"datetime:2006-01-02"`         // 日期，格式为 YYYY-MM-DD，空表示当天
	Level string `json:"level" form:"level" query:"level" validate:"in:DEBUG,INFO,WARN,ERROR"` // 日志级别，空表示全部
}

// LogDates 日志日期列表请求
type LogDates struct {
	Type string `json:"type" form:"type" query:"type" validate:"required && in:app,db,http"`
}

// LogClean 日志清理请求
type LogClean struct {
	Type string `json:"type" form:"type" validate:"required && in:app,db,http"`
	Date string `json:"date" form:"date" validate:"required && datetime:2006-01-02"` // 清理该日期及之前的日志
}
