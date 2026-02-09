package tlscert

import (
	"crypto/tls"
	"fmt"
	"path/filepath"
	"sync"

	"github.com/fsnotify/fsnotify"
)

// Reloader 证书热重载器
type Reloader struct {
	certFile string
	keyFile  string

	mu   sync.RWMutex
	cert *tls.Certificate

	watcher *fsnotify.Watcher
	done    chan struct{}
}

func NewReloader(certFile, keyFile string) (*Reloader, error) {
	r := &Reloader{
		certFile: certFile,
		keyFile:  keyFile,
		done:     make(chan struct{}),
	}
	if err := r.loadCert(); err != nil {
		return nil, err
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create file watcher: %w", err)
	}
	r.watcher = watcher

	certDir := filepath.Dir(certFile)
	if err = watcher.Add(certDir); err != nil {
		_ = watcher.Close()
		return nil, fmt.Errorf("failed to watch directory %s: %w", certDir, err)
	}

	go r.watch()

	return r, nil
}

func (r *Reloader) GetCertificate(_ *tls.ClientHelloInfo) (*tls.Certificate, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.cert, nil
}

func (r *Reloader) Close() error {
	close(r.done)
	return r.watcher.Close()
}

// watch 监听文件系统事件，检测到证书文件变更时自动重载
func (r *Reloader) watch() {
	certBase := filepath.Base(r.certFile)
	keyBase := filepath.Base(r.keyFile)
	for {
		select {
		case event, ok := <-r.watcher.Events:
			if !ok {
				return
			}
			// 仅关注证书文件的写入或创建事件
			name := filepath.Base(event.Name)
			if name != certBase && name != keyBase {
				continue
			}
			if !event.Has(fsnotify.Write) && !event.Has(fsnotify.Create) {
				continue
			}
			if err := r.loadCert(); err == nil {
				fmt.Println("[TLS] certificate reloaded successfully")
			}
		case err, ok := <-r.watcher.Errors:
			if !ok {
				return
			}
			fmt.Println("[TLS] file watcher error:", err)
		case <-r.done:
			return
		}
	}
}

// loadCert 从文件加载并验证证书
func (r *Reloader) loadCert() error {
	cert, err := tls.LoadX509KeyPair(r.certFile, r.keyFile)
	if err != nil {
		return err
	}

	r.mu.Lock()
	r.cert = &cert
	r.mu.Unlock()

	return nil
}
