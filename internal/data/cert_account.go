package data

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/leonelquinteros/gotext"
	"gorm.io/gorm"
	"resty.dev/v3"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/pkg/acme"
)

type certAccountRepo struct {
	t   *gotext.Locale
	db  *gorm.DB
	log *slog.Logger
}

func NewCertAccountRepo(db *gorm.DB, t *gotext.Locale, log *slog.Logger) biz.CertAccountRepo {
	return &certAccountRepo{
		t:   t,
		db:  db,
		log: log,
	}
}

func (r certAccountRepo) List(page, limit uint) ([]*biz.CertAccount, int64, error) {
	accounts := make([]*biz.CertAccount, 0)
	var total int64
	err := r.db.Model(&biz.CertAccount{}).Order("id desc").Count(&total).Offset(int((page - 1) * limit)).Limit(int(limit)).Find(&accounts).Error
	return accounts, total, err
}

func (r certAccountRepo) Get(id uint) (*biz.CertAccount, error) {
	account := new(biz.CertAccount)
	err := r.db.Model(&biz.CertAccount{}).Where("id = ?", id).First(account).Error
	return account, err
}

func (r certAccountRepo) GetByCAEmail(ca, email string) (*biz.CertAccount, error) {
	account := new(biz.CertAccount)
	err := r.db.Model(&biz.CertAccount{}).Where("ca = ?", ca).Where("email = ?", email).First(account).Error
	return account, err
}

func (r certAccountRepo) Create(account *biz.CertAccount) error {
	return r.db.Create(account).Error
}

func (r certAccountRepo) Save(account *biz.CertAccount) error {
	return r.db.Save(account).Error
}

func (r certAccountRepo) Delete(id uint) error {
	return r.db.Model(&biz.CertAccount{}).Where("id = ?", id).Delete(&biz.CertAccount{}).Error
}

// RegisterAccount 注册 ACME 账户
func (r certAccountRepo) RegisterAccount(
	ctx context.Context,
	email, ca string,
	eab *acme.EAB,
	keyType acme.KeyType,
) (*acme.Client, error) {
	return acme.NewRegisterAccount(ctx, email, ca, eab, keyType, r.log)
}

// GetGoogleEAB 获取 Google EAB
func (r certAccountRepo) GetGoogleEAB() (*acme.EAB, error) {
	type data struct {
		Msg  string `json:"msg"`
		Data struct {
			KeyId  string `json:"key_id"`
			MacKey string `json:"mac_key"`
		} `json:"data"`
	}
	client := resty.New()
	defer func(client *resty.Client) { _ = client.Close() }(client)
	client.SetTimeout(5 * time.Second)
	client.SetRetryCount(3)

	resp, err := client.R().SetResult(&data{}).Get("https://gts.rat.dev/eab")
	// 请求通了但状态码不对时 err 是 nil，报错里要带上状态
	if err == nil && !resp.IsStatusSuccess() {
		err = errors.New(resp.Status())
	}
	if err != nil {
		return &acme.EAB{}, errors.New(r.t.Get("failed to get Google EAB: %v", err))
	}
	eab, ok := resp.Result().(*data)
	if !ok {
		return &acme.EAB{}, errors.New(r.t.Get("failed to get Google EAB: %s", r.t.Get("invalid response")))
	}
	if eab.Msg != "success" {
		return &acme.EAB{}, errors.New(r.t.Get("failed to get Google EAB: %s", eab.Msg))
	}

	return &acme.EAB{KeyID: eab.Data.KeyId, MACKey: eab.Data.MacKey}, nil
}

// GetZeroSSLEAB 获取 ZeroSSL EAB
func (r certAccountRepo) GetZeroSSLEAB(email string) (*acme.EAB, error) {
	type data struct {
		Success    bool   `json:"success"`
		EabKid     string `json:"eab_kid"`
		EabHmacKey string `json:"eab_hmac_key"`
	}
	client := resty.New()
	defer func(client *resty.Client) { _ = client.Close() }(client)
	client.SetTimeout(5 * time.Second)
	client.SetRetryCount(3)

	resp, err := client.R().SetFormData(map[string]string{
		"email": email,
	}).SetResult(&data{}).Post("https://api.zerossl.com/acme/eab-credentials-email")
	// 请求通了但状态码不对时 err 是 nil，报错里要带上状态
	if err == nil && !resp.IsStatusSuccess() {
		err = errors.New(resp.Status())
	}
	if err != nil {
		return &acme.EAB{}, errors.New(r.t.Get("failed to get ZeroSSL EAB: %v", err))
	}
	eab, ok := resp.Result().(*data)
	if !ok {
		return &acme.EAB{}, errors.New(r.t.Get("failed to get ZeroSSL EAB"))
	}
	if !eab.Success {
		return &acme.EAB{}, errors.New(r.t.Get("failed to get ZeroSSL EAB"))
	}

	return &acme.EAB{KeyID: eab.EabKid, MACKey: eab.EabHmacKey}, nil
}
