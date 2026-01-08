package dns

import (
	"os"
	"testing"

	"github.com/stretchr/testify/suite"
)

type DNSTestSuite struct {
	suite.Suite
}

func TestDNSTestSuite(t *testing.T) {
	suite.Run(t, &DNSTestSuite{})
}

func (s *DNSTestSuite) SetupTest() {
	if _, err := os.Stat("testdata"); os.IsNotExist(err) {
		s.NoError(os.MkdirAll("testdata", 0755))
	}
}

func (s *DNSTestSuite) TearDownTest() {
	s.NoError(os.RemoveAll("testdata"))
}

func (s *DNSTestSuite) TestManagerString() {
	s.Equal("NetworkManager", ManagerNetworkManager.String())
	s.Equal("netplan", ManagerNetplan.String())
	s.Equal("resolv.conf", ManagerResolvConf.String())
	s.Equal("unknown", ManagerUnknown.String())
}

func (s *DNSTestSuite) TestDetectManager() {
	// DetectManager 会返回一个有效的 Manager 类型
	manager := DetectManager()
	s.True(manager >= ManagerUnknown && manager <= ManagerResolvConf)
}

func (s *DNSTestSuite) TestBuildNetplanDNSConfig() {
	// 测试单个 DNS
	config := buildNetplanDNSConfig("8.8.8.8", "")
	s.Contains(config, "nameservers:")
	s.Contains(config, "8.8.8.8")
	s.NotContains(config, ",")

	// 测试两个 DNS
	config = buildNetplanDNSConfig("8.8.8.8", "8.8.4.4")
	s.Contains(config, "nameservers:")
	s.Contains(config, "8.8.8.8")
	s.Contains(config, "8.8.4.4")
}

func (s *DNSTestSuite) TestUpdateNetplanDNS() {
	// 测试基本的 netplan 配置更新
	content := `network:
  version: 2
  ethernets:
    eth0:
      dhcp4: true`

	result, err := updateNetplanDNS(content, "8.8.8.8", "8.8.4.4")
	s.NoError(err)
	s.Contains(result, "nameservers:")
	s.Contains(result, "8.8.8.8")
	s.Contains(result, "8.8.4.4")
}

func (s *DNSTestSuite) TestUpdateNetplanDNSWithExisting() {
	// 测试替换现有的 DNS 配置
	content := `network:
  version: 2
  ethernets:
    eth0:
      dhcp4: true
      nameservers:
        addresses: [1.1.1.1, 1.0.0.1]`

	result, err := updateNetplanDNS(content, "8.8.8.8", "8.8.4.4")
	s.NoError(err)
	s.Contains(result, "8.8.8.8")
	s.Contains(result, "8.8.4.4")
	// 旧的 DNS 应该被移除
	s.NotContains(result, "1.1.1.1")
	s.NotContains(result, "1.0.0.1")
}

func (s *DNSTestSuite) TestFindNetplanConfig() {
	// findNetplanConfig 应该能正常执行不崩溃
	// 在实际系统上可能会找到配置文件也可能找不到
	configPath, err := findNetplanConfig()
	if err == nil {
		// 如果找到了配置文件，验证文件确实存在
		s.FileExists(configPath)
	}
	// 无论是否找到配置文件，函数都应该正常返回
}

func (s *DNSTestSuite) TestSetDNSWithResolvConf() {
	// 这个测试需要 root 权限才能写入 /etc/resolv.conf
	// 在非特权环境中跳过
	s.T().Skip("需要 root 权限")
}

func (s *DNSTestSuite) TestGetDNS() {
	// GetDNS 应该能返回当前的 DNS 配置
	dns, manager, err := GetDNS()
	// 即使出错也不应该 panic
	if err != nil {
		s.T().Logf("获取 DNS 出错（可能没有权限）: %v", err)
		return
	}
	s.NotNil(dns)
	s.True(manager >= ManagerUnknown && manager <= ManagerResolvConf)
}
