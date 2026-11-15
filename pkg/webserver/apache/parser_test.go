package apache

import (
	"strings"
	"testing"

	"github.com/acepanel/panel/v3/pkg/webserver/conf"

	"github.com/libtnb/assert/check"
	"github.com/libtnb/assert/must"
)

func TestParseDirective(t *testing.T) {
	cfg, err := ParseString("ServerName www.example.com")
	must.NoError(t, err)

	d := cfg.Get("ServerName")
	must.NotNil(t, d)
	check.Equal(t, d.Name, "ServerName")
	check.DeepEqual(t, d.Values(), []string{"www.example.com"})
}

func TestParseMultipleArgs(t *testing.T) {
	cfg, err := ParseString("Listen 192.168.1.100:80")
	must.NoError(t, err)
	check.DeepEqual(t, conf.Values(cfg.Get("Listen").Args), []string{"192.168.1.100:80"})
}

func TestParseQuotedArgs(t *testing.T) {
	cfg, err := ParseString(`CustomLog "/var/log/apache2/access.log" combined`)
	must.NoError(t, err)

	d := cfg.Get("CustomLog")
	must.NotNil(t, d)
	// 引号被解析，值不含引号，但引号风格记在 Arg 上
	check.DeepEqual(t, d.Args, []conf.Arg{
		{Value: "/var/log/apache2/access.log", Quote: conf.QuoteDouble},
		{Value: "combined", Quote: conf.QuoteNone},
	})
	check.Contains(t, Export(cfg), `CustomLog "/var/log/apache2/access.log" combined`)
}

func TestParseVirtualHost(t *testing.T) {
	input := `<VirtualHost *:80>
    ServerName www.example.com
    DocumentRoot /var/www/html
</VirtualHost>`

	cfg, err := ParseString(input)
	must.NoError(t, err)

	vhosts := cfg.Blocks("VirtualHost")
	must.Len(t, vhosts, 1)
	check.DeepEqual(t, vhosts[0].Values(), []string{"*:80"})
	check.Equal(t, vhosts[0].Value("ServerName"), "www.example.com")
	check.Equal(t, vhosts[0].Value("DocumentRoot"), "/var/www/html")
}

func TestNestedBlocksTriple(t *testing.T) {
	input := `<VirtualHost *:80>
    <Directory /var/www>
        <Files index.php>
            Require all granted
        </Files>
    </Directory>
</VirtualHost>`

	cfg, err := ParseString(input)
	must.NoError(t, err)

	vhost := cfg.GetBlock("VirtualHost")
	must.NotNil(t, vhost)
	dir := vhost.GetBlock("Directory")
	must.NotNil(t, dir)
	files := dir.GetBlock("Files")
	must.NotNil(t, files)
	check.DeepEqual(t, files.Get("Require").Values(), []string{"all", "granted"})
}

// 双层嵌套块曾被旧实现丢弃
func TestNestedIfModuleNotDropped(t *testing.T) {
	input := `<IfModule mod_proxy_balancer.c>
    <Proxy balancer://backend>
        BalancerMember http://127.0.0.1:8080 loadfactor=5
        BalancerMember http://127.0.0.1:8081
        ProxySet lbmethod=byrequests
    </Proxy>
</IfModule>`

	cfg, err := ParseString(input)
	must.NoError(t, err)

	members := cfg.Find("IfModule.Proxy.BalancerMember")
	must.Len(t, members, 2)
	check.DeepEqual(t, members[0].Values(), []string{"http://127.0.0.1:8080", "loadfactor=5"})
	check.DeepEqual(t, members[1].Values(), []string{"http://127.0.0.1:8081"})
}

// 旧 lexer 的字符白名单会把特殊字符粘连或截断
func TestTokenizeSpecialChars(t *testing.T) {
	cases := []struct {
		input string
		want  []string
	}{
		{`RewriteCond %{HTTP_HOST} ^old\.example\.com$ [NC]`, []string{"%{HTTP_HOST}", `^old\.example\.com$`, "[NC]"}},
		{`RewriteRule ^(.*)$ https://new.example.com$1 [R=308,L]`, []string{"^(.*)$", "https://new.example.com$1", "[R=308,L]"}},
		{`ProxyPass / http://127.0.0.1:8080/app`, []string{"/", "http://127.0.0.1:8080/app"}},
		{`SetHandler "proxy:unix:/tmp/php-cgi-84.sock|fcgi://localhost/"`, []string{"proxy:unix:/tmp/php-cgi-84.sock|fcgi://localhost/"}},
	}
	for _, c := range cases {
		t.Run(c.input, func(t *testing.T) {
			cfg, err := ParseString(c.input)
			must.NoError(t, err)
			must.Len(t, cfg.Nodes, 1)
			d, ok := cfg.Nodes[0].(*conf.Directive)
			must.True(t, ok, must.Msgf("首个节点应为指令，实际是 %T", cfg.Nodes[0]))
			check.DeepEqual(t, d.Values(), c.want)
		})
	}
}

func TestLineContinuation(t *testing.T) {
	cfg, err := ParseString("RewriteCond %{HTTP_HOST} foo \\\n    [NC]")
	must.NoError(t, err)
	d := cfg.Get("RewriteCond")
	must.NotNil(t, d)
	check.DeepEqual(t, d.Values(), []string{"%{HTTP_HOST}", "foo", "[NC]"})
}

func TestLineContinuationEscapedBackslash(t *testing.T) {
	cfg, err := ParseString("ServerName a\\\\\nServerAdmin b")
	must.NoError(t, err)
	check.NotNil(t, cfg.Get("ServerName"))
	check.NotNil(t, cfg.Get("ServerAdmin"))
}

func TestCommentSemantics(t *testing.T) {
	cfg, err := ParseString("# a comment\nServerName x")
	must.NoError(t, err)
	cmts := collectComments(cfg.Nodes)
	must.Len(t, cmts, 1)
	check.Equal(t, cmts[0].Text, " a comment")

	// 行内 # 是参数的一部分，不当注释
	cfg2, err := ParseString("Redirect 301 /a /b#frag")
	must.NoError(t, err)
	check.DeepEqual(t, cfg2.Get("Redirect").Values(), []string{"301", "/a", "/b#frag"})
	check.Empty(t, collectComments(cfg2.Nodes))
}

func TestCommentPreserveLeadingSpace(t *testing.T) {
	cfg, err := ParseString("#  double space")
	must.NoError(t, err)
	check.Contains(t, Export(cfg), "#  double space")
}

func TestRoundTripDefaultVhostConf(t *testing.T) {
	cfg, err := ParseString(defaultVhostConf)
	must.NoError(t, err)

	rendered := Render(cfg)
	cfg2, err := ParseString(rendered)
	must.NoError(t, err)
	check.Equal(t, Render(cfg2), rendered, check.Msgf("规范化导出应幂等"))

	must.Contains(t, rendered, "IncludeOptional")
	must.Contains(t, rendered, "<VirtualHost")
	check.Less(t, strings.Index(rendered, "IncludeOptional"), strings.Index(rendered, "<VirtualHost"),
		check.Msgf("顶层 IncludeOptional 应排在 VirtualHost 之前，实际导出：\n%s", rendered))
}

func TestExportNestedRoundTrip(t *testing.T) {
	input := `<VirtualHost *:80>
    ServerName x
    <Directory /var/www>
        Require all granted
    </Directory>
</VirtualHost>`

	cfg, err := ParseString(input)
	must.NoError(t, err)

	out := Export(cfg)
	cfg2, err := ParseString(out)
	must.NoError(t, err)
	check.Equal(t, Export(cfg2), out, check.Msgf("保序导出应幂等"))
	vhost := cfg2.GetBlock("VirtualHost")
	must.NotNil(t, vhost)
	check.NotNil(t, vhost.GetBlock("Directory"))
}

func TestQueryCaseInsensitive(t *testing.T) {
	cfg, err := ParseString("ServerName x")
	must.NoError(t, err)
	check.NotNil(t, cfg.Get("servername"))
	check.Equal(t, cfg.Value("SERVERNAME"), "x")
	check.True(t, cfg.Has("ServerName"))
}

func TestFindDotPath(t *testing.T) {
	input := `<IfModule a.c>
    <Proxy p>
        Member 1
        Member 2
    </Proxy>
</IfModule>`

	cfg, err := ParseString(input)
	must.NoError(t, err)
	check.Len(t, cfg.Find("IfModule.Proxy.Member"), 2)
	check.Len(t, cfg.FindBlocks("IfModule.Proxy"), 1)
	check.Nil(t, cfg.FindOne("IfModule.Proxy.Missing"))
}

func TestTolerantUnclosedBlock(t *testing.T) {
	cfg, err := ParseString("<VirtualHost *:80>\n    ServerName x")
	must.NoError(t, err)
	must.Len(t, cfg.Blocks("VirtualHost"), 1)
	check.Equal(t, cfg.Blocks("VirtualHost")[0].Value("ServerName"), "x")
}

func TestTolerantOrphanCloseTag(t *testing.T) {
	cfg, err := ParseString("</Foo>\nServerName x")
	must.NoError(t, err)
	check.Equal(t, cfg.Value("ServerName"), "x")
}

func TestParseEmpty(t *testing.T) {
	cfg, err := ParseString("")
	must.NoError(t, err)
	check.Empty(t, cfg.Nodes)
}

func TestSetAndRemove(t *testing.T) {
	cfg, err := ParseString("ServerName old")
	must.NoError(t, err)

	cfg.Set("ServerName", "new")
	check.Equal(t, cfg.Value("ServerName"), "new")

	cfg.Add("ServerAlias", "a", "b")
	check.DeepEqual(t, cfg.Get("ServerAlias").Values(), []string{"a", "b"})

	check.Equal(t, cfg.Remove("ServerName"), 1)
	check.False(t, cfg.Has("ServerName"))
}

func TestAddDirectiveAutoQuote(t *testing.T) {
	cfg := &conf.Config{}
	cfg.Add("AuthName", "My Realm")
	check.Contains(t, Export(cfg), `AuthName "My Realm"`)

	cfg2 := &conf.Config{}
	cfg2.Add("DocumentRoot", "/var/www")
	out := Export(cfg2)
	check.Contains(t, out, "DocumentRoot /var/www")
	check.NotContains(t, out, `"`)
}

func collectComments(nodes []conf.Node) []*conf.Comment {
	var out []*conf.Comment
	for _, n := range nodes {
		switch v := n.(type) {
		case *conf.Comment:
			out = append(out, v)
		case *conf.Directive:
			if v.Block != nil {
				out = append(out, collectComments(v.Nodes)...)
			}
		}
	}
	return out
}
