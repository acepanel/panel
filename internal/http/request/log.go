package request

// LogList 日志列表请求
type LogList struct {
	Type  string `json:"type" form:"type" query:"type" validate:"required|in:app,db,http"`
	Limit int    `json:"limit" form:"limit" query:"limit" validate:"min:1|max:1000"`
	Date  string `json:"date" form:"date" query:"date"` // 日期，格式为 YYYY-MM-DD，空表示当天
}

// LogDates 日志日期列表请求
type LogDates struct {
	Type string `json:"type" form:"type" query:"type" validate:"required|in:app,db,http"`
}
