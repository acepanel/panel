package request

type UserPasskeyList struct {
	UserID uint `query:"user_id"`
}

type UserPasskeyDelete struct {
	ID     uint `uri:"id" validate:"required|min:1"`
	UserID uint `query:"user_id"`
}
