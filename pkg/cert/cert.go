package cert

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"slices"
	"strings"
	"time"

	"github.com/samber/lo"
)

func ParseCert(crt []byte) (x509.Certificate, error) {
	certBlock, _ := pem.Decode(crt)
	if certBlock == nil {
		return x509.Certificate{}, errors.New("invalid PEM block")
	}
	cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return x509.Certificate{}, err
	}

	return *cert, nil
}

// SANs 取证书的全部使用者可选名称，域名在前、IP 在后
func SANs(cert x509.Certificate) []string {
	return slices.Concat(cert.DNSNames, lo.Map(cert.IPAddresses, func(ip net.IP, _ int) string {
		return ip.String()
	}))
}

func ParseKey(key []byte) (crypto.Signer, error) {
	keyBlockDER, _ := pem.Decode(key)
	if keyBlockDER == nil {
		return nil, errors.New("invalid PEM block")
	}

	if keyBlockDER.Type != "PRIVATE KEY" && !strings.HasSuffix(keyBlockDER.Type, " PRIVATE KEY") {
		return nil, fmt.Errorf("unknown PEM header %q", keyBlockDER.Type)
	}

	if parse, err := x509.ParsePKCS1PrivateKey(keyBlockDER.Bytes); err == nil {
		return parse, nil
	}

	if parse, err := x509.ParsePKCS8PrivateKey(keyBlockDER.Bytes); err == nil {
		switch parsed := parse.(type) {
		case *rsa.PrivateKey:
			return parsed, nil
		case *ecdsa.PrivateKey:
			return parsed, nil
		case ed25519.PrivateKey:
			return parsed, nil
		default:
			return nil, fmt.Errorf("found unknown private key type in PKCS#8 wrapping: %T", parse)
		}
	}

	return x509.ParseECPrivateKey(keyBlockDER.Bytes)
}

// Match 判断私钥与证书是否成对
func Match(cert x509.Certificate, key crypto.Signer) bool {
	pub, ok := cert.PublicKey.(interface{ Equal(crypto.PublicKey) bool })
	return ok && pub.Equal(key.Public())
}

func EncodeKey(key crypto.Signer) ([]byte, error) {
	var pemType string
	var keyBytes []byte
	switch key := key.(type) {
	case *ecdsa.PrivateKey:
		var err error
		pemType = "EC"
		keyBytes, err = x509.MarshalECPrivateKey(key)
		if err != nil {
			return nil, err
		}
	case *rsa.PrivateKey:
		pemType = "RSA"
		keyBytes = x509.MarshalPKCS1PrivateKey(key)
	case ed25519.PrivateKey:
		var err error
		pemType = "ED25519"
		keyBytes, err = x509.MarshalPKCS8PrivateKey(key)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported key type %T", key)
	}
	pemKey := pem.Block{Type: pemType + " PRIVATE KEY", Bytes: keyBytes}
	return pem.EncodeToMemory(&pemKey), nil
}

// GenerateSelfSigned 生成自签名证书
func GenerateSelfSigned(names []string) (cert []byte, key []byte, err error) {
	// 1) 生成 ECDSA P-256 密钥对
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("generate ecdsa key: %w", err)
	}

	// 2) 解析 SAN
	var dnsNames []string
	var ipAddrs []net.IP
	for _, n := range names {
		if ip := net.ParseIP(n); ip != nil {
			ipAddrs = append(ipAddrs, ip)
		} else if n != "" {
			dnsNames = append(dnsNames, n)
		}
	}
	if len(dnsNames) == 0 && len(ipAddrs) == 0 {
		return nil, nil, errors.New("names is empty: SAN must not be empty")
	}

	// 3) 随机 128 位序列号
	serialLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serial, err := rand.Int(rand.Reader, serialLimit)
	if err != nil {
		return nil, nil, err
	}

	now := time.Now().Add(-5 * time.Minute) // 避免时钟偏差导致 not yet valid

	// 4) 组装证书模板
	tmpl := x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: "AcePanel"},
		NotBefore:    now,
		NotAfter:     now.AddDate(1, 0, 0), // 1 年

		DNSNames:    dnsNames,
		IPAddresses: ipAddrs,

		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &priv.PublicKey, priv)
	if err != nil {
		return nil, nil, err
	}

	// 5) 输出 PEM 证书
	cert = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})

	keyBytes, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return nil, nil, err
	}
	key = pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyBytes})

	return cert, key, nil
}
