package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

type SFTPConfig struct {
	Host       string        // SFTP 服务器地址
	Port       int           // SFTP 端口，默认 22
	Username   string        // 用户名
	Password   string        // 密码
	PrivateKey string        // SSH 私钥
	BasePath   string        // 基础路径
	Timeout    time.Duration // 连接超时时间
}

type sftpStorage struct {
	config SFTPConfig
}

func NewSFTP(config SFTPConfig) (Storage, error) {
	if config.Port == 0 {
		config.Port = 22
	}
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	config.BasePath = strings.TrimSuffix(config.BasePath, "/")

	if config.Username == "" || (config.Password == "" && config.PrivateKey == "") {
		return nil, errors.New("username and either password or private key must be provided")
	}

	return &sftpStorage{config: config}, nil
}

// Delete 删除文件
func (s *sftpStorage) Delete(ctx context.Context, files ...string) error {
	client, cleanup, err := s.connect(ctx)
	if err != nil {
		return err
	}
	defer cleanup()

	for _, file := range files {
		remotePath := joinPath(s.config.BasePath, file)
		if err = client.Remove(remotePath); err != nil {
			return err
		}
	}
	return nil
}

// List 列出目录下的所有文件
func (s *sftpStorage) List(ctx context.Context, path string) ([]File, error) {
	client, cleanup, err := s.connect(ctx)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	// 确保基础路径存在
	if s.config.BasePath != "" {
		_ = client.MkdirAll(s.config.BasePath)
	}

	entries, err := client.ReadDir(joinPath(s.config.BasePath, path))
	if err != nil {
		return nil, err
	}
	return toFiles(entries), nil
}

// Put 写入文件内容
func (s *sftpStorage) Put(ctx context.Context, file string, content io.Reader) error {
	client, cleanup, err := s.connect(ctx)
	if err != nil {
		return err
	}
	defer cleanup()

	remotePath := joinPath(s.config.BasePath, file)

	// 确保目录存在
	remoteDir := filepath.Dir(remotePath)
	if remoteDir != "." {
		_ = client.MkdirAll(remoteDir)
	}

	// 确保基础路径存在
	if s.config.BasePath != "" {
		_ = client.MkdirAll(s.config.BasePath)
	}

	remoteFile, err := client.Create(remotePath)
	if err != nil {
		return err
	}

	if _, err = io.Copy(remoteFile, &ctxReader{ctx: ctx, r: content}); err != nil {
		_ = remoteFile.Close()
		return err
	}

	// 写入错误可能要到 Close 才暴露，这里不能吞
	return remoteFile.Close()
}

// Rename 改名
func (s *sftpStorage) Rename(ctx context.Context, src, dst string) error {
	client, cleanup, err := s.connect(ctx)
	if err != nil {
		return err
	}
	defer cleanup()

	return client.Rename(joinPath(s.config.BasePath, src), joinPath(s.config.BasePath, dst))
}

// connect 建立 SFTP 连接，返回 client 和 cleanup 函数
func (s *sftpStorage) connect(ctx context.Context) (*sftp.Client, func(), error) {
	var auth []ssh.AuthMethod
	// 密码认证
	if s.config.Password != "" {
		auth = append(auth, ssh.Password(s.config.Password))
	}
	// 私钥认证
	if s.config.PrivateKey != "" {
		signer, err := ssh.ParsePrivateKey([]byte(s.config.PrivateKey))
		if err != nil {
			return nil, nil, fmt.Errorf("failed to parse private key: %w", err)
		}
		auth = append(auth, ssh.PublicKeys(signer))
	}

	clientConfig := &ssh.ClientConfig{
		User:            s.config.Username,
		Auth:            auth,
		Timeout:         s.config.Timeout,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), //nolint:gosec
	}

	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)
	conn, err := (&net.Dialer{Timeout: s.config.Timeout}).DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to connect to SSH server: %w", err)
	}
	// ClientConfig.Timeout 只管拨号，握手和之后的读写在连接半死时会一直挂着；取消时直接关连接把它踹醒
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })

	sshConn, chans, reqs, err := ssh.NewClientConn(conn, addr, clientConfig)
	if err != nil {
		stop()
		_ = conn.Close()
		return nil, nil, fmt.Errorf("failed to connect to SSH server: %w", err)
	}
	sshClient := ssh.NewClient(sshConn, chans, reqs)

	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		stop()
		_ = sshClient.Close()
		return nil, nil, fmt.Errorf("failed to create SFTP client: %w", err)
	}

	cleanup := func() {
		stop()
		_ = sftpClient.Close()
		_ = sshClient.Close()
	}

	return sftpClient, cleanup, nil
}
