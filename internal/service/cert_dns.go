package service

import (
	"github.com/gofiber/fiber/v3"
	"net/http"

	"github.com/libtnb/chix"

	"github.com/tnborg/panel/internal/biz"
	"github.com/tnborg/panel/internal/http/request"
)

type CertDNSService struct {
	certDNSRepo biz.CertDNSRepo
}

func NewCertDNSService(certDNS biz.CertDNSRepo) *CertDNSService {
	return &CertDNSService{
		certDNSRepo: certDNS,
	}
}

func (s *CertDNSService) List(c fiber.Ctx) error {
	req, err := Bind[request.Paginate](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	certDNS, total, err := s.certDNSRepo.List(req.Page, req.Limit)
	if err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, chix.M{
		"total": total,
		"items": certDNS,
	})
}

func (s *CertDNSService) Create(c fiber.Ctx) error {
	req, err := Bind[request.CertDNSCreate](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	certDNS, err := s.certDNSRepo.Create(req)
	if err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, certDNS)
}

func (s *CertDNSService) Update(c fiber.Ctx) error {
	req, err := Bind[request.CertDNSUpdate](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if err = s.certDNSRepo.Update(req); err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, nil)
}

func (s *CertDNSService) Get(c fiber.Ctx) error {
	req, err := Bind[request.ID](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	certDNS, err := s.certDNSRepo.Get(req.ID)
	if err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, certDNS)
}

func (s *CertDNSService) Delete(c fiber.Ctx) error {
	req, err := Bind[request.ID](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if err = s.certDNSRepo.Delete(req.ID); err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, nil)
}
