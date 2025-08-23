package service

import (
	"github.com/gofiber/fiber/v3"
	"net/http"
	"time"

	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/chix"

	"github.com/tnborg/panel/internal/biz"
	"github.com/tnborg/panel/internal/http/request"
)

type UserTokenService struct {
	t             *gotext.Locale
	userTokenRepo biz.UserTokenRepo
}

func NewUserTokenService(t *gotext.Locale, userToken biz.UserTokenRepo) *UserTokenService {
	return &UserTokenService{
		t:             t,
		userTokenRepo: userToken,
	}
}

func (s *UserTokenService) List(c fiber.Ctx) error {
	req, err := Bind[request.UserTokenList](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	userTokens, total, err := s.userTokenRepo.List(req.UserID, req.Page, req.Limit)
	if err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, chix.M{
		"total": total,
		"items": userTokens,
	})
}

func (s *UserTokenService) Create(c fiber.Ctx) error {
	req, err := Bind[request.UserTokenCreate](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	expiredAt := time.Unix(0, req.ExpiredAt*int64(time.Millisecond))
	if expiredAt.Before(time.Now()) {
		return Error(c, http.StatusUnprocessableEntity, s.t.Get("expiration time must be greater than current time"))
	}
	if expiredAt.After(time.Now().AddDate(10, 0, 0)) {
		return Error(c, http.StatusUnprocessableEntity, s.t.Get("expiration time must be less than 10 years"))
	}

	userToken, err := s.userTokenRepo.Create(req.UserID, req.IPs, expiredAt)
	if err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	// 手动组装响应，因为 Token 设置了 json:"-"
	return Success(c, chix.M{
		"id":         userToken.ID,
		"user_id":    userToken.UserID,
		"token":      userToken.Token,
		"ips":        userToken.IPs,
		"expired_at": userToken.ExpiredAt,
		"created_at": userToken.CreatedAt,
		"updated_at": userToken.UpdatedAt,
	})
}

func (s *UserTokenService) Update(c fiber.Ctx) error {
	req, err := Bind[request.UserTokenUpdate](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	expiredAt := time.Unix(0, req.ExpiredAt*int64(time.Millisecond))
	if expiredAt.Before(time.Now()) {
		return Error(c, http.StatusUnprocessableEntity, s.t.Get("expiration time must be greater than current time"))
	}
	if expiredAt.After(time.Now().AddDate(10, 0, 0)) {
		return Error(c, http.StatusUnprocessableEntity, s.t.Get("expiration time must be less than 10 years"))
	}

	userToken, err := s.userTokenRepo.Update(req.ID, req.IPs, expiredAt)
	if err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, userToken)
}

func (s *UserTokenService) Delete(c fiber.Ctx) error {
	req, err := Bind[request.ID](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if err = s.userTokenRepo.Delete(req.ID); err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, nil)
}
