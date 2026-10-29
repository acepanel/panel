package conf

import (
	"testing"

	"github.com/libtnb/assert/check"
	"github.com/libtnb/assert/must"
)

func TestQuery(t *testing.T) {
	cfg := &Config{}
	cfg.Add("listen", "80")
	cfg.Add("listen", "443", "ssl")
	server := cfg.AddBlock("server", "a")
	server.Add("root", "/var/www")
	server.AddBlock("location", "/").Add("try_files", "$uri")
	cfg.AddMeta("pass", "http://x")

	check.Equal(t, cfg.Value("listen"), "80")
	must.Len(t, cfg.GetAll("listen"), 2)
	check.DeepEqual(t, cfg.GetAll("listen")[1].Values(), []string{"443", "ssl"})
	check.Equal(t, cfg.FindOne("server.root").Arg(0), "/var/www")
	check.Equal(t, cfg.FindOne("server.location.try_files").Arg(0), "$uri")
	check.Len(t, cfg.FindBlocks("server.location"), 1)
	check.Equal(t, cfg.GetBlock("server", "a"), server)
	check.Nil(t, cfg.GetBlock("server", "b"))
	check.Equal(t, cfg.Meta("pass"), "http://x")

	// nil 安全的链式取值
	check.Equal(t, cfg.Get("missing").Arg(0), "")
	check.Nil(t, cfg.Get("listen").Get("x"))
	check.Equal(t, cfg.Get("listen").Len(), 0)

	cfg.Set("root", "/new")
	check.Equal(t, cfg.Value("root"), "/new")
	check.Equal(t, cfg.Remove("listen"), 2)
	check.False(t, cfg.Has("listen"))
	check.Equal(t, cfg.RemoveFunc("server", func(d *Directive) bool { return d.Arg(0) == "a" }), 1)
	check.Nil(t, cfg.GetBlock("server", "a"))
}

func TestCustom(t *testing.T) {
	loc := Blk("location", "/")
	loc.Add("proxy_pass", "http://x")
	loc.AppendCustom(nil)
	check.Len(t, loc.Nodes, 1)

	loc.AppendCustom([]Node{Dir("proxy_cache_convert_head", "off"), Cmt("note")})
	check.Len(t, loc.Nodes, 4)
	gen, custom := loc.SplitCustom()
	check.DeepEqual(t, gen.Values(), []string{"/"})
	check.Len(t, gen.Nodes, 1)
	check.Equal(t, gen.Value("proxy_pass"), "http://x")
	check.False(t, gen.Has("proxy_cache_convert_head"))
	check.DeepEqual(t, custom, []Node{Dir("proxy_cache_convert_head", "off"), Cmt("note")})
	// 原指令不受影响，往浅拷贝追加也不会覆盖原节点
	check.Len(t, loc.Nodes, 4)
	gen.Add("proxy_buffering", "on")
	check.DeepEqual(t, loc.Nodes[1], Node(&Comment{Text: " ace:custom"}))

	plain := Blk("location", "/")
	plain.Add("proxy_pass", "http://x")
	gen, custom = plain.SplitCustom()
	check.Equal(t, gen, plain)
	check.Nil(t, custom)
	gen, custom = Dir("listen", "80").SplitCustom()
	check.Nil(t, gen.Block)
	check.Nil(t, custom)
}

func TestSort(t *testing.T) {
	cfg := &Config{}
	cfg.Append(Cmt("c1"))
	cfg.Add("b")
	cfg.Add("a")
	cfg.Append(Cmt("tail"))
	order := map[string]int{"a": 0, "b": 1}

	var got []string
	for _, n := range Sort(cfg.Nodes, func(d *Directive) int { return order[d.Name] }) {
		switch v := n.(type) {
		case *Directive:
			got = append(got, v.Name)
		case *Comment:
			got = append(got, "#"+v.Text)
		}
	}
	check.DeepEqual(t, got, []string{"a", "# c1", "b", "# tail"})
}
