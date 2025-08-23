package service

import (
	"github.com/gofiber/fiber/v2"
	"net/http"

	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/chix"

	"github.com/tnborg/panel/internal/biz"
	"github.com/tnborg/panel/internal/http/request"
	"github.com/tnborg/panel/pkg/acme"
	"github.com/tnborg/panel/pkg/types"
)

type CertService struct {
	t        *gotext.Locale
	certRepo biz.CertRepo
}

func NewCertService(t *gotext.Locale, cert biz.CertRepo) *CertService {
	return &CertService{
		t:        t,
		certRepo: cert,
	}
}

func (s *CertService) CAProviders(c fiber.Ctx) error {
	return Success(c, []types.LV{
		{
			Label: "Let's Encrypt",
			Value: "letsencrypt",
		},
		{
			Label: "ZeroSSL",
			Value: "zerossl",
		},
		{
			Label: "SSL.com",
			Value: "sslcom",
		},
		{
			Label: "GoogleCN",
			Value: "googlecn",
		},
		{
			Label: "Google",
			Value: "google",
		},
		{
			Label: "Buypass",
			Value: "buypass",
		},
	})
}

func (s *CertService) DNSProviders(c fiber.Ctx) error {
	return Success(c, []types.LV{
		{
			Label: s.t.Get("Aliyun"),
			Value: string(acme.AliYun),
		},
		{
			Label: s.t.Get("Tencent Cloud"),
			Value: string(acme.Tencent),
		},
		{
			Label: s.t.Get("Huawei Cloud"),
			Value: string(acme.Huawei),
		},
		{
			Label: s.t.Get("West.cn"),
			Value: string(acme.Westcn),
		},
		{
			Label: s.t.Get("CloudFlare"),
			Value: string(acme.CloudFlare),
		},
		{
			Label: s.t.Get("Gcore"),
			Value: string(acme.Gcore),
		},
		{
			Label: s.t.Get("Porkbun"),
			Value: string(acme.Porkbun),
		},
		{
			Label: s.t.Get("NameSilo"),
			Value: string(acme.NameSilo),
		},
		{
			Label: s.t.Get("ClouDNS"),
			Value: string(acme.ClouDNS),
		},
		{
			Label: s.t.Get("Hetzner"),
			Value: string(acme.Hetzner),
		},
	})
}

func (s *CertService) Algorithms(c fiber.Ctx) error {
	return Success(c, []types.LV{
		{
			Label: "EC256",
			Value: string(acme.KeyEC256),
		},
		{
			Label: "EC384",
			Value: string(acme.KeyEC384),
		},
		{
			Label: "RSA2048",
			Value: string(acme.KeyRSA2048),
		},
		{
			Label: "RSA4096",
			Value: string(acme.KeyRSA4096),
		},
	})

}

func (s *CertService) List(c fiber.Ctx) error {
	req, err := Bind[request.Paginate](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	certs, total, err := s.certRepo.List(req.Page, req.Limit)
	if err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, chix.M{
		"total": total,
		"items": certs,
	})
}

func (s *CertService) Upload(c fiber.Ctx) error {
	req, err := Bind[request.CertUpload](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	cert, err := s.certRepo.Upload(req)
	if err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, cert)
}

func (s *CertService) Create(c fiber.Ctx) error {
	req, err := Bind[request.CertCreate](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	cert, err := s.certRepo.Create(req)
	if err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, cert)
}

func (s *CertService) Update(c fiber.Ctx) error {
	req, err := Bind[request.CertUpdate](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if err = s.certRepo.Update(req); err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, nil)
}

func (s *CertService) Get(c fiber.Ctx) error {
	req, err := Bind[request.ID](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	cert, err := s.certRepo.Get(req.ID)
	if err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, cert)
}

func (s *CertService) Delete(c fiber.Ctx) error {
	req, err := Bind[request.ID](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	err = s.certRepo.Delete(req.ID)
	if err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, nil)
}

func (s *CertService) ObtainAuto(c fiber.Ctx) error {
	req, err := Bind[request.ID](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if _, err = s.certRepo.ObtainAuto(req.ID); err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, nil)
}

func (s *CertService) ObtainManual(c fiber.Ctx) error {
	req, err := Bind[request.ID](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if _, err = s.certRepo.ObtainManual(req.ID); err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, nil)
}

func (s *CertService) ObtainSelfSigned(c fiber.Ctx) error {
	req, err := Bind[request.ID](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if err = s.certRepo.ObtainSelfSigned(req.ID); err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, nil)
}

func (s *CertService) Renew(c fiber.Ctx) error {
	req, err := Bind[request.ID](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	_, err = s.certRepo.Renew(req.ID)
	if err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, nil)
}

func (s *CertService) ManualDNS(c fiber.Ctx) error {
	req, err := Bind[request.ID](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	dns, err := s.certRepo.ManualDNS(req.ID)
	if err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, dns)
}

func (s *CertService) Deploy(c fiber.Ctx) error {
	req, err := Bind[request.CertDeploy](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	err = s.certRepo.Deploy(req.ID, req.WebsiteID)
	if err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, nil)
}
