package ntp

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/beevik/ntp"

	"github.com/acepanel/panel/pkg/shell"
)

var ErrNotReachable = errors.New("failed to reach NTP server")

var ErrNoAvailableServer = errors.New("no available NTP server found")

// builtinAddresses 内置的 NTP 服务器地址列表
var builtinAddresses = []string{
	"ntp.aliyun.com",   // 阿里云
	"ntp1.aliyun.com",  // 阿里云2
	"ntp.tencent.com",  // 腾讯云
	"time.windows.com", // Windows
	"time.apple.com",   // Apple
}

// defaultAddresses 用于时间同步的 NTP 服务器地址列表（可被用户自定义）
var defaultAddresses = builtinAddresses

// mu 保护 defaultAddresses 的读写
var mu sync.RWMutex

// GetDefaultServers 获取当前配置的 NTP 服务器列表
func GetDefaultServers() []string {
	mu.RLock()
	defer mu.RUnlock()
	result := make([]string, len(defaultAddresses))
	copy(result, defaultAddresses)
	return result
}

// SetDefaultServers 设置 NTP 服务器列表
func SetDefaultServers(servers []string) {
	mu.Lock()
	defer mu.Unlock()
	defaultAddresses = servers
}

// GetBuiltinServers 获取内置的 NTP 服务器列表
func GetBuiltinServers() []string {
	result := make([]string, len(builtinAddresses))
	copy(result, builtinAddresses)
	return result
}

func Now(address ...string) (time.Time, error) {
	if len(address) > 0 && address[0] != "" {
		if now, err := ntp.Time(address[0]); err != nil {
			return time.Now(), fmt.Errorf("%w: %s", ErrNotReachable, err)
		} else {
			return now, nil
		}
	}

	servers := GetDefaultServers()
	best, err := bestServer(servers...)
	if err != nil {
		return time.Now(), err
	}

	now, err := ntp.Time(best)
	if err != nil {
		return time.Now(), fmt.Errorf("%w: %s", ErrNotReachable, err)
	}

	return now, nil
}

func UpdateSystemTime(t time.Time) error {
	_, err := shell.Execf(`date -s '%s'`, t.Format(time.DateTime))
	return err
}

func UpdateSystemTimeZone(tz string) error {
	_, err := shell.Execf(`timedatectl set-timezone '%s'`, tz)
	return err
}

// pingServer 计算NTP服务器的延迟
func pingServer(addr string) (time.Duration, error) {
	options := ntp.QueryOptions{Timeout: 1 * time.Second}
	response, err := ntp.QueryWithOptions(addr, options)
	if err != nil {
		return 0, err
	}

	return response.RTT, nil
}

// bestServer 返回延迟最低的NTP服务器
func bestServer(addresses ...string) (string, error) {
	if len(addresses) == 0 {
		addresses = GetDefaultServers()
	}

	type ntpResult struct {
		address string
		delay   time.Duration
		err     error
	}

	results := make(chan ntpResult, len(addresses))
	var wg sync.WaitGroup

	for _, addr := range addresses {
		wg.Add(1)
		go func(addr string) {
			defer wg.Done()

			delay, err := pingServer(addr)
			results <- ntpResult{address: addr, delay: delay, err: err}
		}(addr)
	}

	wg.Wait()
	close(results)

	var bestAddr string
	var bestDelay time.Duration
	found := false

	for result := range results {
		if result.err != nil {
			continue
		}

		if !found || result.delay < bestDelay {
			bestAddr = result.address
			bestDelay = result.delay
			found = true
		}
	}

	if !found {
		return "", ErrNoAvailableServer
	}

	return bestAddr, nil
}
