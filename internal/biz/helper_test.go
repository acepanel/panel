package biz

import (
	"encoding/json"
	"testing"

	"github.com/libtnb/assert/check"
	"github.com/libtnb/assert/must"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/pkg/types"
)

const testAppKey = "0123456789abcdef0123456789abcdef"

func TestEncryptDecryptFields(t *testing.T) {
	app.Key = testAppKey

	secret, empty := "secret", ""
	must.NoError(t, encryptFields(&secret, &empty))
	check.NotEqual(t, secret, "secret")
	check.NotEqual(t, empty, "")
	must.NoError(t, decryptFields(&secret, &empty))
	check.Equal(t, secret, "secret")
	check.Equal(t, empty, "")

	raw := json.RawMessage(`{"a":1}`)
	must.NoError(t, encryptFields(&raw))
	check.NotEqual(t, string(raw), `{"a":1}`)
	must.NoError(t, decryptFields(&raw))
	check.Equal(t, string(raw), `{"a":1}`)

	// 解不开的值保持原样
	plain := "not encrypted"
	must.NoError(t, decryptFields(&plain))
	check.Equal(t, plain, "not encrypted")

	app.Key = "short"
	defer func() { app.Key = testAppKey }()
	check.Error(t, encryptFields(&plain))
	check.Error(t, decryptFields(&plain))
	check.Equal(t, plain, "not encrypted")
}

func TestBackupStorageSecrets(t *testing.T) {
	app.Key = testAppKey

	s3 := BackupStorage{Type: BackupStorageTypeS3, Info: types.BackupStorageInfo{AccessKey: "ak", SecretKey: "sk", Username: "u"}}
	must.NoError(t, s3.BeforeSave(nil))
	check.NotEqual(t, s3.Info.AccessKey, "ak")
	check.NotEqual(t, s3.Info.SecretKey, "sk")
	check.Equal(t, s3.Info.Username, "u")
	must.NoError(t, s3.AfterFind(nil))
	check.DeepEqual(t, s3.Info, types.BackupStorageInfo{AccessKey: "ak", SecretKey: "sk", Username: "u"})

	// SFTP 没填的密码保持空串
	sftp := BackupStorage{Type: BackupStorageTypeSFTP, Info: types.BackupStorageInfo{Username: "u", PrivateKey: "key"}}
	must.NoError(t, sftp.BeforeSave(nil))
	check.NotEqual(t, sftp.Info.Username, "u")
	check.NotEqual(t, sftp.Info.PrivateKey, "key")
	check.Equal(t, sftp.Info.Password, "")
	must.NoError(t, sftp.AfterFind(nil))
	check.DeepEqual(t, sftp.Info, types.BackupStorageInfo{Username: "u", PrivateKey: "key"})

	// WebDAV 的空密码照样加密
	webdav := BackupStorage{Type: BackupStorageTypeWebDAV, Info: types.BackupStorageInfo{Username: "u"}}
	must.NoError(t, webdav.BeforeSave(nil))
	check.NotEqual(t, webdav.Info.Username, "u")
	check.NotEqual(t, webdav.Info.Password, "")
	check.Equal(t, webdav.Info.PrivateKey, "")
	must.NoError(t, webdav.AfterFind(nil))
	check.DeepEqual(t, webdav.Info, types.BackupStorageInfo{Username: "u"})

	local := BackupStorage{Type: BackupStorageTypeLocal, Info: types.BackupStorageInfo{Username: "u", Password: "p"}}
	must.NoError(t, local.BeforeSave(nil))
	check.DeepEqual(t, local.Info, types.BackupStorageInfo{Username: "u", Password: "p"})
}
