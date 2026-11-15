package types

import (
	"fmt"
	"os"
	"strings"

	lop "github.com/samber/lo/parallel"
	"golang.org/x/crypto/bcrypt"
)

// WriteUserFile 把面板的明文 htpasswd 转写成方言能用的加密文件，返回里面是否有用户。
// 面板文件保持 {PLAIN} 明文以便回读，而 Apache 在 Unix 下不认明文密码、Caddy 只收 bcrypt 或 argon2id；
// 用户文件缺失时当作没有用户，不能阻塞整个站点保存
func WriteUserFile(userFile, suffix, sep string) (bool, error) {
	content, err := os.ReadFile(userFile)
	if err != nil {
		return false, nil //nolint:nilerr // 用户文件缺失不能阻塞整个站点保存
	}

	type entry struct {
		user, password string
		plain          bool
	}
	users := make([]entry, 0)
	for line := range strings.SplitSeq(string(content), "\n") {
		user, password, ok := strings.Cut(strings.TrimSpace(line), ":")
		if !ok || user == "" || strings.HasPrefix(user, "#") {
			continue
		}
		password, plain := strings.CutPrefix(password, "{PLAIN}")
		users = append(users, entry{user, password, plain})
	}

	// bcrypt 是故意设计得慢的纯 CPU 活，重建全部站点时几百个哈希串起来要几十秒
	type hashed struct {
		line string
		err  error
	}
	results := lop.Map(users, func(u entry, _ int) hashed {
		// 已经是哈希的原样带过，用户可能直接手写了加密口令；面板写入的带 {PLAIN} 标记，以 $ 开头也是明文
		if !u.plain && (strings.HasPrefix(u.password, "$") || strings.HasPrefix(u.password, "{SHA}")) {
			return hashed{line: u.user + sep + u.password}
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(u.password), bcrypt.DefaultCost)
		return hashed{line: u.user + sep + string(hash), err: err}
	})

	lines := make([]string, 0, len(results))
	for _, r := range results {
		if r.err != nil {
			return false, r.err
		}
		lines = append(lines, r.line)
	}

	if err = os.WriteFile(userFile+suffix, []byte(strings.Join(lines, "\n")+"\n"), 0600); err != nil {
		return false, fmt.Errorf("failed to write user file: %w", err)
	}

	return len(lines) > 0, nil
}
