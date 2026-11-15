package websitestat

import (
	"testing"

	"github.com/libtnb/assert/must"
)

func TestParseLogEntry(t *testing.T) {
	tests := []struct {
		name    string
		msg     string
		wantTag string
		want    *LogEntry
	}{
		{
			name:    "nginx",
			msg:     `<190>Sep 14 10:00:00 ace_stat_demo: {"site":"demo","uri":"/a","status":200,"bytes":10,"ua":"curl","ip":"1.2.3.4","method":"GET","content_type":"text/html","req_length":100,"rt":0.5}`,
			wantTag: "ace_stat_demo",
			want:    &LogEntry{Site: "demo", URI: "/a", Status: 200, Bytes: 10, UA: "curl", IP: "1.2.3.4", Method: "GET", ContentType: "text/html", ReqLength: 100, RequestTime: 0.5},
		},
		{
			name:    "caddy",
			msg:     `{"level":"info","ts":1789386806.38,"logger":"http.log.access.ace_stat","msg":"handled request","request":{"remote_ip":"::1","remote_port":"49367","client_ip":"::1","proto":"HTTP/2.0","method":"POST","host":"localhost","uri":"/api?x=1","headers":{"User-Agent":["curl/8.7.1"],"Accept":["*/*"]}},"bytes_read":42,"user_id":"","duration":0.0009,"size":11,"status":404,"resp_headers":{"Content-Type":["text/html; charset=utf-8"]},"site":"demo"}`,
			wantTag: "",
			want:    &LogEntry{Site: "demo", URI: "/api?x=1", Status: 404, Bytes: 11, UA: "curl/8.7.1", IP: "::1", Method: "POST", ContentType: "text/html; charset=utf-8", ReqLength: 42, RequestTime: 0.0009},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tag, data := parseSyslog([]byte(test.msg))
			must.Equal(t, tag, test.wantTag)
			entry, err := ParseLogEntry(tag, data)
			must.NoError(t, err)
			must.DeepEqual(t, entry, test.want)
		})
	}
}
