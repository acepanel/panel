//go:build linux

package tamper

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"golang.org/x/sys/unix"

	"github.com/acepanel/panel/v3/pkg/shell"
)

var grubCmdlineRe = regexp.MustCompile(`(?m)^(GRUB_CMDLINE_LINUX(?:_DEFAULT)?=)"([^"]*)"`)

// Supported 当前平台是否支持防篡改(Linux 即支持 chattr 模式)
func Supported() bool {
	return true
}

func DetectEBPF() EBPFStatus {
	st := EBPFStatus{
		KernelVersion: kernelVersion(),
		ActiveLSM:     activeLSM(),
	}

	// bpf 必须在激活的 LSM 列表中
	for l := range strings.SplitSeq(st.ActiveLSM, ",") {
		if strings.TrimSpace(l) == "bpf" {
			st.BPFLSMActive = true
			break
		}
	}
	if !st.BPFLSMActive {
		st.Reason = "bpf LSM not active, append lsm=...,bpf to kernel cmdline and reboot"
		return st
	}

	st.Available = true
	return st
}

// EnableBPFLSMGrub 修改 grub 激活 bpf LSM,需重启系统生效
func EnableBPFLSMGrub(ctx context.Context) error {
	const grubFile = "/etc/default/grub"
	data, err := os.ReadFile(grubFile)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", grubFile, err)
	}

	newContent, changed := injectLSMBpf(string(data), activeLSM())
	if changed {
		if err = os.WriteFile(grubFile, []byte(newContent), 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", grubFile, err)
		}
	}

	if err = regenerateGrub(ctx); err != nil {
		return fmt.Errorf("failed to regenerate grub config: %w", err)
	}
	return nil
}

func kernelVersion() string {
	var uts unix.Utsname
	if err := unix.Uname(&uts); err != nil {
		return ""
	}
	return string(uts.Release[:bytes.IndexByte(uts.Release[:], 0)])
}

func activeLSM() string {
	data, err := os.ReadFile("/sys/kernel/security/lsm")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// injectLSMBpf 在 grub 内核命令行中注入 bpf LSM
// 已有 lsm= 则向其追加 bpf,否则新增 lsm=<当前激活列表>,bpf
func injectLSMBpf(content, active string) (string, bool) {
	changed := false
	out := grubCmdlineRe.ReplaceAllStringFunc(content, func(line string) string {
		m := grubCmdlineRe.FindStringSubmatch(line)
		prefix, value := m[1], m[2]

		if idx := regexp.MustCompile(`(?:^|\s)lsm=([^\s"]*)`).FindStringSubmatchIndex(value); idx != nil {
			lsmVal := value[idx[2]:idx[3]]
			for l := range strings.SplitSeq(lsmVal, ",") {
				if strings.TrimSpace(l) == "bpf" {
					return line // 已含 bpf
				}
			}
			newVal := value[:idx[2]] + lsmVal + ",bpf" + value[idx[3]:]
			changed = true
			return prefix + `"` + newVal + `"`
		}

		// 无 lsm= 项,追加
		lsmList := active
		if lsmList == "" {
			lsmList = "landlock,lockdown,yama,integrity,apparmor,bpf"
		} else if !strings.Contains(lsmList, "bpf") {
			lsmList += ",bpf"
		}
		newVal := strings.TrimSpace(value + " lsm=" + lsmList)
		changed = true
		return prefix + `"` + newVal + `"`
	})
	return out, changed
}

// regenerateGrub 重新生成 grub 配置(兼容各发行版路径)
func regenerateGrub(ctx context.Context) error {
	if _, err := exec.LookPath("update-grub"); err == nil {
		return shell.Command(ctx, "update-grub").Run()
	}
	candidates := []string{"/boot/grub2/grub.cfg", "/boot/grub/grub.cfg"}
	for _, out := range candidates {
		if _, err := os.Stat(out); err == nil {
			return shell.Command(ctx, "grub2-mkconfig", "-o", out).Run()
		}
	}
	return shell.Command(ctx, "grub2-mkconfig", "-o", "/boot/grub2/grub.cfg").Run()
}
