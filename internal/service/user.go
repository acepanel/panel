package service

import (
	"bytes"
	"crypto/rsa"
	"encoding/base64"
	"image/png"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/dchest/captcha"
	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/chix/v2"
	"github.com/pquerna/otp/totp"
	"github.com/spf13/cast"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/middleware"
	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/pkg/config"
	"github.com/acepanel/panel/v3/pkg/cookie"
	"github.com/acepanel/panel/v3/pkg/rsacrypto"
)

// 登录失败次数阈值，超过此次数需要验证码
const loginFailThreshold = 3

type UserService struct {
	t               *gotext.Locale
	log             *slog.Logger
	conf            *config.Config
	userRepo        *biz.UserUsecase
	userSessionRepo *biz.UserSessionUsecase
	notifyRepo      *biz.NotifyUsecase
	guard           *loginGuard
	key             func() (*rsa.PrivateKey, error) // 登录加密用的密钥，进程内只生成一把
	nonces          *pendingStore[struct{}]         // 已签发、尚未使用的登录随机数
}

func NewUserService(
	notifyUsecase *biz.NotifyUsecase,
	userSessionUsecase *biz.UserSessionUsecase,
	userUsecase *biz.UserUsecase,
	conf *config.Config,
	t *gotext.Locale,
	log *slog.Logger,
) *UserService {
	return &UserService{
		t:               t,
		log:             log,
		conf:            conf,
		userRepo:        userUsecase,
		userSessionRepo: userSessionUsecase,
		notifyRepo:      notifyUsecase,
		guard:           newLoginGuard(),
		key:             sync.OnceValues(rsacrypto.GenerateKey),
		nonces:          newPendingStore[struct{}](),
	}
}

// GetKey 获取登录公钥和一次性随机数
func (s *UserService) GetKey(w http.ResponseWriter, r *http.Request) {
	key, err := s.key()
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	pk, err := rsacrypto.PublicKeyToString(&key.PublicKey)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	nonce, ok := s.nonces.Put(struct{}{})
	if !ok {
		Error(w, http.StatusServiceUnavailable, s.t.Get("too many logins in progress, please try again later"))
		return
	}

	Success(w, chix.M{
		"key":   pk,
		"nonce": nonce,
	})
}

// GetCaptcha 获取登录验证码
func (s *UserService) GetCaptcha(w http.ResponseWriter, r *http.Request) {
	if !s.conf.HTTP.LoginCaptcha || s.guard.Count(clientIP(r, s.conf)) < loginFailThreshold {
		Success(w, chix.M{
			"required": false,
		})
		return
	}

	captchaID := captcha.NewLen(4)
	var buf bytes.Buffer
	if err := captcha.WriteImage(&buf, captchaID, 150, 50); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, chix.M{
		"required": true,
		"id":       captchaID,
		"image":    base64.StdEncoding.EncodeToString(buf.Bytes()),
	})
}

func (s *UserService) Login(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.UserLogin](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	ip := clientIP(r, s.conf)
	if s.conf.HTTP.LoginCaptcha && s.guard.Count(ip) >= loginFailThreshold {
		if !captcha.VerifyString(req.CaptchaID, req.CaptchaCode) {
			Error(w, http.StatusForbidden, s.t.Get("invalid captcha code"))
			return
		}
	}

	key, err := s.key()
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	decryptedUsername, _ := rsacrypto.DecryptData(key, req.Username)
	decryptedPassword, _ := rsacrypto.DecryptData(key, req.Password)

	// 密码密文里带着一次性随机数，截获的登录请求不能重放
	nonce, password, _ := strings.Cut(string(decryptedPassword), ":")
	if _, ok := s.nonces.Take(nonce); !ok {
		Error(w, http.StatusForbidden, s.t.Get("invalid key, please refresh the page"))
		return
	}

	user, err := s.userRepo.CheckPassword(string(decryptedUsername), password)
	if err != nil {
		s.loginFailed(r, ip, string(decryptedUsername))
		Error(w, http.StatusForbidden, "%v", err)
		return
	}

	// 2FA 失败同样计入，否则已知密码者可无限尝试验证码而不触发告警
	if user.TwoFA != "" {
		if valid := totp.Validate(req.PassCode, user.TwoFA); !valid {
			s.loginFailed(r, ip, string(decryptedUsername))
			Error(w, http.StatusForbidden, s.t.Get("invalid 2FA code"))
			return
		}
	}

	s.guard.Reset(ip)

	// 安全登录把会话绑定到当前 IP，只在未启用面板 HTTPS 时生效
	token, err := s.userSessionRepo.Create(user.ID, ip, r.UserAgent(), req.SafeLogin && !s.conf.HTTP.IsHTTPS())
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	cookie.Set(w, r, middleware.SessionCookie, token, s.conf.Session.TTL())

	s.notifyRepo.SendEvent(biz.NotifyEventLogin, s.t.Get("[AcePanel] Panel Login"), biz.NotifyBody(s.t.Get("panel login detected"), [][2]string{
		{s.t.Get("Username"), user.Username},
		{s.t.Get("IP"), ip},
		{s.t.Get("User Agent"), r.UserAgent()},
		{s.t.Get("Time"), time.Now().Format(time.DateTime)},
	}))

	s.log.Info(s.t.Get("user logged in"), slog.String("type", biz.OperationTypeUser), slog.Uint64("operator_id", uint64(user.ID)), slog.String("username", user.Username), slog.String("ip", ip), slog.String("method", "password"))

	Success(w, nil)
}

func (s *UserService) Logout(w http.ResponseWriter, r *http.Request) {
	token := cookie.Get(r, middleware.SessionCookie)
	ip := clientIP(r, s.conf)

	session, err := s.userSessionRepo.Authenticate(token, ip)
	if err == nil {
		if err = s.userSessionRepo.Delete(token); err != nil {
			Error(w, http.StatusInternalServerError, "%v", err)
			return
		}
		if user, err := s.userRepo.Get(session.UserID); err == nil {
			s.log.Info(s.t.Get("user logged out"), slog.String("type", biz.OperationTypeUser), slog.Uint64("operator_id", uint64(user.ID)), slog.String("username", user.Username), slog.String("ip", ip))
		}
	}
	cookie.Clear(w, r, middleware.SessionCookie)

	Success(w, nil)
}

func (s *UserService) IsLogin(w http.ResponseWriter, r *http.Request) {
	_, err := s.userSessionRepo.Authenticate(cookie.Get(r, middleware.SessionCookie), clientIP(r, s.conf))
	Success(w, err == nil)
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
	userID := uint(biz.OperatorID(r.Context()))
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

// loginFailed 记录一次登录失败，同来源短时间内失败过多时告警
func (s *UserService) loginFailed(r *http.Request, ip, username string) {
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
