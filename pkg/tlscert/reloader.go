package tlscert

import (
	"crypto/tls"
	"fmt"
	"os"
	"sync"
	"time"
)

// Reloader 证书热重载器，通过 GetCertificate 回调在每次 TLS 握手时
// 检测证书文件是否变更，如果变更且合法则自动加载新证书，无需重启服务器
type Reloader struct {
	certFile string
	keyFile  string

	mu          sync.RWMutex
	cert        *tls.Certificate
	certModTime time.Time
	keyModTime  time.Time
}

// NewReloader 创建证书热重载器
func NewReloader(certFile, keyFile string) (*Reloader, error) {
	r := &Reloader{
		certFile: certFile,
		keyFile:  keyFile,
	}
	if err := r.loadCert(); err != nil {
		return nil, err
	}
	return r, nil
}

// GetCertificate 作为 tls.Config.GetCertificate 回调
// 每次 TLS 握手时检测证书文件修改时间，变更则热重载
func (r *Reloader) GetCertificate(_ *tls.ClientHelloInfo) (*tls.Certificate, error) {
	certInfo, certErr := os.Stat(r.certFile)
	keyInfo, keyErr := os.Stat(r.keyFile)
	if certErr != nil || keyErr != nil {
		r.mu.RLock()
		defer r.mu.RUnlock()
		return r.cert, nil
	}

	r.mu.RLock()
	needReload := !certInfo.ModTime().Equal(r.certModTime) || !keyInfo.ModTime().Equal(r.keyModTime)
	r.mu.RUnlock()

	if needReload {
		if err := r.loadCert(); err != nil {
			fmt.Println("[TLS] certificate reload failed, keeping current certificate:", err)
		} else {
			fmt.Println("[TLS] certificate reloaded successfully")
		}
	}

	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.cert, nil
}

// loadCert 从文件加载并验证证书
func (r *Reloader) loadCert() error {
	cert, err := tls.LoadX509KeyPair(r.certFile, r.keyFile)
	if err != nil {
		return err
	}

	certInfo, err := os.Stat(r.certFile)
	if err != nil {
		return err
	}
	keyInfo, err := os.Stat(r.keyFile)
	if err != nil {
		return err
	}

	r.mu.Lock()
	r.cert = &cert
	r.certModTime = certInfo.ModTime()
	r.keyModTime = keyInfo.ModTime()
	r.mu.Unlock()

	return nil
}
