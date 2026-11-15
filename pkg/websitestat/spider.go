package websitestat

import "strings"

// spiderDef 蜘蛛定义：UA 关键词 → 展示名称
type spiderDef struct {
	keyword string
	name    string
}

// 已知蜘蛛 UA 关键词及对应展示名称
var spiderDefs = []spiderDef{
	{"googlebot", "Googlebot"},
	{"bingbot", "Bingbot"},
	{"baiduspider", "Baiduspider"},
	{"yandexbot", "YandexBot"},
	{"sogou", "Sogou"},
	{"360spider", "360Spider"},
	{"bytespider", "Bytespider"},
	{"gptbot", "GPTBot"},
	{"claudebot", "ClaudeBot"},
	{"ahrefsbot", "AhrefsBot"},
	{"semrushbot", "SemrushBot"},
	{"dotbot", "DotBot"},
	{"mj12bot", "MJ12Bot"},
	{"petalbot", "PetalBot"},
	{"applebot", "Applebot"},
	{"duckduckbot", "DuckDuckBot"},
	{"slurp", "Slurp"},
	{"ia_archiver", "Alexa"},
	{"facebookexternalhit", "Facebook"},
	{"twitterbot", "Twitterbot"},
	{"rogerbot", "Rogerbot"},
	{"linkedinbot", "LinkedInBot"},
	{"embedly", "Embedly"},
	{"quora link preview", "Quora"},
	{"showyoubot", "ShowyouBot"},
	{"outbrain", "Outbrain"},
	{"pinterest", "Pinterest"},
	{"slackbot", "Slackbot"},
	{"vkshare", "VKShare"},
	{"w3c_validator", "W3CValidator"},
	{"redditbot", "Redditbot"},
	{"scrapy", "Scrapy"},
	{"curl", "cURL"},
	{"wget", "Wget"},
}

// spiderByKeyword 用关键词表精确命名，未命中返回空字符串
func spiderByKeyword(ua string) string {
	lower := strings.ToLower(ua)
	for _, def := range spiderDefs {
		if strings.Contains(lower, def.keyword) {
			return def.name
		}
	}

	return ""
}
