package api

type Category struct {
	Slug  string `json:"slug"`
	Name  string `json:"name"`
	Order int    `json:"order"`
}

type Categories []*Category

// Categories 返回所有分类
func (r *API) Categories() (*Categories, error) {
	return get[Categories](r, "/categories", nil)
}
