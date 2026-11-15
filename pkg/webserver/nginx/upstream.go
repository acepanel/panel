package nginx

import (
	"fmt"
	"maps"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/acepanel/panel/v3/pkg/webserver/conf"
	"github.com/acepanel/panel/v3/pkg/webserver/types"
)

var upstreamFilePattern = regexp.MustCompile(`^(\d{3})-(.+)\.conf$`)

func parseUpstreamFiles(sharedDir string) []types.Upstream {
	// upstreamAlgos 能直接当指令写的算法，nginx 的 hash 必须带参数，裸写会拒载
	upstreamAlgos := []string{types.AlgoLeastConn, types.AlgoIPHash, types.AlgoRandom}
	var upstreams []types.Upstream
	for _, file := range conf.ListFiles(sharedDir, upstreamFilePattern, upstreamStartNum, upstreamEndNum) {
		cfg, err := ParseFile(file)
		if err != nil {
			continue
		}
		up := cfg.GetBlock("upstream")
		if up == nil || up.Arg(0) == "" {
			continue
		}
		upstream := types.Upstream{
			Name:      up.Arg(0),
			Servers:   make(map[string]string),
			Resolver:  []string{},
			Keepalive: atoi(up.Value("keepalive")),
		}
		for _, algo := range upstreamAlgos {
			if up.Has(algo) {
				upstream.Algo = algo
				break
			}
		}
		for _, srv := range up.GetAll("server") {
			if srv.Arg(0) != "" {
				upstream.Servers[srv.Arg(0)] = strings.Join(srv.ArgsFrom(1), " ")
			}
		}
		if resolver := up.Get("resolver").Values(); resolver != nil {
			upstream.Resolver = resolver
		}
		upstream.ResolverTimeout = parseDuration(up.Value("resolver_timeout"))
		upstreams = append(upstreams, upstream)
	}
	return upstreams
}

func writeUpstreamFiles(sharedDir string, upstreams []types.Upstream) error {
	if len(upstreams) > upstreamEndNum-upstreamStartNum+1 {
		return fmt.Errorf("upstreams exceed limit (%d)", upstreamEndNum-upstreamStartNum+1)
	}
	if err := conf.ClearFiles(sharedDir, upstreamFilePattern, upstreamStartNum, upstreamEndNum); err != nil {
		return err
	}
	for i, upstream := range upstreams {
		path := filepath.Join(sharedDir, fmt.Sprintf("%03d-%s.conf", upstreamStartNum+i, upstream.Name))
		if err := writeFragment(path, conf.Cmt("Upstream: "+upstream.Name), upstreamNode(upstream)); err != nil {
			return err
		}
	}
	return nil
}

func upstreamNode(u types.Upstream) *conf.Directive {
	up := conf.Blk("upstream", u.Name)
	up.Add("zone", u.Name, "512k")
	// 其它服务器带过来的算法名在 nginx 下是未知指令，整个 nginx 都会起不来
	if algo := types.NormalizeAlgo(u.Algo); algo != "" {
		up.Add(algo)
	}
	if len(u.Resolver) > 0 {
		up.Add("resolver", u.Resolver...)
		if u.ResolverTimeout > 0 {
			up.Add("resolver_timeout", formatDuration(u.ResolverTimeout))
		}
	}
	for _, addr := range slices.Sorted(maps.Keys(u.Servers)) {
		up.Add("server", append([]string{addr}, strings.Fields(u.Servers[addr])...)...)
	}
	if u.Keepalive > 0 {
		up.Add("keepalive", strconv.Itoa(u.Keepalive))
	}
	return up
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}
