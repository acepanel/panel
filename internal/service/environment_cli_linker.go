package service

import "github.com/acepanel/panel/v3/pkg/shell"

// linkCLIBinaries 将指定二进制文件软链接到 /usr/local/bin。
func linkCLIBinaries(binPath string, binaries []string) error {
	for _, bin := range binaries {
		if _, err := shell.Execf("ln -sf %s/%s /usr/local/bin/%s", binPath, bin, bin); err != nil {
			return err
		}
	}

	return nil
}
