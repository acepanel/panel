package service

import (
	"bytes"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/gob"
	"fmt"
	"image/png"
	"log/slog"
	"net/http"
	"time"

	"github.com/dchest/captcha"
	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/chix/v2"
	"github.com/libtnb/sessions"
	"github.com/pquerna/otp/totp"
	"github.com/spf13/cast"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/pkg/config"
	"github.com/acepanel/panel/v3/pkg/rsacrypto"
)

// 登录失败次数阈值，超过此次数需要验证码
const loginFailThreshold = 3

type UserService struct {
	t          *gotext.Locale
	log        *slog.Logger
	conf       *config.Config
	session    *sessions.Manager
	userRepo   *biz.UserUsecase
	notifyRepo *biz.NotifyUsecase
	guard      *loginGuard
}

func NewUserService(notifyUsecase *biz.NotifyUsecase, userUsecase *biz.UserUsecase, conf *config.Config, t *gotext.Locale, log *slog.Logger, session *sessions.Manager) *UserService {
	// 必须注册 rsa.PrivateKey 类型否则无法反序列化 session 中的 key
	gob.Register(rsa.PrivateKey{})
	return &UserService{
		t:          t,
		log:        log,
		conf:       conf,
		session:    session,
		userRepo:   userUsecase,
		notifyRepo: notifyUsecase,
		guard:      newLoginGuard(),
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

	ip := clientIP(r, s.conf.HTTP.IPHeader)

	decryptedUsername, _ := rsacrypto.DecryptData(&key, req.Username)
	decryptedPassword, _ := rsacrypto.DecryptData(&key, req.Password)
	user, err := s.userRepo.CheckPassword(string(decryptedUsername), string(decryptedPassword))
	if err != nil {
		s.loginFailed(r, sess, ip, string(decryptedUsername), failCount)
		Error(w, http.StatusForbidden, "%v", err)
		return
	}

	// 2FA 失败同样计入，否则已知密码者可无限尝试验证码而不触发告警
	if user.TwoFA != "" {
		if valid := totp.Validate(req.PassCode, user.TwoFA); !valid {
			s.loginFailed(r, sess, ip, string(decryptedUsername), failCount)
			Error(w, http.StatusForbidden, s.t.Get("invalid 2FA code"))
			return
		}
	}

	s.guard.Reset(ip)

	// 重新生成会话 ID
	if err = sess.Regenerate(true); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	// 安全登录下，将当前客户端与会话绑定
	// 安全登录只在未启用面板 HTTPS 时生效
	if req.SafeLogin && !s.conf.HTTP.IsHTTPS() {
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

	s.notifyRepo.SendEvent(biz.NotifyEventLogin, s.t.Get("[AcePanel] Panel Login"), biz.NotifyBody(s.t.Get("panel login detected"), [][2]string{
		{s.t.Get("Username"), user.Username},
		{s.t.Get("IP"), ip},
		{s.t.Get("User Agent"), r.UserAgent()},
		{s.t.Get("Time"), time.Now().Format(time.DateTime)},
	}))

	s.log.Info(s.t.Get("user logged in"), slog.String("type", biz.OperationTypeUser), slog.Uint64("operator_id", uint64(user.ID)), slog.String("username", user.Username), slog.String("ip", ip), slog.String("method", "password"))

	Success(w, nil)
}

// loginFailed 记录一次登录失败，同来源短时间内失败过多时告警
func (s *UserService) loginFailed(r *http.Request, sess *sessions.Session, ip, username string, failCount int) {
	sess.Put("login_fail_count", failCount+1)

	s.log.Warn(s.t.Get("user login failed"), slog.String("type", biz.OperationTypeUser), slog.Uint64("operator_id", 0), slog.String("username", username), slog.String("ip", ip))

	count, exceeded := s.guard.Fail(ip)
	if !exceeded {
		return
	}

	s.notifyRepo.SendEvent(biz.NotifyEventLoginFailed, s.t.Get("[AcePanel] Suspicious Login Attempts"), biz.NotifyBody(s.t.Get("too many failed panel login attempts"), [][2]string{
		{s.t.Get("IP"), ip},
		{s.t.Get("Username"), username},
		{s.t.Get("Failed Attempts"), cast.ToString(count)},
		{s.t.Get("User Agent"), r.UserAgent()},
		{s.t.Get("Time"), time.Now().Format(time.DateTime)},
	}))
}

func (s *UserService) Logout(w http.ResponseWriter, r *http.Request) {
	sess, err := s.session.GetSession(r)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	userID := cast.ToUint(sess.Get("user_id"))
	sess.Forget("user_id")
	sess.Forget("key")
	sess.Forget("safe_login")
	sess.Forget("safe_client")

	// 重新生成会话 ID
	if err = sess.Regenerate(true); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	if user, err := s.userRepo.Get(userID); err == nil {
		s.log.Info(s.t.Get("user logged out"), slog.String("type", biz.OperationTypeUser), slog.Uint64("operator_id", uint64(user.ID)), slog.String("username", user.Username), slog.String("ip", clientIP(r, s.conf.HTTP.IPHeader)))
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

	users, total, err := s.userRepo.List(r.Context(), req.Page, req.Limit)
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

	if err = s.userRepo.UpdateTwoFA(r.Context(), req.ID, req.Code, req.Secret); err != nil {
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
