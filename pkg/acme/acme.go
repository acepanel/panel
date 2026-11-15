package acme

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"log/slog"
	"net/http"

	"github.com/mholt/acmez/v3"
	"github.com/mholt/acmez/v3/acme"

	"github.com/acepanel/panel/v3/pkg/cert"
)

const (
	CAGoogleCN           = "https://gts.rat.dev/directory"
	CAGoogle             = "https://dv.acme-v02.api.pki.goog/directory"
	CALetsEncryptStaging = "https://acme-staging-v02.api.letsencrypt.org/directory"
	CALetsEncrypt        = "https://acme-v02.api.letsencrypt.org/directory"
	CALiteSSL            = "https://acme.litessl.com/acme/v2/directory"
	CAZeroSSL            = "https://acme.zerossl.com/v2/DV90"
	CASSLcom             = "https://acme.ssl.com/sslcom-dv-rsa"
)

type KeyType string

const (
	KeyEC256   = KeyType("P256")
	KeyEC384   = KeyType("P384")
	KeyRSA2048 = KeyType("2048")
	KeyRSA3072 = KeyType("3072")
	KeyRSA4096 = KeyType("4096")
)

type EAB = acme.EAB

func NewRegisterAccount(
	ctx context.Context,
	email, ca string,
	eab *EAB,
	keyType KeyType,
	log *slog.Logger,
) (*Client, error) {
	client := getClient(ca, log)

	accountPrivateKey, err := generatePrivateKey(keyType)
	if err != nil {
		return nil, err
	}
	account := acme.Account{
		Contact:              []string{"mailto:" + email},
		TermsOfServiceAgreed: true,
		PrivateKey:           accountPrivateKey,
	}
	if eab != nil {
		err = account.SetExternalAccountBinding(ctx, client.Client, *eab)
		if err != nil {
			return nil, err
		}
	}

	account, err = client.NewAccount(ctx, account)
	if err != nil {
		return nil, err
	}

	return &Client{Account: account, zClient: client}, nil
}

func NewPrivateKeyAccount(
	ctx context.Context,
	email, privateKey, ca string,
	eab *EAB,
	log *slog.Logger,
) (*Client, error) {
	client := getClient(ca, log)

	key, err := cert.ParseKey([]byte(privateKey))
	if err != nil {
		return nil, err
	}

	account := acme.Account{
		Contact:              []string{"mailto:" + email},
		TermsOfServiceAgreed: true,
		PrivateKey:           key,
	}
	if eab != nil {
		err = account.SetExternalAccountBinding(ctx, client.Client, *eab)
		if err != nil {
			return nil, err
		}
	}

	account, err = client.GetAccount(ctx, account)
	if err != nil {
		return nil, err
	}

	return &Client{Account: account, zClient: client}, nil
}

func generatePrivateKey(keyType KeyType) (crypto.Signer, error) {
	switch keyType {
	case KeyEC256:
		return ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	case KeyEC384:
		return ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	case KeyRSA2048:
		return rsa.GenerateKey(rand.Reader, 2048)
	case KeyRSA3072:
		return rsa.GenerateKey(rand.Reader, 3072)
	case KeyRSA4096:
		return rsa.GenerateKey(rand.Reader, 4096)
	}

	return nil, errors.New("unsupported key type")
}

func getClient(ca string, log *slog.Logger) acmez.Client {
	return acmez.Client{
		Client: &acme.Client{
			Directory:  ca,
			HTTPClient: http.DefaultClient,
			Logger:     log,
		},
	}
}
