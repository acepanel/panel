package ipset

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/libtnb/assert/check"
	"github.com/libtnb/assert/must"
)

// lines 集合写出的各行
func lines(t *testing.T, set Set) []string {
	t.Helper()
	var sb strings.Builder
	_, err := set.WriteTo(&sb)
	must.NoError(t, err)
	return strings.Fields(sb.String())
}

func TestParse(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "地址、CIDR 与地址段",
			input: "1.2.3.4\n5.6.7.0/24\n9.9.9.1-9.9.9.9\n",
			want:  []string{"1.2.3.4", "5.6.7.0-5.6.7.255", "9.9.9.1-9.9.9.9"},
		},
		{
			name:  "注释、空行与行内附加内容",
			input: "# firehol_level1\n\n; Spamhaus DROP\n1.2.3.4 # scanner\n5.6.7.0/24 ; SBL123\n8.8.8.8\tgoogle\n9.9.9.9,quad9\n",
			want:  []string{"1.2.3.4", "5.6.7.0-5.6.7.255", "8.8.8.8", "9.9.9.9"},
		},
		{
			name:  "认不出的行跳过",
			input: "<html>\nexample.com\n1.2.3.4/33\n9.9.9.9-1.1.1.1\n1.2.3.4-::1\nfe80::1%eth0\n1.2.3.4\n",
			want:  []string{"1.2.3.4"},
		},
		{
			name:  "CIDR 的主机位清零",
			input: "10.1.2.3/8\n",
			want:  []string{"10.0.0.0-10.255.255.255"},
		},
		{
			name:  "重叠与相邻的地址段合并",
			input: "1.1.1.0/24\n1.1.1.128/25\n1.1.2.0/24\n1.1.4.0\n1.1.3.255\n",
			want:  []string{"1.1.1.0-1.1.2.255", "1.1.3.255-1.1.4.0"},
		},
		{
			name:  "IPv4 在前 IPv6 在后",
			input: "2001:db8::/32\n1.2.3.4\n2001:db8::1\n::ffff:5.6.7.8\n::ffff:9.9.9.0/120\n",
			want:  []string{"1.2.3.4", "5.6.7.8", "9.9.9.0-9.9.9.255", "2001:db8::-2001:db8:ffff:ffff:ffff:ffff:ffff:ffff"},
		},
		{
			name:  "到地址空间末尾的段吞掉后面的",
			input: "240.0.0.0/4\n255.255.255.255\n250.0.0.1\n",
			want:  []string{"240.0.0.0-255.255.255.255"},
		},
		{
			name:  "Windows 换行",
			input: "1.2.3.4\r\n5.6.7.8\r\n",
			want:  []string{"1.2.3.4", "5.6.7.8"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			set, err := Parse(strings.NewReader(tt.input))
			must.NoError(t, err)
			check.DeepEqual(t, lines(t, set), tt.want)
			check.Equal(t, set.Len(), len(tt.want))
		})
	}
}

func TestParseRoundTrip(t *testing.T) {
	set := Of("1.2.3.4", "5.6.7.0/24", "2001:db8::/48", "9.9.9.1-9.9.9.9")
	var sb strings.Builder
	_, err := set.WriteTo(&sb)
	must.NoError(t, err)

	parsed, err := Parse(strings.NewReader(sb.String()))
	must.NoError(t, err)
	check.DeepEqual(t, lines(t, parsed), lines(t, set))
}

func TestUnion(t *testing.T) {
	set := Union(Of("1.1.1.0/24", "2001:db8::1"), Of("1.1.2.0/24", "8.8.8.8"), Set{})
	check.DeepEqual(t, lines(t, set), []string{"1.1.1.0-1.1.2.255", "8.8.8.8", "2001:db8::1"})
}

func TestTableRender(t *testing.T) {
	f, err := os.Create(filepath.Join(t.TempDir(), "table.nft"))
	must.NoError(t, err)
	must.NoError(t, Table{}.render(f, Of("192.0.2.1"), Of("1.2.3.4", "5.6.7.0/24", "2001:db8::/32")))
	must.NoError(t, f.Close())

	content, err := os.ReadFile(f.Name())
	must.NoError(t, err)
	check.Equal(t, string(content), `table inet acepanel
delete table inet acepanel
table inet acepanel {
	set allow4 {
		type ipv4_addr
		flags interval
		elements = { 192.0.2.1 }
	}
	set allow6 {
		type ipv6_addr
		flags interval
	}
	set block4 {
		type ipv4_addr
		flags interval
		elements = { 1.2.3.4,
			5.6.7.0-5.6.7.255 }
	}
	set block6 {
		type ipv6_addr
		flags interval
		elements = { 2001:db8::-2001:db8:ffff:ffff:ffff:ffff:ffff:ffff }
	}
`+inboundChain+"}\n")
}
