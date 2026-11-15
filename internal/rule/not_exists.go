package rule

import (
	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/validator"
	"gorm.io/gorm"
)

// notExists 验证一个值在某个表中的字段中不存在，支持同时判断多个字段
// 用法：not_exists:表名称,字段名称,字段名称
// 例子：not_exists:users,phone,email
type notExists struct {
	db *gorm.DB
	t  *gotext.Locale
}

func newNotExists(db *gorm.DB, t *gotext.Locale) *notExists {
	return &notExists{db: db, t: t}
}

func (r *notExists) Signature() string { return "not_exists" }

func (r *notExists) Message() string { return r.t.Get("{field} already exists") }

func (r *notExists) Validate(f *validator.Field) (bool, error) {
	rv := f.Reflect()
	if validator.IsEmptyValue(rv) {
		return true, nil
	}
	args := f.Attrs()
	if len(args) < 2 {
		return false, nil
	}

	val := rv.Interface()
	tableName := args[0]
	fieldNames := args[1:]

	query := r.db.Table(tableName).Where(fieldNames[0]+" = ?", val)
	for _, fieldName := range fieldNames[1:] {
		query = query.Or(fieldName+" = ?", val)
	}

	var count int64
	if err := query.Count(&count).Error; err != nil {
		return false, err
	}

	return count == 0, nil
}
