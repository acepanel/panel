package frp

type Name struct {
	Name string `form:"name" json:"name" validate:"required"`
}

type UpdateConfig struct {
	Name   string `form:"name" json:"name" validate:"required"`
	Config string `form:"config" json:"config" validate:"required"`
}

type UpdateUser struct {
	Name  string `form:"name" json:"name" validate:"required"`
	User  string `form:"user" json:"user" validate:"required"`
	Group string `form:"group" json:"group" validate:"required"`
}
