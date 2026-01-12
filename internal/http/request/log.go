package request

// LogList 日志列表请求
type LogList struct {
	Type  string `json:"type" form:"type" query:"type" validate:"required|in:app,db,http"`
	Limit int    `json:"limit" form:"limit" query:"limit" validate:"min:1|max:1000"`
}
