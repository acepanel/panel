package websitestat

import (
	"sync"

	ua "github.com/medama-io/go-useragent"
)

// 字典树常驻约 5 MB，推迟到收到首条访问日志再构建
var uaParser = sync.OnceValue(ua.NewParser)

// classifyUA 单次解析 UA
func classifyUA(rawUA string) (spider, browser, os string) {
	if spider = spiderByKeyword(rawUA); spider != "" {
		return spider, "", ""
	}

	agent := uaParser().Parse(rawUA)
	if agent.IsBot() {
		return "Other", "", ""
	}

	return "", browserOf(agent), osOf(agent)
}

// browserOf 浏览器名称 + 主版本号
func browserOf(agent ua.UserAgent) string {
	name := string(agent.Browser())
	major := agent.BrowserVersionMajor()
	switch {
	case name == "":
		return "Other"
	case major == "":
		return name
	default:
		return name + " " + major
	}
}

func osOf(agent ua.UserAgent) string {
	if name := string(agent.OS()); name != "" {
		return name
	}
	return "Other"
}
