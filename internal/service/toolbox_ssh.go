package service

import (
	"net/http"
	"regexp"
	"strings"

	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/chix"
	"github.com/spf13/cast"

	"github.com/acepanel/panel/internal/http/request"
	"github.com/acepanel/panel/pkg/io"
	"github.com/acepanel/panel/pkg/shell"
	"github.com/acepanel/panel/pkg/systemctl"
)

type ToolboxSSHService struct {
	t *gotext.Locale
}

func NewToolboxSSHService(t *gotext.Locale) *ToolboxSSHService {
	return &ToolboxSSHService{
		t: t,
	}
}

// GetInfo 获取 SSH 信息
func (s *ToolboxSSHService) GetInfo(w http.ResponseWriter, r *http.Request) {
	// 读取 sshd_config
	sshdConfig, err := io.Read("/etc/ssh/sshd_config")
	if err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("failed to read sshd_config: %v", err))
		return
	}

	// 获取 SSH 服务状态
	status, err := systemctl.Status("sshd")
	if err != nil {
		// 尝试 ssh 服务名
		status, err = systemctl.Status("ssh")
		if err != nil {
			status = false
		}
	}

	// 解析端口
	port := 22
	portMatch := regexp.MustCompile(`(?m)^Port\s+(\d+)`).FindStringSubmatch(sshdConfig)
	if len(portMatch) >= 2 {
		port = cast.ToInt(portMatch[1])
	}

	// 解析密码认证
	passwordAuth := true
	passwordAuthMatch := regexp.MustCompile(`(?m)^PasswordAuthentication\s+(\S+)`).FindStringSubmatch(sshdConfig)
	if len(passwordAuthMatch) >= 2 {
		passwordAuth = strings.ToLower(passwordAuthMatch[1]) == "yes"
	}

	// 解析密钥认证
	pubKeyAuth := true
	pubKeyAuthMatch := regexp.MustCompile(`(?m)^PubkeyAuthentication\s+(\S+)`).FindStringSubmatch(sshdConfig)
	if len(pubKeyAuthMatch) >= 2 {
		pubKeyAuth = strings.ToLower(pubKeyAuthMatch[1]) == "yes"
	}

	// 解析 Root 登录设置
	rootLogin := "yes"
	rootLoginMatch := regexp.MustCompile(`(?m)^PermitRootLogin\s+(\S+)`).FindStringSubmatch(sshdConfig)
	if len(rootLoginMatch) >= 2 {
		rootLogin = strings.ToLower(rootLoginMatch[1])
	}

	Success(w, chix.M{
		"status":        status,
		"port":          port,
		"password_auth": passwordAuth,
		"pubkey_auth":   pubKeyAuth,
		"root_login":    rootLogin,
	})
}

// Start 启动 SSH 服务
func (s *ToolboxSSHService) Start(w http.ResponseWriter, r *http.Request) {
	err := systemctl.Start("sshd")
	if err != nil {
		err = systemctl.Start("ssh")
		if err != nil {
			Error(w, http.StatusInternalServerError, s.t.Get("failed to start SSH service: %v", err))
			return
		}
	}
	Success(w, nil)
}

// Stop 停止 SSH 服务
func (s *ToolboxSSHService) Stop(w http.ResponseWriter, r *http.Request) {
	err := systemctl.Stop("sshd")
	if err != nil {
		err = systemctl.Stop("ssh")
		if err != nil {
			Error(w, http.StatusInternalServerError, s.t.Get("failed to stop SSH service: %v", err))
			return
		}
	}
	Success(w, nil)
}

// Restart 重启 SSH 服务
func (s *ToolboxSSHService) Restart(w http.ResponseWriter, r *http.Request) {
	err := systemctl.Restart("sshd")
	if err != nil {
		err = systemctl.Restart("ssh")
		if err != nil {
			Error(w, http.StatusInternalServerError, s.t.Get("failed to restart SSH service: %v", err))
			return
		}
	}
	Success(w, nil)
}

// UpdatePort 修改 SSH 端口
func (s *ToolboxSSHService) UpdatePort(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ToolboxSSHPort](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.updateSSHConfig("Port", cast.ToString(req.Port)); err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("failed to update SSH port: %v", err))
		return
	}

	// 重启 SSH 服务
	if err = s.restartSSH(); err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("failed to restart SSH service: %v", err))
		return
	}

	Success(w, nil)
}

// UpdatePasswordAuth 设置密码认证
func (s *ToolboxSSHService) UpdatePasswordAuth(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ToolboxSSHPasswordAuth](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	value := "no"
	if req.Enabled {
		value = "yes"
	}

	if err = s.updateSSHConfig("PasswordAuthentication", value); err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("failed to update password authentication: %v", err))
		return
	}

	if err = s.restartSSH(); err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("failed to restart SSH service: %v", err))
		return
	}

	Success(w, nil)
}

// UpdatePubKeyAuth 设置密钥认证
func (s *ToolboxSSHService) UpdatePubKeyAuth(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ToolboxSSHPubKeyAuth](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	value := "no"
	if req.Enabled {
		value = "yes"
	}

	if err = s.updateSSHConfig("PubkeyAuthentication", value); err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("failed to update pubkey authentication: %v", err))
		return
	}

	if err = s.restartSSH(); err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("failed to restart SSH service: %v", err))
		return
	}

	Success(w, nil)
}

// UpdateRootLogin 设置 Root 登录
func (s *ToolboxSSHService) UpdateRootLogin(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ToolboxSSHRootLogin](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.updateSSHConfig("PermitRootLogin", req.Mode); err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("failed to update root login setting: %v", err))
		return
	}

	if err = s.restartSSH(); err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("failed to restart SSH service: %v", err))
		return
	}

	Success(w, nil)
}

// UpdateRootPassword 修改 Root 密码
func (s *ToolboxSSHService) UpdateRootPassword(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ToolboxSSHRootPassword](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	password := strings.ReplaceAll(req.Password, `'`, `\'`)
	if _, err = shell.Execf(`yes '%s' | passwd root`, password); err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("failed to update root password: %v", err))
		return
	}

	Success(w, nil)
}

// GetRootKey 获取 Root 私钥
func (s *ToolboxSSHService) GetRootKey(w http.ResponseWriter, r *http.Request) {
	var privateKey string

	// 优先尝试 ed25519 密钥
	if io.Exists("/root/.ssh/id_ed25519") {
		privateKey, _ = io.Read("/root/.ssh/id_ed25519")
	} else if io.Exists("/root/.ssh/id_rsa") {
		privateKey, _ = io.Read("/root/.ssh/id_rsa")
	}

	Success(w, strings.TrimSpace(privateKey))
}

// GenerateRootKey 生成 Root 密钥对
func (s *ToolboxSSHService) GenerateRootKey(w http.ResponseWriter, r *http.Request) {
	// 确保 .ssh 目录存在
	if _, err := shell.Execf("mkdir -p /root/.ssh && chmod 700 /root/.ssh"); err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("failed to create .ssh directory: %v", err))
		return
	}

	// 优先生成 ED25519 密钥对
	keyType := "ed25519"
	if _, err := shell.Execf(`yes 'y' | ssh-keygen -t ed25519 -f /root/.ssh/id_ed25519 -N ""`); err != nil {
		// 不行再生成 RSA 密钥
		keyType = "rsa"
		if _, err = shell.Execf(`yes 'y' | ssh-keygen -t rsa -b 4096 -f /root/.ssh/id_rsa -N ""`); err != nil {
			Error(w, http.StatusInternalServerError, s.t.Get("failed to generate SSH key: %v", err))
			return
		}
	}

	// 读取生成的密钥
	var pubKey, privateKey string
	var err error
	if keyType == "ed25519" {
		pubKey, err = io.Read("/root/.ssh/id_ed25519.pub")
		if err == nil {
			privateKey, _ = io.Read("/root/.ssh/id_ed25519")
		}
	} else {
		pubKey, err = io.Read("/root/.ssh/id_rsa.pub")
		if err == nil {
			privateKey, _ = io.Read("/root/.ssh/id_rsa")
		}
	}

	if err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("failed to read generated key: %v", err))
		return
	}

	// 将公钥添加到 authorized_keys
	pubKey = strings.TrimSpace(pubKey)
	privateKey = strings.TrimSpace(privateKey)
	authorizedKeysPath := "/root/.ssh/authorized_keys"
	authorizedKeys, _ := io.Read(authorizedKeysPath)

	// 检查公钥是否已存在
	if !strings.Contains(authorizedKeys, pubKey) {
		if authorizedKeys != "" && !strings.HasSuffix(authorizedKeys, "\n") {
			authorizedKeys += "\n"
		}
		authorizedKeys += pubKey + "\n"
		if err = io.Write(authorizedKeysPath, authorizedKeys, 0600); err != nil {
			Error(w, http.StatusInternalServerError, s.t.Get("failed to update authorized_keys: %v", err))
			return
		}
	}

	_ = s.restartSSH()

	Success(w, privateKey)
}

// updateSSHConfig 更新 SSH 配置项
func (s *ToolboxSSHService) updateSSHConfig(key, value string) error {
	sshdConfig, err := io.Read("/etc/ssh/sshd_config")
	if err != nil {
		return err
	}

	// 检查配置项是否存在（包括注释的）
	configRegex := regexp.MustCompile(`(?m)^#?\s*` + key + `\s+.*$`)

	if configRegex.MatchString(sshdConfig) {
		// 替换现有配置
		sshdConfig = configRegex.ReplaceAllString(sshdConfig, key+" "+value)
	} else {
		// 添加新配置
		sshdConfig = sshdConfig + "\n" + key + " " + value + "\n"
	}

	return io.Write("/etc/ssh/sshd_config", sshdConfig, 0600)
}

// restartSSH 重启 SSH 服务
func (s *ToolboxSSHService) restartSSH() error {
	err := systemctl.Restart("sshd")
	if err != nil {
		err = systemctl.Restart("ssh")
	}
	return err
}
