package data

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/spf13/cast"

	"github.com/acepanel/panel/internal/biz"
	"github.com/acepanel/panel/pkg/firewall"
	"github.com/acepanel/panel/pkg/os"
	"github.com/acepanel/panel/pkg/shell"
	"github.com/acepanel/panel/pkg/systemctl"
)

type safeRepo struct {
	ssh string
	log *slog.Logger
}

func NewSafeRepo(log *slog.Logger) biz.SafeRepo {
	var ssh string
	if os.IsRHEL() {
		ssh = "sshd"
	} else {
		ssh = "ssh"
	}
	return &safeRepo{
		ssh: ssh,
		log: log,
	}
}

func (r *safeRepo) GetSSH() (uint, bool, error) {
	out, err := shell.Execf("cat /etc/ssh/sshd_config | grep 'Port ' | awk '{print $2}'")
	if err != nil {
		return 0, false, err
	}

	running, err := systemctl.Status(r.ssh)
	if err != nil {
		return 0, false, err
	}

	return cast.ToUint(out), running, nil
}

func (r *safeRepo) UpdateSSH(ctx context.Context, port uint, status bool) error {
	oldPort, err := shell.Execf("cat /etc/ssh/sshd_config | grep 'Port ' | awk '{print $2}'")
	if err != nil {
		return err
	}

	_, _ = shell.Execf("sed -i 's/#Port %s/Port %d/g' /etc/ssh/sshd_config", oldPort, port)
	_, _ = shell.Execf("sed -i 's/Port %s/Port %d/g' /etc/ssh/sshd_config", oldPort, port)

	if !status {
		if err = systemctl.Stop(r.ssh); err != nil {
			return err
		}
	} else {
		if err = systemctl.Restart(r.ssh); err != nil {
			return err
		}
	}

	// 记录日志
	r.log.Info("ssh settings updated", slog.String("type", biz.OperationTypeSafe), slog.Uint64("operator_id", getOperatorID(ctx)), slog.Uint64("port", uint64(port)), slog.Bool("status", status))

	return nil
}

func (r *safeRepo) GetPingStatus() (bool, error) {
	out, err := shell.Execf(`firewall-cmd --list-rich-rules`)
	if err != nil { // 可能防火墙已关闭等
		return true, nil
	}

	if !strings.Contains(out, `rule protocol value="icmp" drop`) {
		return true, nil
	}

	return false, nil
}

func (r *safeRepo) UpdatePingStatus(ctx context.Context, status bool) error {
	fw, err := firewall.NewFirewall().Status()
	if err != nil {
		return err
	}
	if !fw {
		return fmt.Errorf("failed to update ping status: firewalld is not running")
	}

	if status {
		_, err = shell.Execf(`firewall-cmd --permanent --remove-rich-rule='rule protocol value=icmp drop'`)
	} else {
		_, err = shell.Execf(`firewall-cmd --permanent --add-rich-rule='rule protocol value=icmp drop'`)
	}
	if err != nil {
		return err
	}

	_, err = shell.Execf(`firewall-cmd --reload`)
	if err != nil {
		return err
	}

	// 记录日志
	r.log.Info("ping status updated", slog.String("type", biz.OperationTypeSafe), slog.Uint64("operator_id", getOperatorID(ctx)), slog.Bool("status", status))

	return nil
}
