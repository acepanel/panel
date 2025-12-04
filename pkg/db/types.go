package db

type User struct {
	User   string   `json:"user"`   // 用户名，PG 里面对应 Role
	Host   string   `json:"host"`   // 主机，PG 这个字段为空
	Grants []string `json:"grants"` // 权限列表
}

type Database struct {
	Name      string `json:"name"`      // 数据库名
	Owner     string `json:"owner"`     // 所有者，MySQL 这个字段为空
	CharSet   string `json:"char_set"`  // 字符集，PG 里面对应 Encoding
	Collation string `json:"collation"` // 校对集，PG 这个字段为空
	Comment   string `json:"comment"`
}
