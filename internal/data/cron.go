package data

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/utils/str"
	"gorm.io/gorm"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/pkg/io"
	"github.com/acepanel/panel/v3/pkg/os"
	"github.com/acepanel/panel/v3/pkg/shell"
	"github.com/acepanel/panel/v3/pkg/systemctl"
	"github.com/acepanel/panel/v3/pkg/types"
)

// cronLockSkipCode flock 未抢到锁时的退出码，属正常跳过而非失败
const cronLockSkipCode = 200

type cronRepo struct {
	t  *gotext.Locale
	db *gorm.DB
}

func NewCronRepo(db *gorm.DB, t *gotext.Locale) biz.CronRepo {
	return &cronRepo{
		t:  t,
		db: db,
	}
}

func (r *cronRepo) Count() (int64, error) {
	var count int64
	if err := r.db.Model(&biz.Cron{}).Count(&count).Error; err != nil {
		return 0, err
	}

	return count, nil
}

func (r *cronRepo) List(page, limit uint) ([]*biz.Cron, int64, error) {
	cron := make([]*biz.Cron, 0)
	var total int64
	err := r.db.Model(&biz.Cron{}).Order("id desc").Count(&total).Offset(int((page - 1) * limit)).Limit(int(limit)).Find(&cron).Error
	return cron, total, err
}

func (r *cronRepo) Get(id uint) (*biz.Cron, error) {
	cron := new(biz.Cron)
	if err := r.db.Where("id = ?", id).First(cron).Error; err != nil {
		return nil, err
	}

	return cron, nil
}

func (r *cronRepo) Create(cron *biz.Cron) error {
	return r.db.Create(cron).Error
}

func (r *cronRepo) Save(cron *biz.Cron) error {
	return r.db.Save(cron).Error
}

func (r *cronRepo) Delete(cron *biz.Cron) error {
	return r.db.Delete(cron).Error
}

// GenerateScript 根据任务类型和配置生成 shell 脚本
func (r *cronRepo) GenerateScript(typ string, config types.CronConfig, rawScript string) string {
	if typ == "shell" {
		return rawScript
	}

	var sb strings.Builder
	sb.WriteString("#!/bin/bash\nexport PATH=/bin:/sbin:/usr/bin:/usr/sbin:/usr/local/bin:/usr/local/sbin:$PATH\n\n")

	switch typ {
	case "backup":
		for _, target := range config.Targets {
			switch config.Type {
			case "website":
				_, _ = fmt.Fprintf(&sb, "acepanel backup website -n '%s' -s '%d'\n", target, config.Storage)
			case "mysql", "postgresql", "clickhouse", "redis", "valkey":
				_, _ = fmt.Fprintf(&sb, "acepanel backup database -t '%s' -n '%s' -s '%d'\n", config.Type, target, config.Storage)
			case "path":
				_, _ = fmt.Fprintf(&sb, "acepanel backup path -p '%s' -s '%d'\n", target, config.Storage)
			}
		}
		for _, target := range config.Targets {
			switch config.Type {
			case "website":
				_, _ = fmt.Fprintf(&sb, "acepanel backup clear -t website -f '%s' -k '%d' -s '%d'\n", target, config.Keep, config.Storage)
			case "mysql", "postgresql", "clickhouse", "redis", "valkey":
				_, _ = fmt.Fprintf(&sb, "acepanel backup clear -t '%s' -f '%s' -k '%d' -s '%d'\n", config.Type, target, config.Keep, config.Storage)
			case "path":
				_, _ = fmt.Fprintf(&sb, "acepanel backup clear -t path -f '%s' -k '%d' -s '%d'\n", filepath.Base(target), config.Keep, config.Storage)
			}
		}
	case "cutoff":
		for _, target := range config.Targets {
			switch config.Type {
			case "website":
				_, _ = fmt.Fprintf(&sb, "acepanel cutoff website -n '%s' -s '%d'\n", target, config.Storage)
			case "container":
				_, _ = fmt.Fprintf(&sb, "acepanel cutoff container -n '%s' -s '%d'\n", target, config.Storage)
			}
		}
		for _, target := range config.Targets {
			switch config.Type {
			case "website":
				_, _ = fmt.Fprintf(&sb, "acepanel cutoff clear -t website -n '%s' -k '%d' -s '%d'\n", target, config.Keep, config.Storage)
			case "container":
				_, _ = fmt.Fprintf(&sb, "acepanel cutoff clear -t container -n '%s' -k '%d' -s '%d'\n", target, config.Keep, config.Storage)
			}
		}
	case "url":
		method := config.Method
		if method == "" {
			method = "GET"
		}
		_, _ = fmt.Fprintf(&sb, "curl -sSL -X %s", method)
		if config.Timeout > 0 {
			_, _ = fmt.Fprintf(&sb, " --connect-timeout %d", config.Timeout)
		}
		if config.Insecure {
			sb.WriteString(" -k")
		}
		if config.Retries > 0 {
			_, _ = fmt.Fprintf(&sb, " --retry %d", config.Retries)
		}
		for key, value := range config.Headers {
			_, _ = fmt.Fprintf(&sb, " -H '%s: %s'", strings.ReplaceAll(key, "'", "'\"'\"'"), strings.ReplaceAll(value, "'", "'\"'\"'"))
		}
		if config.Body != "" {
			_, _ = fmt.Fprintf(&sb, " -d '%s'", strings.ReplaceAll(config.Body, "'", "'\"'\"'"))
		}
		_, _ = fmt.Fprintf(&sb, " '%s'\n", strings.ReplaceAll(config.URL, "'", "'\"'\"'"))
	case "synctime":
		sb.WriteString("acepanel sync-time\n")
	}

	return sb.String()
}

// WriteNewScript 生成随机脚本文件并返回脚本与日志路径
func (r *cronRepo) WriteNewScript(ctx context.Context, script string) (string, string, error) {
	shellDir := app.Root + "/server/cron/"
	shellLogDir := app.Root + "/server/cron/logs/"
	shellFile := str.Random(16)
	if err := io.Write(filepath.Join(shellDir, shellFile+".sh"), script, 0700); err != nil {
		return "", "", err
	}
	// 编码转换
	_, _ = shell.Execf(ctx, "dos2unix %s%s.sh", shellDir, shellFile)

	return shellDir + shellFile + ".sh", shellLogDir + shellFile + ".log", nil
}

// WriteScript 写入脚本内容到指定路径
func (r *cronRepo) WriteScript(path, script string) error {
	return io.Write(path, script, 0700)
}

// Dos2Unix 转换脚本文件编码
func (r *cronRepo) Dos2Unix(ctx context.Context, path string) error {
	if out, err := shell.Execf(ctx, "dos2unix %s", path); err != nil {
		return errors.New(out)
	}

	return nil
}

// AddToSystem 添加到系统
// 统一经 wrapper 脚本执行，以便捕获退出码并上报失败
func (r *cronRepo) AddToSystem(ctx context.Context, cron *biz.Cron) error {
	cmd := cron.Shell
	if cron.Config.Flock {
		lockFile := strings.TrimSuffix(cron.Shell, ".sh") + ".lock"
		// -E 指定未抢到锁时的退出码，与脚本自身失败区分，避免正常跳过被误报
		cmd = fmt.Sprintf("flock -xn -E %d %s %s", cronLockSkipCode, lockFile, cron.Shell)
	}

	// 秒级任务由每分钟触发的 wrapper 内部循环模拟
	spec := cron.Time
	seconds := r.parseSeconds(cron.Time)
	if seconds > 0 {
		spec = "* * * * *"
	}

	wrapperPath := strings.TrimSuffix(cron.Shell, ".sh") + "_wrapper.sh"
	if err := io.Write(wrapperPath, r.generateWrapper(cron.ID, cmd, cron.Log, seconds), 0700); err != nil {
		return err
	}
	if _, err := shell.Execf(ctx, `( crontab -l; echo "%s %s" ) | sort - | uniq - | crontab -`, spec, wrapperPath); err != nil {
		return err
	}

	return r.restartCron(ctx)
}

// DeleteFromSystem 从系统中删除
func (r *cronRepo) DeleteFromSystem(ctx context.Context, cron *biz.Cron) error {
	// 清理秒级任务的 wrapper 条目和脚本
	wrapperPath := strings.TrimSuffix(cron.Shell, ".sh") + "_wrapper.sh"
	_, _ = shell.Execf(ctx, `( crontab -l | grep -v -F "%s" ) | crontab -`, wrapperPath)
	_ = io.Remove(wrapperPath)

	// 清理普通任务的 crontab 条目
	if _, err := shell.Execf(ctx, `( crontab -l | grep -v -F "%s >> %s 2>&1" ) | crontab -`, cron.Shell, cron.Log); err != nil {
		return err
	}

	return r.restartCron(ctx)
}

// RemoveFiles 清理脚本、日志及关联的 .lock、_wrapper.sh 文件
func (r *cronRepo) RemoveFiles(ctx context.Context, cron *biz.Cron) error {
	if err := io.Remove(cron.Shell); err != nil {
		return err
	}
	base := strings.TrimSuffix(cron.Shell, ".sh")
	_ = io.Remove(base + ".lock")
	_ = io.Remove(base + "_wrapper.sh")
	_ = io.Remove(cron.Log)

	return nil
}

// parseSeconds 从 6 字段 cron 表达式中解析秒级间隔
// 返回 0 表示非秒级任务
func (r *cronRepo) parseSeconds(time string) int {
	fields := strings.Fields(time)
	if len(fields) != 6 {
		return 0
	}

	// 6 字段格式：秒 分 时 日 月 周，后 5 个字段必须全是 *
	for _, f := range fields[1:] {
		if f != "*" {
			return 0
		}
	}

	second := fields[0]
	// 每秒：* * * * * *
	if second == "*" {
		return 1
	}
	// 每 N 秒：*/N * * * * *
	if strings.HasPrefix(second, "*/") {
		n, err := strconv.Atoi(second[2:])
		if err != nil || n <= 0 || n > 59 {
			return 0
		}
		return n
	}

	return 0
}

// generateWrapper 生成任务的 wrapper 脚本，捕获退出码并上报失败
// seconds 大于 0 时为秒级任务，用每分钟触发 + 循环 sleep 模拟
func (r *cronRepo) generateWrapper(id uint, cmd, logFile string, seconds int) string {
	// wrapperPathEnv crontab 环境的 PATH 极简，需补全以便调用 acepanel
	const wrapperPathEnv = "export PATH=/bin:/sbin:/usr/bin:/usr/sbin:/usr/local/bin:/usr/local/sbin:$PATH"

	if seconds <= 0 {
		return fmt.Sprintf(`#!/bin/bash
%s

%s >> %s 2>&1
code=$?
if [ $code -ne 0 ] && [ $code -ne %d ]; then
    acepanel cron failed -i %d -c $code >/dev/null 2>&1
fi
exit $code
`, wrapperPathEnv, cmd, logFile, cronLockSkipCode, id)
	}

	// 并发执行拿不到子进程退出码，用标记文件汇总本分钟是否出错
	count := 60 / seconds
	return fmt.Sprintf(`#!/bin/bash
%s

INTERVAL=%d
COUNT=%d
FLAG=$(mktemp)
for i in $(seq 1 $COUNT); do
    ( %s >> %s 2>&1; c=$?; [ $c -ne 0 ] && [ $c -ne %d ] && echo $c >> "$FLAG" ) &
    [ $i -lt $COUNT ] && sleep $INTERVAL
done
wait
if [ -s "$FLAG" ]; then
    acepanel cron failed -i %d -c "$(tail -n 1 "$FLAG")" >/dev/null 2>&1
fi
rm -f "$FLAG"
`, wrapperPathEnv, seconds, count, cmd, logFile, cronLockSkipCode, id)
}

// restartCron 重启 cron 服务
func (r *cronRepo) restartCron(ctx context.Context) error {
	if os.IsRHEL() {
		return systemctl.Restart(ctx, "crond")
	}

	if os.IsDebian() || os.IsUbuntu() {
		return systemctl.Restart(ctx, "cron")
	}

	return errors.New(r.t.Get("unsupported system"))
}
