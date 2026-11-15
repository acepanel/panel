// Package confval 读写行式配置文件中的键值项
package confval

import (
	"slices"
	"strings"
)

// Codec 描述一种行式配置文件的书写约定
type Codec struct {
	// Delim 读取时分隔键与值的字符，空表示按空白切分
	Delim string
	// Assign 写入时的键值分隔符，如 " "、"="、" = "
	Assign string
	// Terminator 行尾终止符，如 nginx 的 ";"
	Terminator string
	// Quote 写入时给值添加的引号，读取时一并去除
	Quote string
	// Comments 识别为注释的前缀，第一个用于注释掉配置项
	Comments []string
	// Sections 是否按 [section] 分组匹配
	Sections bool
	// InlineComment 该格式支持行尾注释
	// 仅对确实支持的格式开启：redis.conf 与 java properties 不支持，
	// 值里的 "#" 是数据（如密码），误判会损坏配置
	InlineComment bool
	// RequireValue 匹配时要求该行已带值，用于区分「开关」与「键 值」
	RequireValue bool
	// AppendIndent 追加新配置项时使用的缩进
	AppendIndent string
}

// line 为一行配置解析后的各组成部分
type line struct {
	indent    string // 行首缩进
	commented string // 命中的注释前缀，非注释行为空
	key       string
	value     string // 已去引号
	comment   string // 行尾注释（含前缀），无则为空
	ok        bool   // 是否为可识别的键值行
}

// 各应用配置文件的书写约定
var (
	// Directive 「键 值」，用于 redis、valkey
	Directive = Codec{Assign: " ", Comments: []string{"#"}}
	// FTP 「键 值」，pure-ftpd 的配置项必须带值
	FTP = Codec{Assign: " ", Comments: []string{"#"}, RequireValue: true}
	// Properties 「键=值」，用于 kafka、rocketmq
	Properties = Codec{Delim: "=", Assign: "=", Comments: []string{"#"}}
	// Nginx 「键 值;」，支持行尾注释
	Nginx = Codec{Assign: " ", Terminator: ";", Comments: []string{"#"}, InlineComment: true, RequireValue: true, AppendIndent: "    "}
	// Postgres 「键 = '值'」，支持行尾注释
	Postgres = Codec{Delim: "=", Assign: " = ", Quote: "'", Comments: []string{"#"}, InlineComment: true}
	// SectionINI 分组感知的 INI，用于 grafana
	SectionINI = Codec{Delim: "=", Assign: " = ", Comments: []string{"#", ";"}, Sections: true}
	// PHPINI PHP 的 ini，注释优先使用 ";"
	PHPINI = Codec{Delim: "=", Assign: " = ", Comments: []string{";", "#"}}
	// Supervisor 「键=值」，注释同时支持 ";" 与 "#"
	Supervisor = Codec{Delim: "=", Assign: "=", Comments: []string{";", "#"}}
)

// Get 读取配置项，未命中返回空串
func (c Codec) Get(content, key string) string {
	return c.GetIn(content, "", key)
}

// GetIn 在指定分组内读取配置项，section 为空表示不限分组
func (c Codec) GetIn(content, section, key string) string {
	current := ""
	for raw := range strings.SplitSeq(content, "\n") {
		if name, ok := sectionName(raw); ok {
			current = name
			continue
		}
		if c.Sections && current != section {
			continue
		}

		// 被注释掉的配置项视为未设置
		parsed := c.parse(raw)
		if parsed.ok && parsed.commented == "" && parsed.key == key {
			return parsed.value
		}
	}

	return ""
}

// Set 写入配置项，值为空时注释掉该项
func (c Codec) Set(content, key, value string) string {
	return c.SetIn(content, "", key, value)
}

// SetIn 在指定分组内写入配置项，分组不存在时在末尾补建
func (c Codec) SetIn(content, section, key, value string) string {
	value = strings.NewReplacer("\n", "", "\r", "").Replace(value)

	raws := strings.Split(content, "\n")
	result := make([]string, 0, len(raws)+3)
	current := ""
	found := false
	// 目标分组最后一行在 result 中的位置，用于在组内补插新项
	sectionEnd := -1

	for _, raw := range raws {
		if name, ok := sectionName(raw); ok {
			current = name
			result = append(result, raw)
			continue
		}
		if c.Sections && current == section && strings.TrimSpace(raw) != "" {
			sectionEnd = len(result)
		}
		if c.Sections && current != section {
			result = append(result, raw)
			continue
		}

		// 注释掉的配置项同样参与匹配，便于重新启用
		parsed := c.parse(raw)
		if !parsed.ok || parsed.key != key {
			result = append(result, raw)
			continue
		}

		// 同名项只保留第一处，后续重复行丢弃
		if found {
			continue
		}
		found = true

		if value == "" {
			// 值为空时注释掉该配置项，已注释的原样保留
			if parsed.commented == "" {
				result = append(result, parsed.indent+c.comment()+" "+strings.TrimSpace(raw))
			} else {
				result = append(result, raw)
			}
			continue
		}

		// 保留原行的缩进与行尾注释，只替换值
		result = append(result, parsed.indent+c.format(key, value)+trailing(parsed.comment))
		if c.Sections {
			sectionEnd = len(result) - 1
		}
	}

	if found || value == "" {
		return strings.Join(result, "\n")
	}

	newLine := c.AppendIndent + c.format(key, value)
	switch {
	case !c.Sections:
		result = append(result, newLine)
	case sectionEnd >= 0:
		result = slices.Insert(result, sectionEnd+1, newLine)
	default:
		// 分组不存在，在文件末尾补建
		result = append(result, "", "["+section+"]", newLine)
	}

	return strings.Join(result, "\n")
}

// parse 将一行拆解为缩进、注释状态、键、值与行尾注释
func (c Codec) parse(raw string) line {
	parsed := line{indent: indentOf(raw)}

	body := strings.TrimSpace(raw)
	if body == "" {
		return parsed
	}
	if parsed.commented = c.commentOf(body); parsed.commented != "" {
		body = strings.TrimSpace(strings.TrimPrefix(body, parsed.commented))
	}

	// 行尾注释在终止符之后，需先摘除
	if c.InlineComment {
		body, parsed.comment = c.cutComment(body)
	}
	if c.Terminator != "" {
		if !strings.HasSuffix(body, c.Terminator) {
			return parsed
		}
		body = strings.TrimSpace(strings.TrimSuffix(body, c.Terminator))
	}

	key, value, ok := c.split(body)
	if !ok {
		return parsed
	}
	parsed.key, parsed.value, parsed.ok = key, c.unquote(value), true

	return parsed
}

// split 从已剥离注释与终止符的正文中取出键与值
func (c Codec) split(body string) (key, value string, ok bool) {
	if c.Delim != "" {
		k, v, found := strings.Cut(body, c.Delim)
		if !found {
			return "", "", false
		}
		return strings.TrimSpace(k), strings.TrimSpace(v), true
	}

	parts := strings.Fields(body)
	if len(parts) == 0 || (c.RequireValue && len(parts) < 2) {
		return "", "", false
	}

	return parts[0], strings.Join(parts[1:], " "), true
}

// cutComment 切出行尾注释，引号内的注释符是数据而非注释
func (c Codec) cutComment(body string) (string, string) {
	inSingle, inDouble := false, false
	for i := range len(body) {
		switch ch := body[i]; {
		case ch == '\'' && !inDouble:
			inSingle = !inSingle
		case ch == '"' && !inSingle:
			inDouble = !inDouble
		case !inSingle && !inDouble:
			for _, prefix := range c.Comments {
				if strings.HasPrefix(body[i:], prefix) {
					return strings.TrimRight(body[:i], " \t"), body[i:]
				}
			}
		}
	}

	return body, ""
}

// unquote 去除值两端的引号
func (c Codec) unquote(value string) string {
	if c.Quote == "" {
		return value
	}

	return strings.Trim(value, `'"`)
}

// format 拼出一条完整的配置行
func (c Codec) format(key, value string) string {
	return key + c.Assign + c.Quote + value + c.Quote + c.Terminator
}

// comment 注释掉配置项时使用的前缀
func (c Codec) comment() string {
	if len(c.Comments) == 0 {
		return "#"
	}

	return c.Comments[0]
}

// commentOf 返回该行命中的注释前缀，非注释行返回空串
func (c Codec) commentOf(trimmed string) string {
	for _, prefix := range c.Comments {
		if strings.HasPrefix(trimmed, prefix) {
			return prefix
		}
	}

	return ""
}

// trailing 把行尾注释接回改写后的行
func trailing(comment string) string {
	if comment == "" {
		return ""
	}

	return " " + comment
}

// sectionName 解析 [section] 行
func sectionName(raw string) (string, bool) {
	trimmed := strings.TrimSpace(raw)
	if !strings.HasPrefix(trimmed, "[") || !strings.HasSuffix(trimmed, "]") {
		return "", false
	}

	return strings.TrimSpace(trimmed[1 : len(trimmed)-1]), true
}

// indentOf 取出行首缩进
func indentOf(raw string) string {
	return raw[:len(raw)-len(strings.TrimLeft(raw, " \t"))]
}
