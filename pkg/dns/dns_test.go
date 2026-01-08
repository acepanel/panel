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

// TestUpdateNetplanDNSWithWifi 测试 wifi 网络接口配置
func (s *DNSTestSuite) TestUpdateNetplanDNSWithWifi() {
	content := `network:
  version: 2
  wifis:
    wlan0:
      dhcp4: true
      access-points:
        "my-wifi":
          password: "secret"`

	result, err := updateNetplanDNS(content, "8.8.8.8", "")
	s.NoError(err)
	s.Contains(result, "nameservers:")
	s.Contains(result, "8.8.8.8")
}

// TestUpdateNetplanDNSWithMultipleInterfaces 测试多接口配置
func (s *DNSTestSuite) TestUpdateNetplanDNSWithMultipleInterfaces() {
	content := `network:
  version: 2
  ethernets:
    eth0:
      dhcp4: true
    eth1:
      addresses:
        - 192.168.1.100/24
      gateway4: 192.168.1.1`

	result, err := updateNetplanDNS(content, "8.8.8.8", "114.114.114.114")
	s.NoError(err)
	s.Contains(result, "nameservers:")
	s.Contains(result, "8.8.8.8")
	s.Contains(result, "114.114.114.114")
}

// TestUpdateNetplanDNSWithRoutes 测试带路由配置的接口
func (s *DNSTestSuite) TestUpdateNetplanDNSWithRoutes() {
	content := `network:
  version: 2
  ethernets:
    eth0:
      dhcp4: false
      addresses:
        - 10.0.0.10/24
      routes:
        - to: default
          via: 10.0.0.1`

	result, err := updateNetplanDNS(content, "223.5.5.5", "223.6.6.6")
	s.NoError(err)
	s.Contains(result, "nameservers:")
	s.Contains(result, "223.5.5.5")
	s.Contains(result, "223.6.6.6")
	// 路由配置应该被保留
	s.Contains(result, "routes:")
}

// TestUpdateNetplanDNSWithBond 测试 bond 网络配置
func (s *DNSTestSuite) TestUpdateNetplanDNSWithBond() {
	content := `network:
  version: 2
  ethernets:
    eth0: {}
    eth1: {}
  bonds:
    bond0:
      interfaces:
        - eth0
        - eth1
      addresses:
        - 10.0.0.100/24
      parameters:
        mode: active-backup
        primary: eth0`

	result, err := updateNetplanDNS(content, "8.8.8.8", "8.8.4.4")
	s.NoError(err)
	s.Contains(result, "nameservers:")
	s.Contains(result, "8.8.8.8")
	// bond 配置应该被保留
	s.Contains(result, "bonds:")
	s.Contains(result, "bond0:")
}

// TestUpdateNetplanDNSWithRenderer 测试带 renderer 的配置
func (s *DNSTestSuite) TestUpdateNetplanDNSWithRenderer() {
	content := `network:
  version: 2
  renderer: networkd
  ethernets:
    ens3:
      dhcp4: true`

	result, err := updateNetplanDNS(content, "8.8.8.8", "")
	s.NoError(err)
	s.Contains(result, "renderer: networkd")
	s.Contains(result, "nameservers:")
}

// TestUpdateNetplanDNSPreserveSearch 测试保留 search 域
func (s *DNSTestSuite) TestUpdateNetplanDNSPreserveSearch() {
	content := `network:
  version: 2
  ethernets:
    eth0:
      dhcp4: true
      nameservers:
        addresses: [1.1.1.1]
        search: [example.com, local]`

	result, err := updateNetplanDNS(content, "8.8.8.8", "8.8.4.4")
	s.NoError(err)
	s.Contains(result, "8.8.8.8")
	s.Contains(result, "8.8.4.4")
	// 旧 DNS 应该被替换
	s.NotContains(result, "1.1.1.1")
	// search 域应该被保留
	s.Contains(result, "search:")
	s.Contains(result, "example.com")
}

// TestUpdateNetplanDNSEmptyConfig 测试空配置
// 注意：当配置为空时，会尝试检测活动网络接口并自动添加配置
func (s *DNSTestSuite) TestUpdateNetplanDNSEmptyConfig() {
	content := `network:
  version: 2`

	result, err := updateNetplanDNS(content, "8.8.8.8", "")
	// 如果系统有活动网络接口，应该成功
	// 如果没有（如在 CI 环境），应该返回错误
	if err == nil {
		s.Contains(result, "nameservers:")
		s.Contains(result, "8.8.8.8")
	}
}

// TestUpdateNetplanDNSInvalidYAML 测试无效的 YAML
func (s *DNSTestSuite) TestUpdateNetplanDNSInvalidYAML() {
	content := `network:
  version: 2
  ethernets:
    eth0:
    invalid yaml here`

	_, err := updateNetplanDNS(content, "8.8.8.8", "")
	s.Error(err)
}

// TestUpdateNetplanDNSWithMatch 测试带 match 规则的配置
func (s *DNSTestSuite) TestUpdateNetplanDNSWithMatch() {
	content := `network:
  version: 2
  ethernets:
    id0:
      match:
        macaddress: "00:11:22:33:44:55"
      set-name: eth0
      dhcp4: true`

	result, err := updateNetplanDNS(content, "8.8.8.8", "")
	s.NoError(err)
	s.Contains(result, "nameservers:")
	s.Contains(result, "match:")
	s.Contains(result, "macaddress")
}

// TestUpdateNetplanDNSWithVlan 测试 VLAN 配置
func (s *DNSTestSuite) TestUpdateNetplanDNSWithVlan() {
	content := `network:
  version: 2
  ethernets:
    eth0:
      dhcp4: false
  vlans:
    vlan100:
      id: 100
      link: eth0
      addresses:
        - 192.168.100.1/24`

	result, err := updateNetplanDNS(content, "8.8.8.8", "8.8.4.4")
	s.NoError(err)
	s.Contains(result, "nameservers:")
	s.Contains(result, "vlans:")
	s.Contains(result, "vlan100:")
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

// TestIsValidNetworkInterface 测试网络接口名称验证
func (s *DNSTestSuite) TestIsValidNetworkInterface() {
	// 有效的接口名
	s.True(isValidNetworkInterface("eth0"))
	s.True(isValidNetworkInterface("ens3"))
	s.True(isValidNetworkInterface("enp0s3"))
	s.True(isValidNetworkInterface("wlan0"))
	s.True(isValidNetworkInterface("bond0"))
	s.True(isValidNetworkInterface("br0"))

	// 无效的接口名（虚拟接口）
	s.False(isValidNetworkInterface("lo"))
	s.False(isValidNetworkInterface("docker0"))
	s.False(isValidNetworkInterface("veth12345"))
	s.False(isValidNetworkInterface("br-abc123"))
	s.False(isValidNetworkInterface("virbr0"))
	s.False(isValidNetworkInterface("tun0"))
	s.False(isValidNetworkInterface("tap0"))
	s.False(isValidNetworkInterface("flannel.1"))
	s.False(isValidNetworkInterface("cni0"))
	s.False(isValidNetworkInterface("cali12345"))
	s.False(isValidNetworkInterface(""))
}

// TestDetectActiveInterface 测试活动接口检测
func (s *DNSTestSuite) TestDetectActiveInterface() {
	// 这个测试依赖系统环境，只验证函数不会 panic
	iface := detectActiveInterface()
	// 在有网络的环境下应该能检测到接口
	// 在 CI 环境可能返回空字符串
	if iface != "" {
		s.True(isValidNetworkInterface(iface))
	}
}
