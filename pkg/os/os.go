package os

import (
	"bufio"
	"os"
	"strings"
	"time"
)

// IsDebian 判断是否是 Debian 系统
func IsDebian() bool {
	return isDebian(readOSRelease())
}

// IsRHEL 判断是否是 RHEL 系统
func IsRHEL() bool {
	return isRHEL(readOSRelease())
}

// IsUbuntu 判断是否是 Ubuntu 系统
func IsUbuntu() bool {
	return isUbuntu(readOSRelease())
}

// IsEOL 判断系统是否已到达生命周期终点
func IsEOL() bool {
	return isEOL(readOSRelease(), time.Now())
}

func readOSRelease() map[string]string {
	file, err := os.Open("/etc/os-release")
	if err != nil {
		return nil
	}
	defer func(file *os.File) { _ = file.Close() }(file)

	osRelease := make(map[string]string)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) > 0 && strings.Contains(line, "=") {
			parts := strings.SplitN(line, "=", 2)
			key := parts[0]
			value := strings.Trim(parts[1], `"`)
			osRelease[key] = value
		}
	}
	if scanner.Err() != nil {
		return nil
	}
	return osRelease
}

func isDebian(osRelease map[string]string) bool {
	id, idLike := osRelease["ID"], osRelease["ID_LIKE"]
	return id == "debian" || strings.Contains(idLike, "debian")
}

func isRHEL(osRelease map[string]string) bool {
	// alinux Alibaba Cloud Linux
	// hce Huawei Cloud EulerOS
	// openEuler 欧拉
	id, idLike := osRelease["ID"], osRelease["ID_LIKE"]
	return id == "rhel" || id == "almalinux" || id == "rocky" || id == "alinux" || id == "tencentos" || id == "opencloudos" || strings.Contains(idLike, "rhel")
}

func isUbuntu(osRelease map[string]string) bool {
	id, idLike := osRelease["ID"], osRelease["ID_LIKE"]
	return id == "ubuntu" || strings.Contains(idLike, "ubuntu")
}

func isEOL(osRelease map[string]string, now time.Time) bool {
	eolTimeTable := map[string]map[string]time.Time{
		"rhel": {
			"9":  time.Date(2032, 5, 31, 0, 0, 0, 0, time.UTC),
			"10": time.Date(2035, 5, 31, 0, 0, 0, 0, time.UTC),
		},
		"debian": {
			"12": time.Date(2028, 6, 30, 0, 0, 0, 0, time.UTC),
			"13": time.Date(2030, 6, 30, 0, 0, 0, 0, time.UTC),
		},
		"ubuntu": {
			"22.04": time.Date(2027, 4, 1, 0, 0, 0, 0, time.UTC),
			"24.04": time.Date(2029, 5, 31, 0, 0, 0, 0, time.UTC),
			"26.04": time.Date(2031, 4, 30, 0, 0, 0, 0, time.UTC),
		},
	}

	// Ubuntu 以 年.月 为大版本，其余取第一段；衍生版的 VERSION_ID 可能不含点
	parts := strings.SplitN(osRelease["VERSION_ID"], ".", 3)
	major, majorMinor := parts[0], strings.Join(parts[:min(len(parts), 2)], ".")

	if isRHEL(osRelease) {
		if eol, ok := eolTimeTable["rhel"][major]; ok {
			return now.After(eol)
		}
	}
	if isUbuntu(osRelease) {
		if eol, ok := eolTimeTable["ubuntu"][majorMinor]; ok {
			return now.After(eol)
		}
	}
	if isDebian(osRelease) {
		if eol, ok := eolTimeTable["debian"][major]; ok {
			return now.After(eol)
		}
	}

	return false
}
