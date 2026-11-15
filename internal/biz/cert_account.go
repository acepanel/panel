package biz

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"time"

	"github.com/leonelquinteros/gotext"

	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/pkg/acme"
	"github.com/acepanel/panel/v3/pkg/cert"
)

type CertAccount struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Email       string    `gorm:"not null;default:''" json:"email"`
	CA          string    `gorm:"not null;default:'letsencrypt'" json:"ca"` // CA 提供商 (letsencrypt, zerossl, sslcom, google)
	Kid         string    `gorm:"not null;default:''" json:"kid"`
	HmacEncoded string    `gorm:"not null;default:''" json:"hmac_encoded"`
	PrivateKey  string    `gorm:"not null;default:''" json:"private_key"`
	KeyType     string    `gorm:"not null;default:'P256'" json:"key_type"` // 密钥类型 (P256, P384, 2048, 3072, 4096)
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	Certs []*Cert `gorm:"foreignKey:AccountID" json:"-"`
}

// Directory 取账户所属 CA 的 ACME 目录地址，CA 不受支持时为空
func (r *CertAccount) Directory() string {
	// caDirectories CA 标识到 ACME 目录地址的映射
	caDirectories := map[string]string{
		"googlecn":    acme.CAGoogleCN,
		"google":      acme.CAGoogle,
		"letsencrypt": acme.CALetsEncrypt,
		"litessl":     acme.CALiteSSL,
		"zerossl":     acme.CAZeroSSL,
		"sslcom":      acme.CASSLcom,
	}
	return caDirectories[r.CA]
}

// EAB 取回已注册账户时要带的 EAB 凭据，不需要时为 nil
func (r *CertAccount) EAB() *acme.EAB {
	if !slices.Contains([]string{"googlecn", "google", "zerossl", "sslcom"}, r.CA) {
		return nil
	}
	return &acme.EAB{KeyID: r.Kid, MACKey: r.HmacEncoded}
}

type CertAccountRepo interface {
	List(page, limit uint) ([]*CertAccount, int64, error)
	Get(id uint) (*CertAccount, error)
	GetByCAEmail(ca, email string) (*CertAccount, error)
	Create(account *CertAccount) error
	Save(account *CertAccount) error
	Delete(id uint) error
	GetGoogleEAB() (*acme.EAB, error)
	GetZeroSSLEAB(email string) (*acme.EAB, error)
	RegisterAccount(ctx context.Context, email, ca string, eab *acme.EAB, keyType acme.KeyType) (*acme.Client, error)
}

type CertAccountUsecase struct {
	repo     CertAccountRepo
	userRepo UserRepo
	t        *gotext.Locale
	log      *slog.Logger
}

func NewCertAccountUsecase(
	t *gotext.Locale,
	log *slog.Logger,
	certAccountRepo CertAccountRepo,
	userRepo UserRepo,
) *CertAccountUsecase {
	return &CertAccountUsecase{
		repo:     certAccountRepo,
		userRepo: userRepo,
		t:        t,
		log:      log,
	}
}

func (uc *CertAccountUsecase) List(page, limit uint) ([]*CertAccount, int64, error) {
	return uc.repo.List(page, limit)
}

func (uc *CertAccountUsecase) Get(id uint) (*CertAccount, error) {
	return uc.repo.Get(id)
}

func (uc *CertAccountUsecase) GetDefault(ctx context.Context, userID uint) (*CertAccount, error) {
	user, err := uc.userRepo.Get(userID)
	if err != nil {
		return nil, err
	}

	account, err := uc.repo.GetByCAEmail("letsencrypt", user.Email)
	if err == nil {
		return account, nil
	}

	req := &request.CertAccountCreate{
		CA:      "letsencrypt",
		Email:   user.Email,
		KeyType: string(acme.KeyEC256),
	}

	return uc.Create(ctx, req)
}

func (uc *CertAccountUsecase) Create(ctx context.Context, req *request.CertAccountCreate) (*CertAccount, error) {
	account := new(CertAccount)
	account.CA = req.CA
	account.Email = req.Email
	account.Kid = req.Kid
	account.HmacEncoded = req.HmacEncoded
	account.KeyType = req.KeyType

	if err := uc.register(ctx, account); err != nil {
		return nil, err
	}

	if err := uc.repo.Create(account); err != nil {
		return nil, err
	}

	// 记录日志
	uc.log.Info(uc.t.Get("cert account created"), slog.String("type", OperationTypeCert), slog.Uint64("operator_id", OperatorID(ctx)), slog.Uint64("id", uint64(account.ID)), slog.String("ca", req.CA), slog.String("email", req.Email))

	return account, nil
}

func (uc *CertAccountUsecase) Update(ctx context.Context, req *request.CertAccountUpdate) error {
	account, err := uc.repo.Get(req.ID)
	if err != nil {
		return err
	}

	account.CA = req.CA
	account.Email = req.Email
	account.Kid = req.Kid
	account.HmacEncoded = req.HmacEncoded
	account.KeyType = req.KeyType

	if err = uc.register(ctx, account); err != nil {
		return err
	}

	if err = uc.repo.Save(account); err != nil {
		return err
	}

	// 记录日志
	uc.log.Info(uc.t.Get("cert account updated"), slog.String("type", OperationTypeCert), slog.Uint64("operator_id", OperatorID(ctx)), slog.Uint64("id", uint64(req.ID)), slog.String("ca", req.CA), slog.String("email", req.Email), slog.String("key_type", req.KeyType))

	return nil
}

func (uc *CertAccountUsecase) Delete(ctx context.Context, id uint) error {
	account, err := uc.repo.Get(id)
	if err != nil {
		return err
	}

	if err = uc.repo.Delete(id); err != nil {
		return err
	}

	// 记录日志
	uc.log.Warn(uc.t.Get("cert account deleted"), slog.String("type", OperationTypeCert), slog.Uint64("operator_id", OperatorID(ctx)), slog.Uint64("id", uint64(id)), slog.String("email", account.Email), slog.String("ca", account.CA))

	return nil
}

// register 向 CA 注册账户并把账户私钥写回 account
func (uc *CertAccountUsecase) register(ctx context.Context, account *CertAccount) error {
	directory := account.Directory()
	if directory == "" {
		return errors.New(uc.t.Get("unsupported CA"))
	}

	// letsencrypt 不需要 EAB，googlecn 与 zerossl 的由接口自动申请，其余由用户填写
	var eab *acme.EAB
	var err error
	switch account.CA {
	case "letsencrypt":
	case "googlecn":
		eab, err = uc.repo.GetGoogleEAB()
	case "zerossl":
		eab, err = uc.repo.GetZeroSSLEAB(account.Email)
	default:
		eab = &acme.EAB{KeyID: account.Kid, MACKey: account.HmacEncoded}
	}
	if err != nil {
		return err
	}
	// 自动申请到的凭据要随账户保存
	if eab != nil {
		account.Kid = eab.KeyID
		account.HmacEncoded = eab.MACKey
	}

	client, err := uc.repo.RegisterAccount(ctx, account.Email, directory, eab, acme.KeyType(account.KeyType))
	if err != nil {
		return errors.New(uc.t.Get("failed to register account: %v", err))
	}

	privateKey, err := cert.EncodeKey(client.Account.PrivateKey)
	if err != nil {
		return errors.New(uc.t.Get("failed to get private key: %v", err))
	}
	account.PrivateKey = string(privateKey)

	return nil
}
