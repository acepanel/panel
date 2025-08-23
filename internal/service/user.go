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

	"github.com/gofiber/fiber/v3"
	"github.com/knadh/koanf/v2"
	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/sessions"
	"github.com/pquerna/otp/totp"
	"github.com/spf13/cast"

	"github.com/tnborg/panel/internal/biz"
	"github.com/tnborg/panel/internal/http/middleware"
	"github.com/tnborg/panel/internal/http/request"
	"github.com/tnborg/panel/pkg/rsacrypto"
)

type UserService struct {
	t        *gotext.Locale
	conf     *koanf.Koanf
	session  *sessions.Manager
	userRepo biz.UserRepo
}

func NewUserService(t *gotext.Locale, conf *koanf.Koanf, session *sessions.Manager, user biz.UserRepo) *UserService {
	gob.Register(rsa.PrivateKey{}) // 必须注册 rsa.PrivateKey 类型否则无法反序列化 session 中的 key
	return &UserService{
		t:        t,
		conf:     conf,
		session:  session,
		userRepo: user,
	}
}

func (s *UserService) GetKey(c fiber.Ctx) error {
	key, err := rsacrypto.GenerateKey()
	if err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	sess, err := middleware.GetSession(c)
	if err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}
	sess.Put("key", *key)

	pk, err := rsacrypto.PublicKeyToString(&key.PublicKey)
	if err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, pk)
}

func (s *UserService) Login(c fiber.Ctx) error {
	sess, err := middleware.GetSession(c)
	if err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	req, err := Bind[request.UserLogin](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	key, ok := sess.Get("key").(rsa.PrivateKey)
	if !ok {
		return Error(c, http.StatusForbidden, s.t.Get("invalid key, please refresh the page"))
	}

	decryptedUsername, _ := rsacrypto.DecryptData(&key, req.Username)
	decryptedPassword, _ := rsacrypto.DecryptData(&key, req.Password)
	user, err := s.userRepo.CheckPassword(string(decryptedUsername), string(decryptedPassword))
	if err != nil {
		return Error(c, http.StatusForbidden, "%v", err)
	}

	if user.TwoFA != "" {
		if valid := totp.Validate(req.PassCode, user.TwoFA); !valid {
			return Error(c, http.StatusForbidden, s.t.Get("invalid 2FA code"))
		}
	}

	// 安全登录下，将当前客户端与会话绑定
	// 安全登录只在未启用面板 HTTPS 时生效
	ip, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}
	if req.SafeLogin && !s.conf.Bool("http.tls") {
		sess.Put("safe_login", true)
		sess.Put("safe_client", fmt.Sprintf("%x", sha256.Sum256([]byte(ip))))
	} else {
		sess.Forget("safe_login")
		sess.Forget("safe_client")
	}

	sess.Put("user_id", user.ID)
	sess.Forget("key")
	return Success(c, nil)
}

func (s *UserService) Logout(c fiber.Ctx) error {
	sess, err := middleware.GetSession(c)
	if err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	sess.Forget("user_id")
	sess.Forget("key")
	sess.Forget("safe_login")
	sess.Forget("safe_client")

	return Success(c, nil)
}

func (s *UserService) IsLogin(c fiber.Ctx) error {
	sess, err := middleware.GetSession(c)
	if err != nil {
		return Success(c, false)
	}
	return Success(c, sess.Has("user_id"))
	return nil
}

func (s *UserService) IsTwoFA(c fiber.Ctx) error {
	req, err := Bind[request.UserIsTwoFA](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	twoFA, _ := s.userRepo.IsTwoFA(req.Username)
	return Success(c, twoFA)
}

func (s *UserService) Info(c fiber.Ctx) error {
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

	return Success(c, chix.M{
		"id":       user.ID,
		"role":     []string{"admin"},
		"username": user.Username,
		"email":    user.Email,
	})
}

func (s *UserService) List(c fiber.Ctx) error {
	req, err := Bind[request.Paginate](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	users, total, err := s.userRepo.List(req.Page, req.Limit)
	if err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, chix.M{
		"total": total,
		"items": users,
	})
}

func (s *UserService) Create(c fiber.Ctx) error {
	req, err := Bind[request.UserCreate](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	user, err := s.userRepo.Create(req.Username, req.Password, req.Email)
	if err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, user)
}

func (s *UserService) UpdateUsername(c fiber.Ctx) error {
	req, err := Bind[request.UserUpdateUsername](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if err = s.userRepo.UpdateUsername(req.ID, req.Username); err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, nil)
}

func (s *UserService) UpdatePassword(c fiber.Ctx) error {
	req, err := Bind[request.UserUpdatePassword](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if err = s.userRepo.UpdatePassword(req.ID, req.Password); err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, nil)
}

func (s *UserService) UpdateEmail(c fiber.Ctx) error {
	req, err := Bind[request.UserUpdateEmail](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if err = s.userRepo.UpdateEmail(req.ID, req.Email); err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, nil)
}

func (s *UserService) GenerateTwoFA(c fiber.Ctx) error {
	req, err := Bind[request.UserID](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	img, url, secret, err := s.userRepo.GenerateTwoFA(req.ID)
	if err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	buf := new(bytes.Buffer)
	if err = png.Encode(buf, img); err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, chix.M{
		"img":    base64.StdEncoding.EncodeToString(buf.Bytes()),
		"url":    url,
		"secret": secret,
	})
}

func (s *UserService) UpdateTwoFA(c fiber.Ctx) error {
	req, err := Bind[request.UserUpdateTwoFA](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if err = s.userRepo.UpdateTwoFA(req.ID, req.Code, req.Secret); err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, nil)
}

func (s *UserService) Delete(c fiber.Ctx) error {
	req, err := Bind[request.UserID](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if err = s.userRepo.Delete(req.ID); err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, nil)
}
