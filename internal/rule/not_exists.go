package rule

import (
	"github.com/libtnb/validator"
	"gorm.io/gorm"
)

// NotExists 验证一个值在某个表中的字段中不存在，支持同时判断多个字段
// 用法：not_exists:表名称,字段名称,字段名称
// 例子：not_exists:users,phone,email
type NotExists struct {
	db *gorm.DB
}

func NewNotExists(db *gorm.DB) *NotExists {
	return &NotExists{db: db}
}

func (r *NotExists) Signature() string { return "not_exists" }

func (r *NotExists) Message() string { return "{field} is exists" }

func (r *NotExists) Validate(f *validator.Field) (bool, error) {
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
