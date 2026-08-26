package rule

import (
	"unicode"

	"github.com/libtnb/validator"
	"github.com/spf13/cast"
)

// Password 密码复杂度校验
type Password struct{}

func NewPassword() *Password {
	return &Password{}
}

func (r *Password) Signature() string { return "password" }

func (r *Password) Message() string {
	return "{field} must be 8-20 characters long and contain at least two types of characters: uppercase letters, lowercase letters, numbers, and special characters"
}

func (r *Password) Passes(f *validator.Field) bool {
	if validator.IsEmptyValue(f.Reflect()) {
		return true
	}
	password := cast.ToString(f.Reflect().Interface())
	if len(password) < 8 || len(password) > 20 {
		return false
	}

	var hasUpper, hasLower, hasNumber, hasSpecial bool
	for _, char := range password {
		switch {
		case unicode.IsUpper(char):
			hasUpper = true
		case unicode.IsLower(char):
			hasLower = true
		case unicode.IsNumber(char):
			hasNumber = true
		case unicode.IsPunct(char) || unicode.IsSymbol(char):
			hasSpecial = true
		}
	}

	// 至少包含两类字符组合
	return (hasUpper && hasLower) ||
		(hasUpper && hasNumber) ||
		(hasUpper && hasSpecial) ||
		(hasLower && hasNumber) ||
		(hasLower && hasSpecial) ||
		(hasNumber && hasSpecial)
}
