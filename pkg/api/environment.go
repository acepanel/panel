package api

type Environment struct {
	Type        string `json:"type"`
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
}

type Environments []*Environment

// Environments 返回所有环境
func (r *API) Environments() (*Environments, error) {
	return get[Environments](r, "/environments", nil)
}
