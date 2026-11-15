package rule

import (
	"unicode"
	"unicode/utf8"

	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/validator"
	"github.com/spf13/cast"
)

// password 密码复杂度校验
type password struct {
	t *gotext.Locale
}

func newPassword(t *gotext.Locale) *password {
	return &password{t: t}
}

func (r *password) Signature() string { return "password" }

func (r *password) Message() string {
	return r.t.Get("{field} must be 8-20 characters long and contain at least two types of characters: uppercase letters, lowercase letters, numbers, and special characters")
}

func (r *password) Passes(f *validator.Field) bool {
	if validator.IsEmptyValue(f.Reflect()) {
		return true
	}
	password := cast.ToString(f.Reflect().Interface())
	if n := utf8.RuneCountInString(password); n < 8 || n > 20 {
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
