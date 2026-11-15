package confval

import "github.com/spf13/cast"

// Kind 字段写回 TOML 时的字面量类型，表单一律用字符串收值
type Kind uint8

const (
	String Kind = iota
	Int
	Bool
)

// Field 一个配置项与承载其值的表单字段的对应关系
type Field struct {
	// Section 所在分组，仅分组感知的 Codec 使用
	Section string
	Key     string
	Value   *string
	// Kind 仅 TOML 使用
	Kind Kind
}

// literal 空串表示注释掉该项
func (f Field) literal() any {
	if *f.Value == "" {
		return ""
	}

	switch f.Kind {
	case Int:
		return cast.ToInt(*f.Value)
	case Bool:
		return cast.ToBool(*f.Value)
	default:
		return *f.Value
	}
}

// Read 按字段表读出各配置项
func (c Codec) Read(content string, fields []Field) {
	for _, f := range fields {
		*f.Value = c.GetIn(content, f.Section, f.Key)
	}
}

// Write 按字段表写回各配置项
func (c Codec) Write(content string, fields []Field) string {
	for _, f := range fields {
		content = c.SetIn(content, f.Section, f.Key, *f.Value)
	}

	return content
}

// ReadTOML 按字段表读出根表配置项
func ReadTOML(content string, fields []Field) {
	for _, f := range fields {
		*f.Value = getTOML(content, f.Key)
	}
}

// WriteTOML 按字段表写回根表配置项
func WriteTOML(content string, fields []Field) string {
	for _, f := range fields {
		content = setTOML(content, f.Key, f.literal())
	}

	return content
}
