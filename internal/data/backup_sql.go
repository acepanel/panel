package data

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/pkg/db"
	"github.com/acepanel/panel/v3/pkg/io"
	"github.com/acepanel/panel/v3/pkg/shell"
	"github.com/acepanel/panel/v3/pkg/storage"
)

type sqlBackupEngine struct {
	typ         biz.BackupType
	settingKey  biz.SettingKey
	passwordEnv string
	open        func(password string) (db.Operator, error)
	dumpCmd     func(target, output string) string
	restoreCmd  func(target, input string) string
}

func (r *backupRepo) mysqlBackupEngine() sqlBackupEngine {
	return sqlBackupEngine{
		typ:         biz.BackupTypeMySQL,
		settingKey:  biz.SettingKeyMySQLRootPassword,
		passwordEnv: "MYSQL_PWD",
		open: func(password string) (db.Operator, error) {
			return db.NewMySQL("root", password, "/tmp/mysql.sock", "unix")
		},
		dumpCmd: func(target, output string) string {
			return fmt.Sprintf("mysqldump -u root --single-transaction --quick '%s' > '%s'", target, output)
		},
		restoreCmd: func(target, input string) string {
			return fmt.Sprintf("mysql -u root '%s' < '%s'", target, input)
		},
	}
}

func (r *backupRepo) postgresBackupEngine() sqlBackupEngine {
	return sqlBackupEngine{
		typ:         biz.BackupTypePostgres,
		settingKey:  biz.SettingKeyPostgresPassword,
		passwordEnv: "PGPASSWORD",
		open: func(password string) (db.Operator, error) {
			return db.NewPostgres("postgres", password, "127.0.0.1", 5432)
		},
		dumpCmd: func(target, output string) string {
			return fmt.Sprintf("pg_dump -h 127.0.0.1 -U postgres '%s' > '%s'", target, output)
		},
		restoreCmd: func(target, input string) string {
			return fmt.Sprintf("psql -h 127.0.0.1 -U postgres '%s' < '%s'", target, input)
		},
	}
}

func (r *backupRepo) createSQLBackup(name string, client storage.Storage, target string, engine sqlBackupEngine) error {
	password, err := r.setting.Get(engine.settingKey)
	if err != nil {
		return err
	}

	operator, err := engine.open(password)
	if err != nil {
		return err
	}
	defer operator.Close()
	if exist, _ := operator.DatabaseExists(target); !exist {
		return errors.New(r.t.Get("database does not exist: %s", target))
	}

	tmpDir, err := os.MkdirTemp("", "ace-backup-*")
	if err != nil {
		return err
	}
	defer func(path string) { _ = os.RemoveAll(path) }(tmpDir)

	if app.IsCli {
		fmt.Println(r.t.Get("|-Temporary directory: %s", tmpDir))
	}

	sqlName := name + ".sql"
	output := filepath.Join(tmpDir, sqlName)
	_ = os.Setenv(engine.passwordEnv, password)
	if _, err = shell.Execf(engine.dumpCmd(target, output)); err != nil {
		return err
	}
	_ = os.Unsetenv(engine.passwordEnv)

	zipName := sqlName + ".zip"
	if err = io.Compress(tmpDir, []string{sqlName}, filepath.Join(tmpDir, zipName)); err != nil {
		return err
	}

	file, err := os.Open(filepath.Join(tmpDir, zipName))
	if err != nil {
		return err
	}
	defer func(file *os.File) { _ = file.Close() }(file)

	if err = client.Put(filepath.Join(string(engine.typ), zipName), file); err != nil {
		return err
	}

	if app.IsCli {
		fmt.Println(r.t.Get("|-Backup file: %s", zipName))
	}

	return nil
}

func (r *backupRepo) restoreSQLBackup(backup, target string, engine sqlBackupEngine) error {
	password, err := r.setting.Get(engine.settingKey)
	if err != nil {
		return err
	}

	operator, err := engine.open(password)
	if err != nil {
		return err
	}
	defer operator.Close()
	if exist, _ := operator.DatabaseExists(target); !exist {
		return errors.New(r.t.Get("database does not exist: %s", target))
	}

	sqlPath := backup
	clean := false
	if !strings.HasSuffix(sqlPath, ".sql") {
		sqlPath, err = r.autoUnCompressSQL(backup)
		if err != nil {
			return err
		}
		clean = true
	}

	_ = os.Setenv(engine.passwordEnv, password)
	if _, err = shell.Execf(engine.restoreCmd(target, sqlPath)); err != nil {
		return err
	}
	_ = os.Unsetenv(engine.passwordEnv)
	if clean {
		_ = io.Remove(filepath.Dir(sqlPath))
	}

	return nil
}
