package request

type UserID struct {
	ID uint `json:"id" validate:"required|exists:users,id"`
}

type UserLogin struct {
	Username    string `json:"username" validate:"required"` // encrypted with RSA-OAEP
	Password    string `json:"password" validate:"required"` // encrypted with RSA-OAEP
	SafeLogin   bool   `json:"safe_login"`
	PassCode    string `json:"pass_code"`
	CaptchaCode string `json:"captcha_code"` // 验证码
}

type UserIsTwoFA struct {
	Username string `query:"username" validate:"required"`
}

type UserCreate struct {
	Username string `json:"username" validate:"required|notExists:users,username|regex:^[a-zA-Z0-9_-]+$"`
	Password string `json:"password" validate:"required|password"`
	Email    string `json:"email" validate:"required|email"`
}

type UserUpdateUsername struct {
	ID       uint   `json:"id" validate:"required|exists:users,id"`
	Username string `json:"username" validate:"required|notExists:users,username|regex:^[a-zA-Z0-9_-]+$"`
}

type UserUpdatePassword struct {
	ID       uint   `json:"id" validate:"required|exists:users,id"`
	Password string `json:"password" validate:"required|password"`
}

type UserUpdateEmail struct {
	ID    uint   `json:"id" validate:"required|exists:users,id"`
	Email string `json:"email" validate:"required|email"`
}

type UserUpdateTwoFA struct {
	ID     uint   `uri:"id" validate:"required|exists:users,id"`
	Secret string `json:"secret"`
	Code   string `json:"code"`
}
