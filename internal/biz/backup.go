package biz

import (
	"context"

	"github.com/acepanel/panel/pkg/types"
)

type BackupType string

const (
	BackupTypePath     BackupType = "path"
	BackupTypeWebsite  BackupType = "website"
	BackupTypeMySQL    BackupType = "mysql"
	BackupTypePostgres BackupType = "postgres"
	BackupTypeRedis    BackupType = "redis"
	BackupTypePanel    BackupType = "panel"
)

type BackupRepo interface {
	List(typ BackupType) ([]*types.BackupFile, error)
	Create(ctx context.Context, typ BackupType, target string, path ...string) error
	Delete(ctx context.Context, typ BackupType, name string) error
	Restore(ctx context.Context, typ BackupType, backup, target string) error
	ClearExpired(path, prefix string, save int) error
	CutoffLog(path, target string) error
	GetPath(typ BackupType) (string, error)
	FixPanel() error
	UpdatePanel(version, url, checksum string) error
}
