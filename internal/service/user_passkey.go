package service

import (
	"crypto/x509"
	"encoding/gob"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/chix"
	"github.com/libtnb/sessions"
	"github.com/spf13/cast"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/http/request"
	"github.com/acepanel/panel/v3/pkg/config"
	"github.com/acepanel/panel/v3/pkg/passkey"
)

type UserPasskeyService struct {
	t               *gotext.Locale
	conf            *config.Config
	session         *sessions.Manager
	userPasskeyRepo biz.UserPasskeyRepo
	userRepo        biz.UserRepo
}

func NewUserPasskeyService(t *gotext.Locale, conf *config.Config, session *sessions.Manager, userPasskeyRepo biz.UserPasskeyRepo, userRepo biz.UserRepo) *UserPasskeyService {
	// 注册 webauthn.SessionData 类型，否则 session 无法序列化
	gob.Register(webauthn.SessionData{})
	return &UserPasskeyService{
		t:               t,
		conf:            conf,
		session:         session,
		userPasskeyRepo: userPasskeyRepo,
		userRepo:        userRepo,
	}
}

// Enabled 检查是否有任何已注册的通行密钥
func (s *UserPasskeyService) Enabled(w http.ResponseWriter, r *http.Request) {
	has, err := s.userPasskeyRepo.HasAny()
	if err != nil {
		Success(w, false)
		return
	}
	Success(w, has)
}

// Supported 检查面板是否满足通行密钥条件
func (s *UserPasskeyService) Supported(w http.ResponseWriter, r *http.Request) {
	// 面板自身开启 TLS 且证书可信，或者反代已做 TLS 终止
	if s.isCertTrusted() {
		Success(w, true)
		return
	}
	// 反代场景：面板本身可能是 HTTP，但客户端通过 HTTPS 访问
	if r.Header.Get("X-Forwarded-Proto") == "https" {
		Success(w, true)
		return
	}
	Success(w, false)
}

// BeginRegister 开始注册通行密钥
func (s *UserPasskeyService) BeginRegister(w http.ResponseWriter, r *http.Request) {
	sess, err := s.session.GetSession(r)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	userID := cast.ToUint(r.Context().Value("user_id"))
	if userID == 0 {
		ErrorSystem(w)
		return
	}

	u, err := s.userRepo.Get(userID)
	if err != nil {
		ErrorSystem(w)
		return
	}

	passkeys, err := s.userPasskeyRepo.List(userID)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	wa, err := passkey.NewWebAuthn(r)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	wUser := &passkey.User{Inner: u, Passkeys: passkeys}

	// 排除已有凭据
	var excludeCredentials []protocol.CredentialDescriptor
	for _, cred := range wUser.WebAuthnCredentials() {
		excludeCredentials = append(excludeCredentials, cred.Descriptor())
	}

	creation, sessionData, err := wa.BeginRegistration(
		wUser,
		webauthn.WithAuthenticatorSelection(protocol.AuthenticatorSelection{
			ResidentKey:      protocol.ResidentKeyRequirementRequired,
			UserVerification: protocol.VerificationRequired,
		}),
		webauthn.WithExclusions(excludeCredentials),
	)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	sess.Put("webauthn_register", *sessionData)
	Success(w, creation)
}

// FinishRegister 完成注册通行密钥
func (s *UserPasskeyService) FinishRegister(w http.ResponseWriter, r *http.Request) {
	sess, err := s.session.GetSession(r)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	userID := cast.ToUint(r.Context().Value("user_id"))
	if userID == 0 {
		ErrorSystem(w)
		return
	}

	sessionData, ok := sess.Get("webauthn_register").(webauthn.SessionData)
	if !ok {
		Error(w, http.StatusBadRequest, s.t.Get("invalid session, please try again"))
		return
	}
	sess.Forget("webauthn_register")

	u, err := s.userRepo.Get(userID)
	if err != nil {
		ErrorSystem(w)
		return
	}

	passkeys, err := s.userPasskeyRepo.List(userID)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	wa, err := passkey.NewWebAuthn(r)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	wUser := &passkey.User{Inner: u, Passkeys: passkeys}
	credential, err := wa.FinishRegistration(wUser, sessionData, r)
	if err != nil {
		Error(w, http.StatusBadRequest, s.t.Get("passkey registration failed: %v", err))
		return
	}

	// 从 query 参数提取 name（FinishRegistration 已消费 body）
	name := r.URL.Query().Get("name")
	if name == "" {
		name = s.t.Get("Passkey")
	}

	// 序列化 transports
	transports, err := json.Marshal(credential.Transport)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	pk := &biz.UserPasskey{
		UserID:       userID,
		Name:         name,
		CredentialID: credential.ID,
		PublicKey:    credential.PublicKey,
		AAGUID:       credential.Authenticator.AAGUID,
		SignCount:    credential.Authenticator.SignCount,
		Transports:   string(transports),
		BackupState:  credential.Flags.BackupState,
	}

	if err = s.userPasskeyRepo.Create(pk); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, pk)
}

// BeginLogin 开始通行密钥登录
func (s *UserPasskeyService) BeginLogin(w http.ResponseWriter, r *http.Request) {
	sess, err := s.session.GetSession(r)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	wa, err := passkey.NewWebAuthn(r)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	assertion, sessionData, err := wa.BeginDiscoverableLogin(
		webauthn.WithUserVerification(protocol.VerificationRequired),
	)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	sess.Put("webauthn_login", *sessionData)
	Success(w, assertion)
}

// FinishLogin 完成通行密钥登录
func (s *UserPasskeyService) FinishLogin(w http.ResponseWriter, r *http.Request) {
	sess, err := s.session.GetSession(r)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	sessionData, ok := sess.Get("webauthn_login").(webauthn.SessionData)
	if !ok {
		Error(w, http.StatusBadRequest, s.t.Get("invalid session, please try again"))
		return
	}
	sess.Forget("webauthn_login")

	wa, err := passkey.NewWebAuthn(r)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	// discoverable login handler：根据 userHandle 找到用户
	handler := func(rawID, userHandle []byte) (webauthn.User, error) {
		userID, err := passkey.ParseUserID(userHandle)
		if err != nil {
			return nil, err
		}
		u, err := s.userRepo.Get(userID)
		if err != nil {
			return nil, err
		}
		pks, err := s.userPasskeyRepo.List(userID)
		if err != nil {
			return nil, err
		}
		return &passkey.User{Inner: u, Passkeys: pks}, nil
	}

	returnedUser, credential, err := wa.FinishPasskeyLogin(handler, sessionData, r)
	if err != nil {
		Error(w, http.StatusUnauthorized, s.t.Get("passkey login failed: %v", err))
		return
	}

	// 更新 sign count 和最后使用时间
	_ = s.userPasskeyRepo.UpdateSignCount(credential.ID, credential.Authenticator.SignCount)
	_ = s.userPasskeyRepo.UpdateLastUsed(credential.ID)

	wUser := returnedUser.(*passkey.User)

	// 重新生成会话 ID
	if err = sess.Regenerate(true); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	// 设置登录状态
	sess.Put("user_id", wUser.Inner.ID)
	sess.Put("refresh_at", time.Now().Unix())
	// 通行密钥登录已经过设备验证，无需 safe_login
	sess.Forget("safe_login")
	sess.Forget("safe_client")

	Success(w, nil)
}

// List 列出指定用户的通行密钥
func (s *UserPasskeyService) List(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.UserPasskeyList](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	// 未指定 user_id 时默认查当前用户
	userID := req.UserID
	if userID == 0 {
		userID = cast.ToUint(r.Context().Value("user_id"))
	}
	if userID == 0 {
		ErrorSystem(w)
		return
	}

	passkeys, err := s.userPasskeyRepo.List(userID)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, chix.M{
		"items": passkeys,
	})
}

// Delete 删除指定通行密钥
func (s *UserPasskeyService) Delete(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.UserPasskeyDelete](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	// 未指定 user_id 时默认为当前用户
	userID := req.UserID
	if userID == 0 {
		userID = cast.ToUint(r.Context().Value("user_id"))
	}
	if userID == 0 {
		ErrorSystem(w)
		return
	}

	if err = s.userPasskeyRepo.Delete(userID, req.ID); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

// isCertTrusted 检查面板证书是否由 CA 签发（非自签名）
// 用证书自身公钥验签自身签名，成功说明是自签名证书
func (s *UserPasskeyService) isCertTrusted() bool {
	if !s.conf.HTTP.TLS {
		return false
	}

	certPath := filepath.Join(app.Root, "panel/storage/cert.pem")
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		return false
	}

	block, _ := pem.Decode(certPEM)
	if block == nil {
		return false
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return false
	}

	// 用自身公钥验证自身签名：成功 = 自签名，失败 = CA 签发
	return cert.CheckSignature(cert.SignatureAlgorithm, cert.RawTBSCertificate, cert.Signature) != nil
}
