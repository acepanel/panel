package frp

import (
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/pkg/io"
)

const (
	proxyPrefix   = "proxy"
	visitorPrefix = "visitor"
)

func (s *App) confPath(name string) string {
	return filepath.Join(app.Root, "server", "frp", name+".toml")
}

func (s *App) confDDir() string {
	return filepath.Join(app.Root, "server", "frp", "conf.d")
}

func (s *App) confDGlob() string {
	return filepath.Join(s.confDDir(), "*.toml")
}

func (s *App) itemPath(prefix, name string) string {
	return filepath.Join(s.confDDir(), prefix+"-"+name+".toml")
}

func (s *App) readConfD(path string) (ConfD, error) {
	raw, err := io.Read(path)
	if err != nil {
		return ConfD{}, err
	}

	var confD ConfD
	return confD, toml.Unmarshal([]byte(raw), &confD)
}

func (s *App) listConfD() (ConfD, error) {
	var all ConfD

	// Glob 在目录不存在时返回空，无需额外判断
	paths, err := filepath.Glob(s.confDGlob())
	if err != nil {
		return all, err
	}

	for _, path := range paths {
		confD, err := s.readConfD(path)
		if err != nil {
			return all, err
		}

		all.Proxies = append(all.Proxies, confD.Proxies...)
		all.Visitors = append(all.Visitors, confD.Visitors...)
	}

	return all, nil
}

func (s *App) writeConfD(prefix, name string, confD ConfD) error {
	if err := os.MkdirAll(s.confDDir(), 0755); err != nil {
		return err
	}

	raw, err := toml.Marshal(confD)
	if err != nil {
		return err
	}

	return io.Write(s.itemPath(prefix, name), string(raw), 0644)
}
