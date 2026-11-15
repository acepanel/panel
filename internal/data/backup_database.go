package data

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cast"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/pkg/db"
	"github.com/acepanel/panel/v3/pkg/io"
	"github.com/acepanel/panel/v3/pkg/shell"
	"github.com/acepanel/panel/v3/pkg/storage"
	"github.com/acepanel/panel/v3/pkg/systemctl"
	"github.com/acepanel/panel/v3/pkg/tools"
)

// redisLikeConf redis/valkey 的连接与持久化参数
type redisLikeConf struct {
	kind       string // redis / valkey
	cli        string // redis-cli / valkey-cli
	authEnv    string // REDISCLI_AUTH / VALKEYCLI_AUTH
	dataDir    string // {app.Root}/server/{kind}
	confPath   string
	port       string
	password   string
	appendonly bool
}

// createMySQL 创建 MySQL 备份
func (r *backupRepo) createMySQL(ctx context.Context, name string, storage storage.Storage, target string) error {
	mysql, rootPassword, err := r.openMySQL(ctx, target)
	if err != nil {
		return err
	}
	defer mysql.Close()

	// 创建用于压缩的临时目录
	tmpDir, err := r.tmpDir()
	if err != nil {
		return err
	}
	defer func(path string) { _ = os.RemoveAll(path) }(tmpDir)

	if app.IsCli {
		fmt.Println(r.t.Get("|-Temporary directory: %s", tmpDir))
	}

	// 导出数据库
	var gtidMode string
	dumpArgs := "--single-transaction --quick --routines --events --max-allowed-packet=1G"
	if mysql.QueryRow(ctx, `SELECT @@gtid_mode`).Scan(&gtidMode) == nil {
		dumpArgs += " --set-gtid-purged=OFF"
	}
	name += ".sql"
	if _, err = shell.ExecfWithEnv(
		ctx, []string{"MYSQL_PWD=" + rootPassword},
		`mysqldump -u root %s '%s' > '%s'`,
		dumpArgs,
		target,
		filepath.Join(tmpDir, name),
	); err != nil {
		return err
	}

	// 压缩备份文件
	if err = io.Compress(ctx, tmpDir, []string{name}, filepath.Join(tmpDir, name+r.backupExt())); err != nil {
		return err
	}

	// 上传备份文件到存储器
	name += r.backupExt()
	file, err := os.Open(filepath.Join(tmpDir, name))
	if err != nil {
		return err
	}
	defer func(file *os.File) { _ = file.Close() }(file)

	if err = r.upload(ctx, storage, filepath.Join(string(biz.BackupTypeMySQL), name), file); err != nil {
		return err
	}

	if app.IsCli {
		fmt.Println(r.t.Get("|-Backup file: %s", name))
	}

	return nil
}

// restoreMySQL 恢复 MySQL 备份
func (r *backupRepo) restoreMySQL(ctx context.Context, backup, target string) error {
	mysql, rootPassword, err := r.openMySQL(ctx, target)
	if err != nil {
		return err
	}
	defer mysql.Close()

	backup, cleanDir, err := r.prepareDatabaseBackup(ctx, backup, target)
	if err != nil {
		return err
	}
	if cleanDir != "" {
		defer func() { _ = os.RemoveAll(cleanDir) }()
		if app.IsCli {
			fmt.Println(r.t.Get("|-Uncompressing backup..."))
		}
	}

	if app.IsCli {
		fmt.Println(r.t.Get("|-Importing SQL into database..."))
	}
	return r.importFile(ctx, backup, "mysql", []string{"MYSQL_PWD=" + rootPassword}, []string{"-u", "root", "--max-allowed-packet=1G", "--database=" + target})
}

// openMySQL 以 root 连接本机 MySQL 并确认目标库存在，返回连接与 root 密码
func (r *backupRepo) openMySQL(ctx context.Context, target string) (db.Operator, string, error) {
	password, err := r.setting.Get(biz.SettingKeyMySQLRootPassword)
	if err != nil {
		return nil, "", err
	}
	mysql, err := db.NewMySQL(ctx, "root", password, db.MySQLSocket(app.Root), "unix")
	if err != nil {
		return nil, "", err
	}
	if exist, _ := mysql.DatabaseExists(ctx, target); !exist {
		mysql.Close()
		return nil, "", errors.New(r.t.Get("database does not exist: %s", target))
	}

	return mysql, password, nil
}

// createPostgres 创建 PostgreSQL 备份
func (r *backupRepo) createPostgres(ctx context.Context, name string, storage storage.Storage, target string) error {
	postgresPassword, port, err := r.checkPostgres(ctx, target)
	if err != nil {
		return err
	}

	// 创建用于压缩的临时目录
	tmpDir, err := r.tmpDir()
	if err != nil {
		return err
	}
	defer func(path string) { _ = os.RemoveAll(path) }(tmpDir)

	if app.IsCli {
		fmt.Println(r.t.Get("|-Temporary directory: %s", tmpDir))
	}

	// 导出数据库
	name += ".sql"
	if _, err = shell.ExecfWithEnv(
		ctx, []string{"PGPASSWORD=" + postgresPassword},
		`pg_dump -h 127.0.0.1 -p %d -U postgres --clean --if-exists '%s' > '%s'`,
		port,
		target,
		filepath.Join(tmpDir, name),
	); err != nil {
		return err
	}

	// 压缩备份文件
	if err = io.Compress(ctx, tmpDir, []string{name}, filepath.Join(tmpDir, name+r.backupExt())); err != nil {
		return err
	}

	// 上传备份文件到存储器
	name += r.backupExt()
	file, err := os.Open(filepath.Join(tmpDir, name))
	if err != nil {
		return err
	}
	defer func(file *os.File) { _ = file.Close() }(file)

	if err = r.upload(ctx, storage, filepath.Join(string(biz.BackupTypePostgres), name), file); err != nil {
		return err
	}

	if app.IsCli {
		fmt.Println(r.t.Get("|-Backup file: %s", name))
	}

	return nil
}

// restorePostgres 恢复 PostgreSQL 备份
func (r *backupRepo) restorePostgres(ctx context.Context, backup, target string) error {
	postgresPassword, port, err := r.checkPostgres(ctx, target)
	if err != nil {
		return err
	}

	archive := r.postgresArchive(backup)
	cleanDir := ""
	if !archive {
		backup, cleanDir, err = r.prepareDatabaseBackup(ctx, backup, target)
		if err != nil {
			return err
		}
		archive = r.postgresArchive(backup)
	}
	if cleanDir != "" {
		defer func() { _ = os.RemoveAll(cleanDir) }()
		if app.IsCli {
			fmt.Println(r.t.Get("|-Uncompressing backup..."))
		}
	}

	if app.IsCli {
		fmt.Println(r.t.Get("|-Importing PostgreSQL backup..."))
	}
	if archive {
		command := shell.Command(ctx, "pg_restore", "--exit-on-error", "-h", "127.0.0.1", "-p", cast.ToString(port), "-U", "postgres", "--dbname="+target, backup)
		command.Env = append(command.Env, "PGPASSWORD="+postgresPassword)
		if output, restoreErr := command.CombinedOutput(); restoreErr != nil {
			return fmt.Errorf("%w: %s", restoreErr, strings.TrimSpace(string(output)))
		}
		return nil
	}
	return r.importFile(ctx, backup, "psql", []string{"PGPASSWORD=" + postgresPassword},
		[]string{"-h", "127.0.0.1", "-p", cast.ToString(port), "-U", "postgres", "-v", "ON_ERROR_STOP=1", "--single-transaction", "--dbname=" + target})
}

// checkPostgres 以 postgres 连接本机 PostgreSQL 确认目标库存在，返回 postgres 密码与端口
func (r *backupRepo) checkPostgres(ctx context.Context, target string) (string, uint, error) {
	password, err := r.setting.Get(biz.SettingKeyPostgresPassword)
	if err != nil {
		return "", 0, err
	}
	port := db.PostgresPort(app.Root)
	postgres, err := db.NewPostgres(ctx, "postgres", password, "127.0.0.1", port)
	if err != nil {
		return "", 0, err
	}
	defer postgres.Close()
	if exist, _ := postgres.DatabaseExists(ctx, target); !exist {
		return "", 0, errors.New(r.t.Get("database does not exist: %s", target))
	}

	return password, port, nil
}

// createClickHouse 创建 ClickHouse 备份
func (r *backupRepo) createClickHouse(ctx context.Context, name string, storage storage.Storage, target string) error {
	conn, _, err := r.clickHouseConn(ctx, target)
	if err != nil {
		return err
	}

	// 创建用于压缩的临时目录
	tmpDir, err := r.tmpDir()
	if err != nil {
		return err
	}
	defer func(path string) { _ = os.RemoveAll(path) }(tmpDir)

	if app.IsCli {
		fmt.Println(r.t.Get("|-Temporary directory: %s", tmpDir))
	}

	// 数据表（含数据）在前，视图（仅结构）在后
	dataTables, err := r.clickHouseTables(ctx, conn, target, false)
	if err != nil {
		return err
	}
	views, err := r.clickHouseTables(ctx, conn, target, true)
	if err != nil {
		return err
	}

	// 导出结构到 schema.sql，去掉库名限定（恢复时由 --database 指定目标库）
	objects := make([]string, 0, len(dataTables)+len(views))
	objects = append(objects, dataTables...)
	objects = append(objects, views...)
	var schema strings.Builder
	for _, tbl := range objects {
		create, err := shell.Execf(ctx, "clickhouse-client %s --query \"SELECT create_table_query FROM system.tables WHERE database = '%s' AND name = '%s' FORMAT TabSeparatedRaw\"", conn, target, tbl)
		if err != nil {
			return err
		}
		stmt := strings.TrimSpace(create)
		stmt = strings.ReplaceAll(stmt, fmt.Sprintf("`%s`.", target), "")
		stmt = strings.ReplaceAll(stmt, target+".", "")
		schema.WriteString(stmt)
		schema.WriteString(";\n")
	}
	if err = io.Write(filepath.Join(tmpDir, "schema.sql"), schema.String(), 0644); err != nil {
		return err
	}

	// 导出数据（仅数据表，Native 格式）
	files := []string{"schema.sql"}
	for _, tbl := range dataTables {
		dataFile := tbl + ".native"
		if _, err = shell.Execf(ctx, "clickhouse-client %s --query 'SELECT * FROM `%s`.`%s` FORMAT Native' > '%s'", conn, target, tbl, filepath.Join(tmpDir, dataFile)); err != nil {
			return err
		}
		files = append(files, dataFile)
	}

	// 压缩备份文件
	name += r.backupExt()
	if err = io.Compress(ctx, tmpDir, files, filepath.Join(tmpDir, name)); err != nil {
		return err
	}

	// 上传备份文件到存储器
	file, err := os.Open(filepath.Join(tmpDir, name))
	if err != nil {
		return err
	}
	defer func(file *os.File) { _ = file.Close() }(file)

	if err = r.upload(ctx, storage, filepath.Join(string(biz.BackupTypeClickHouse), name), file); err != nil {
		return err
	}

	if app.IsCli {
		fmt.Println(r.t.Get("|-Backup file: %s", name))
	}

	return nil
}

// clickHouseTables 列出库中对象名，onlyView 为 true 时仅返回视图，否则返回非视图数据表
func (r *backupRepo) clickHouseTables(ctx context.Context, conn, database string, onlyView bool) ([]string, error) {
	op := "NOT LIKE"
	if onlyView {
		op = "LIKE"
	}
	out, err := shell.Execf(ctx, "clickhouse-client %s --query \"SELECT name FROM system.tables WHERE database = '%s' AND NOT is_temporary AND engine %s '%%View%%' ORDER BY name FORMAT TabSeparated\"", conn, database, op)
	if err != nil {
		return nil, err
	}

	var tables []string
	for line := range strings.SplitSeq(out, "\n") {
		if l := strings.TrimSpace(line); l != "" {
			tables = append(tables, l)
		}
	}
	return tables, nil
}

// restoreClickHouse 恢复 ClickHouse 备份
func (r *backupRepo) restoreClickHouse(ctx context.Context, backup, target string) error {
	_, password, err := r.clickHouseConn(ctx, target)
	if err != nil {
		return err
	}
	connArgs := []string{"--host", "127.0.0.1", "--port", "9000", "--user", "default", "--password", password}

	// 纯 SQL 文件直接执行
	if strings.HasSuffix(backup, ".sql") {
		if app.IsCli {
			fmt.Println(r.t.Get("|-Importing SQL into database..."))
		}
		return r.importFile(ctx, backup, "clickhouse-client", nil, append(slices.Clone(connArgs), "--database", target, "--multiquery"))
	}

	// 解压到临时目录
	tmpDir, err := r.tmpDir()
	if err != nil {
		return err
	}
	defer func(path string) { _ = os.RemoveAll(path) }(tmpDir)

	if app.IsCli {
		fmt.Println(r.t.Get("|-Temporary directory: %s", tmpDir))
		fmt.Println(r.t.Get("|-Uncompressing backup..."))
	}
	if err = io.UnCompress(ctx, backup, tmpDir); err != nil {
		return err
	}

	// 先恢复结构（视图已排在末尾，源表先建好，规避物化视图依赖顺序问题）
	schemaPath := filepath.Join(tmpDir, "schema.sql")
	if io.Exists(schemaPath) {
		if app.IsCli {
			fmt.Println(r.t.Get("|-Restoring schema..."))
		}
		if err = r.importFile(ctx, schemaPath, "clickhouse-client", nil, append(slices.Clone(connArgs), "--database", target, "--multiquery")); err != nil {
			return err
		}
	}

	// 再导入数据
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".native") {
			continue
		}
		tbl := strings.TrimSuffix(entry.Name(), ".native")
		if app.IsCli {
			fmt.Println(r.t.Get("|-Importing table: %s", tbl))
		}
		query := fmt.Sprintf("INSERT INTO `%s` FORMAT Native", tbl)
		if err = r.importFile(ctx, filepath.Join(tmpDir, entry.Name()), "clickhouse-client", nil, append(slices.Clone(connArgs), "--database", target, "--query", query)); err != nil {
			return err
		}
	}

	return nil
}

// clickHouseConn 确认目标库存在，返回 clickhouse-client 的连接参数与 default 密码
func (r *backupRepo) clickHouseConn(ctx context.Context, target string) (string, string, error) {
	password, err := r.setting.Get(biz.SettingKeyClickHouseDefaultPassword)
	if err != nil {
		return "", "", err
	}
	// clickhouse-client 走 native 9000 端口，本地 default 用户
	conn := fmt.Sprintf("--host 127.0.0.1 --port 9000 --user default --password '%s'", password)

	exist, err := shell.Execf(ctx, "clickhouse-client %s --query \"SELECT count() FROM system.databases WHERE name = '%s'\"", conn, target)
	if err != nil {
		return "", "", err
	}
	if strings.TrimSpace(exist) == "0" {
		return "", "", errors.New(r.t.Get("database does not exist: %s", target))
	}

	return conn, password, nil
}

// createRedisLike 创建 Redis/Valkey 整实例备份（{kind}-cli --rdb 导出快照）
// redis 与 valkey 共用本实现，kind 为 "redis" 或 "valkey"
func (r *backupRepo) createRedisLike(ctx context.Context, name string, storage storage.Storage, kind string) error {
	conf, err := r.loadRedisLikeConf(kind)
	if err != nil {
		return err
	}

	var env []string
	if conf.password != "" {
		env = append(env, conf.authEnv+"="+conf.password)
	}

	// 创建用于压缩的临时目录
	tmpDir, err := r.tmpDir()
	if err != nil {
		return err
	}
	defer func(path string) { _ = os.RemoveAll(path) }(tmpDir)

	if app.IsCli {
		fmt.Println(r.t.Get("|-Temporary directory: %s", tmpDir))
	}

	// 通过复制协议拉取整实例 RDB 快照到本地文件
	rdb := filepath.Join(tmpDir, "dump.rdb")
	if _, err = shell.ExecfWithEnv(ctx, env, "%s -h 127.0.0.1 -p %s --rdb '%s'", conf.cli, conf.port, rdb); err != nil {
		return err
	}
	if !io.Exists(rdb) {
		return errors.New(r.t.Get("failed to export RDB snapshot"))
	}

	// 压缩备份文件
	name += r.backupExt()
	if err = io.Compress(ctx, tmpDir, []string{"dump.rdb"}, filepath.Join(tmpDir, name)); err != nil {
		return err
	}

	// 上传备份文件到存储器
	file, err := os.Open(filepath.Join(tmpDir, name))
	if err != nil {
		return err
	}
	defer func(file *os.File) { _ = file.Close() }(file)

	if err = r.upload(ctx, storage, filepath.Join(kind, name), file); err != nil {
		return err
	}

	if app.IsCli {
		fmt.Println(r.t.Get("|-Backup file: %s", name))
	}

	return nil
}

// restoreRedisLike 恢复 Redis/Valkey 整实例备份
// 停服务 → 替换 dump.rdb → 启动；妥善处理 AOF 优先级陷阱（appendonly=yes 时 AOF 会盖过 RDB）
func (r *backupRepo) restoreRedisLike(ctx context.Context, backup, kind string) error {
	conf, err := r.loadRedisLikeConf(kind)
	if err != nil {
		return err
	}

	// 准备 dump.rdb：裸 .rdb 直接用，否则解压取包内的 dump.rdb
	rdb := backup
	if !strings.HasSuffix(backup, ".rdb") {
		tmpDir, err := r.tmpDir()
		if err != nil {
			return err
		}
		defer func(path string) { _ = os.RemoveAll(path) }(tmpDir)
		if app.IsCli {
			fmt.Println(r.t.Get("|-Temporary directory: %s", tmpDir))
			fmt.Println(r.t.Get("|-Uncompressing backup..."))
		}
		if err = io.UnCompress(ctx, backup, tmpDir); err != nil {
			return err
		}
		rdb = filepath.Join(tmpDir, "dump.rdb")
		if !io.Exists(rdb) {
			return errors.New(r.t.Get("dump.rdb not found in backup file"))
		}
	}

	if app.IsCli {
		fmt.Println(r.t.Get("|-Stopping %s service...", conf.kind))
	}
	// 停服到重新启动之间被取消会让实例停在半替换状态，这段全程不可取消
	restoreCtx := context.WithoutCancel(ctx)
	if err = systemctl.Stop(restoreCtx, conf.kind); err != nil {
		return err
	}

	// 清理旧 AOF（多部件目录与旧式单文件），避免 AOF 优先于 RDB 被加载
	_ = io.Remove(filepath.Join(conf.dataDir, "appendonlydir"))
	_ = io.Remove(filepath.Join(conf.dataDir, "appendonly.aof"))

	if app.IsCli {
		fmt.Println(r.t.Get("|-Replacing dump.rdb..."))
	}
	// 覆盖 dump.rdb
	target := filepath.Join(conf.dataDir, "dump.rdb")
	if err = io.Cp(restoreCtx, rdb, target); err != nil {
		_ = systemctl.Start(restoreCtx, conf.kind) // 尽力恢复服务
		return err
	}
	_ = io.Chown(target, kind, kind)
	_ = io.Chmod(target, 0640)

	// 若原本开启 AOF，必须先以 appendonly no 启动加载 RDB，否则会建空 AOF 以空库覆盖
	if conf.appendonly {
		if err = r.disableAppendonly(conf.confPath); err != nil {
			_ = systemctl.Start(restoreCtx, conf.kind)
			return err
		}
	}

	if app.IsCli {
		fmt.Println(r.t.Get("|-Starting %s service (loading RDB)...", conf.kind))
	}
	// 启动服务（Type=notify，返回即已加载 RDB）
	if err = systemctl.Start(restoreCtx, conf.kind); err != nil {
		return err
	}

	// 原本开启 AOF 的，在线转回并持久化配置
	if conf.appendonly {
		if app.IsCli {
			fmt.Println(r.t.Get("|-Re-enabling AOF..."))
		}
		var env []string
		if conf.password != "" {
			env = append(env, conf.authEnv+"="+conf.password)
		}
		_, _ = shell.ExecfWithEnv(restoreCtx, env, "%s -h 127.0.0.1 -p %s config set appendonly yes", conf.cli, conf.port)
		_, _ = shell.ExecfWithEnv(restoreCtx, env, "%s -h 127.0.0.1 -p %s config rewrite", conf.cli, conf.port)
	}

	return nil
}

// loadRedisLikeConf 从 {kind}.conf 读取连接与持久化参数
func (r *backupRepo) loadRedisLikeConf(kind string) (*redisLikeConf, error) {
	dataDir := fmt.Sprintf("%s/server/%s", app.Root, kind)
	confPath := fmt.Sprintf("%s/%s.conf", dataDir, kind)
	content, err := io.Read(confPath)
	if err != nil {
		return nil, err
	}

	port := r.redisLikeValue(content, "port")
	if port == "" {
		port = "6379"
	}
	authEnv := "REDISCLI_AUTH"
	if kind == "valkey" {
		authEnv = "VALKEYCLI_AUTH"
	}

	return &redisLikeConf{
		kind:       kind,
		cli:        kind + "-cli",
		authEnv:    authEnv,
		dataDir:    dataDir,
		confPath:   confPath,
		port:       port,
		password:   r.redisLikeValue(content, "requirepass"),
		appendonly: r.redisLikeValue(content, "appendonly") == "yes",
	}, nil
}

// redisLikeValue 从 redis/valkey 配置内容中读取指定键的有效值（忽略注释行）
func (r *backupRepo) redisLikeValue(content, key string) string {
	for line := range strings.SplitSeq(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) >= 2 && fields[0] == key {
			return strings.Join(fields[1:], " ")
		}
	}
	return ""
}

// disableAppendonly 将 redis/valkey 配置中的 appendonly 临时改为 no
func (r *backupRepo) disableAppendonly(confPath string) error {
	content, err := io.Read(confPath)
	if err != nil {
		return err
	}

	lines := strings.Split(content, "\n")
	found := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		if fields := strings.Fields(trimmed); len(fields) >= 1 && fields[0] == "appendonly" {
			lines[i] = "appendonly no"
			found = true
			break
		}
	}
	if !found {
		lines = append(lines, "appendonly no")
	}

	return io.Write(confPath, strings.Join(lines, "\n"), 0644)
}

// importFile 把备份文件喂给数据库客户端 stdin，并按秒级周期打印大致进度
// pipe 有背压，已写入字节数 ≈ 客户端已消费字节数，足以作为进度参考
func (r *backupRepo) importFile(ctx context.Context, path, name string, env, args []string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func(f *os.File) { _ = f.Close() }(f)

	var total int64
	if stat, err := f.Stat(); err == nil {
		total = stat.Size()
	}

	var progress func(written, total int64, rate float64)
	if app.IsCli {
		fmt.Println(r.t.Get("|-File size: %s", tools.FormatBytes(float64(total))))
		progress = func(written, total int64, rate float64) {
			if total > 0 {
				fmt.Println(r.t.Get("|-Importing: %s / %s (%.1f%%, %s/s)",
					tools.FormatBytes(float64(written)), tools.FormatBytes(float64(total)),
					float64(written)*100/float64(total), tools.FormatBytes(rate)))
			} else {
				fmt.Println(r.t.Get("|-Importing: %s (%s/s)",
					tools.FormatBytes(float64(written)), tools.FormatBytes(rate)))
			}
		}
	}

	_, err = shell.ExecWithStdinProgress(ctx, name, args, env, f, total, 5*time.Second, progress)
	return err
}
