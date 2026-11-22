package request

import (
	"net/http"
	"strings"
	"time"

	"github.com/samber/lo"
)

// WebsiteStatSetting 网站统计设置
type WebsiteStatSetting struct {
	Days          uint `json:"days" validate:"required && min:1 && max:365"`
	ErrBufMax     int  `json:"err_buf_max" validate:"min:0 && max:1000000"`
	UVMaxKeys     int  `json:"uv_max_keys" validate:"min:0 && max:100000000"`
	IPMaxKeys     int  `json:"ip_max_keys" validate:"min:0 && max:100000000"`
	DetailMaxKeys int  `json:"detail_max_keys" validate:"min:0 && max:100000000"`
	BodyEnabled   bool `json:"body_enabled"`
}

// WebsiteStatDateRange 统计日期范围查询参数
type WebsiteStatDateRange struct {
	Start string `json:"start" form:"start" query:"start" validate:"datetime:2006-01-02"`
	End   string `json:"end" form:"end" query:"end" validate:"datetime:2006-01-02 && after_or_equal:Start"`
	Sites string `json:"sites" form:"sites" query:"sites"`
}

func (r *WebsiteStatDateRange) Prepare(_ *http.Request) error {
	if r.Start == "" || r.End == "" {
		today := time.Now().Format(time.DateOnly)
		r.Start = today
		r.End = today
	}
	return nil
}

// SiteList 解析逗号分隔的站点列表
func (r *WebsiteStatDateRange) SiteList() []string {
	if r.Sites == "" {
		return nil
	}
	sites := lo.Filter(strings.Split(r.Sites, ","), func(s string, _ int) bool {
		return strings.TrimSpace(s) != ""
	})
	return lo.Map(sites, func(s string, _ int) string {
		return strings.TrimSpace(s)
	})
}

// WebsiteStatPaginate 统计分页查询参数
type WebsiteStatPaginate struct {
	WebsiteStatDateRange
	Page  uint `json:"page" form:"page" query:"page"`
	Limit uint `json:"limit" form:"limit" query:"limit"`
}

func (r *WebsiteStatPaginate) Prepare(req *http.Request) error {
	if err := r.WebsiteStatDateRange.Prepare(req); err != nil {
		return err
	}
	if r.Page == 0 {
		r.Page = 1
	}
	if r.Limit == 0 {
		r.Limit = 50
	}
	return nil
}

// WebsiteStatGeo 地理位置统计查询参数
type WebsiteStatGeo struct {
	WebsiteStatDateRange
	GroupBy string `json:"group_by" form:"group_by" query:"group_by"`
	Country string `json:"country" form:"country" query:"country"`
	Limit   uint   `json:"limit" form:"limit" query:"limit"`
}

func (r *WebsiteStatGeo) Prepare(req *http.Request) error {
	if err := r.WebsiteStatDateRange.Prepare(req); err != nil {
		return err
	}
	if r.GroupBy == "" {
		r.GroupBy = "country"
	}
	return nil
}

// WebsiteStatSlowURIs 慢请求统计查询参数
type WebsiteStatSlowURIs struct {
	WebsiteStatPaginate
	Threshold uint `json:"threshold" form:"threshold" query:"threshold"`
}

// WebsiteStatErrors 错误日志查询参数
type WebsiteStatErrors struct {
	WebsiteStatPaginate
	Status int `json:"status" form:"status" query:"status"`
}
