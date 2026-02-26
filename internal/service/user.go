package service

import (
	"bytes"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/gob"
	"fmt"
	"image/png"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/dchest/captcha"
	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/chix"
	"github.com/libtnb/sessions"
	"github.com/pquerna/otp/totp"
	"github.com/spf13/cast"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/http/request"
	"github.com/acepanel/panel/v3/pkg/config"
	"github.com/acepanel/panel/v3/pkg/rsacrypto"
)

// 登录失败次数阈值，超过此次数需要验证码
const loginFailThreshold = 3

type UserService struct {
	t        *gotext.Locale
	conf     *config.Config
	session  *sessions.Manager
	userRepo biz.UserRepo
}

func NewUserService(t *gotext.Locale, conf *config.Config, session *sessions.Manager, user biz.UserRepo) *UserService {
	gob.Register(rsa.PrivateKey{}) // 必须注册 rsa.PrivateKey 类型否则无法反序列化 session 中的 key
	return &UserService{
		t:        t,
		conf:     conf,
		session:  session,
		userRepo: user,
	}
}

func (s *UserService) GetKey(w http.ResponseWriter, r *http.Request) {
	key, err := rsacrypto.GenerateKey()
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	sess, err := s.session.GetSession(r)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	sess.Put("key", *key)

	pk, err := rsacrypto.PublicKeyToString(&key.PublicKey)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, pk)
}

// GetCaptcha 获取登录验证码
func (s *UserService) GetCaptcha(w http.ResponseWriter, r *http.Request) {
	sess, err := s.session.GetSession(r)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	failCount := cast.ToInt(sess.Get("login_fail_count"))
	if !s.conf.HTTP.LoginCaptcha || failCount < loginFailThreshold {
		Success(w, chix.M{
			"required": false,
		})
		return
	}

	captchaID := captcha.NewLen(4)
	sess.Put("captcha_id", captchaID)

	var buf bytes.Buffer
	if err := captcha.WriteImage(&buf, captchaID, 150, 50); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, chix.M{
		"required": true,
		"image":    base64.StdEncoding.EncodeToString(buf.Bytes()),
	})
}

func (s *UserService) Login(w http.ResponseWriter, r *http.Request) {
	sess, err := s.session.GetSession(r)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	req, err := Bind[request.UserLogin](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	failCount := cast.ToInt(sess.Get("login_fail_count"))
	if s.conf.HTTP.LoginCaptcha && failCount >= loginFailThreshold {
		captchaID, ok := sess.Get("captcha_id").(string)
		if !ok || captchaID == "" || !captcha.VerifyString(captchaID, req.CaptchaCode) {
			Error(w, http.StatusForbidden, s.t.Get("invalid captcha code"))
			return
		}
		sess.Forget("captcha_id")
	}

	key, ok := sess.Get("key").(rsa.PrivateKey)
	if !ok {
		Error(w, http.StatusForbidden, s.t.Get("invalid key, please refresh the page"))
		return
	}

	decryptedUsername, _ := rsacrypto.DecryptData(&key, req.Username)
	decryptedPassword, _ := rsacrypto.DecryptData(&key, req.Password)
	user, err := s.userRepo.CheckPassword(string(decryptedUsername), string(decryptedPassword))
	if err != nil {
		sess.Put("login_fail_count", failCount+1)
		Error(w, http.StatusForbidden, "%v", err)
		return
	}

	if user.TwoFA != "" {
		if valid := totp.Validate(req.PassCode, user.TwoFA); !valid {
			Error(w, http.StatusForbidden, s.t.Get("invalid 2FA code"))
			return
		}
	}

	// 重新生成会话 ID
	if err = sess.Regenerate(true); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	// 安全登录下，将当前客户端与会话绑定
	// 安全登录只在未启用面板 HTTPS 时生效
	ip := r.RemoteAddr
	ipHeader := s.conf.HTTP.IPHeader
	if ipHeader != "" && r.Header.Get(ipHeader) != "" {
		ip = strings.Split(r.Header.Get(ipHeader), ",")[0]
	}
	ip, _, err = net.SplitHostPort(strings.TrimSpace(ip))
	if err != nil {
		ip = r.RemoteAddr
	}

	if req.SafeLogin && !s.conf.HTTP.TLS {
		sess.Put("safe_login", true)
		sess.Put("safe_client", fmt.Sprintf("%x", sha256.Sum256([]byte(ip))))
	} else {
		sess.Forget("safe_login")
		sess.Forget("safe_client")
	}

	sess.Put("user_id", user.ID)
	sess.Put("refresh_at", time.Now().Unix())
	sess.Forget("key")
	sess.Forget("login_fail_count")

	Success(w, nil)
}

func (s *UserService) Logout(w http.ResponseWriter, r *http.Request) {
	sess, err := s.session.GetSession(r)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
	}

	sess.Forget("user_id")
	sess.Forget("key")
	sess.Forget("safe_login")
	sess.Forget("safe_client")

	// 重新生成会话 ID
	if err = sess.Regenerate(true); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *UserService) IsLogin(w http.ResponseWriter, r *http.Request) {
	sess, err := s.session.GetSession(r)
	if err != nil {
		Success(w, false)
		return
	}
	Success(w, sess.Has("user_id"))
}

func (s *UserService) IsTwoFA(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.UserIsTwoFA](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	twoFA, _ := s.userRepo.IsTwoFA(req.Username)
	Success(w, twoFA)
}

func (s *UserService) Info(w http.ResponseWriter, r *http.Request) {
	userID := cast.ToUint(r.Context().Value("user_id"))
	if userID == 0 {
		ErrorSystem(w)
		return
	}

	user, err := s.userRepo.Get(userID)
	if err != nil {
		ErrorSystem(w)
		return
	}

	Success(w, chix.M{
		"id":       user.ID,
		"role":     []string{"admin"},
		"username": user.Username,
		"email":    user.Email,
	})
}

func (s *UserService) List(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.Paginate](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	users, total, err := s.userRepo.List(req.Page, req.Limit)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, chix.M{
		"total": total,
		"items": users,
	})
}

func (s *UserService) Create(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.UserCreate](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	user, err := s.userRepo.Create(r.Context(), req.Username, req.Password, req.Email)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, user)
}

func (s *UserService) UpdateUsername(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.UserUpdateUsername](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.userRepo.UpdateUsername(r.Context(), req.ID, req.Username); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *UserService) UpdatePassword(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.UserUpdatePassword](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.userRepo.UpdatePassword(r.Context(), req.ID, req.Password); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *UserService) UpdateEmail(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.UserUpdateEmail](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.userRepo.UpdateEmail(r.Context(), req.ID, req.Email); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *UserService) GenerateTwoFA(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.UserID](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	img, url, secret, err := s.userRepo.GenerateTwoFA(req.ID)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	buf := new(bytes.Buffer)
	if err = png.Encode(buf, img); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, chix.M{
		"img":    base64.StdEncoding.EncodeToString(buf.Bytes()),
		"url":    url,
		"secret": secret,
	})
}

func (s *UserService) UpdateTwoFA(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.UserUpdateTwoFA](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.userRepo.UpdateTwoFA(req.ID, req.Code, req.Secret); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *UserService) Delete(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.UserID](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.userRepo.Delete(r.Context(), req.ID); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}
