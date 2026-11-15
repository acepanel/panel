package request

import (
	"net/http"
)

type Paginate struct {
	Page  uint `json:"page" form:"page" query:"page" validate:"required && min:1"`
	Limit uint `json:"limit" form:"limit" query:"limit" validate:"required && min:1 && max:10000"`
}

func (r *Paginate) Prepare(_ *http.Request) error {
	if r.Page == 0 {
		r.Page = 1
	}
	if r.Limit == 0 {
		r.Limit = 10
	}
	return nil
}
