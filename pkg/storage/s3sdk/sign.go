package s3sdk

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"path"
	"sort"
	"strings"
	"time"
)

const (
	algorithm       = "AWS4-HMAC-SHA256"
	serviceName     = "s3"
	amzDateFormat   = "20060102T150405Z"
	shortDateFormat = "20060102"
	// emptyPayloadSHA256 是空请求体的 SHA-256，SigV4 在无 body 时要求使用该值
	emptyPayloadSHA256 = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
)

// signRequest 按 AWS Signature V4 为请求添加鉴权头
// 约定：调用方若有请求体，须先设置 x-amz-content-sha256 头（其 SHA-256 或 UNSIGNED-PAYLOAD）；
// 未设置时按空 body 处理
func (c *S3) signRequest(req *http.Request) {
	c.signRequestAt(req, time.Now().UTC())
}

// signRequestAt 是 signRequest 的可测试内核，使用给定时间签名
func (c *S3) signRequestAt(req *http.Request, now time.Time) {
	amzDate := now.Format(amzDateFormat)

	// 重试会复用同一个请求重签，上一次的鉴权头不能被算进签名头
	req.Header.Del("Authorization")
	req.Header.Set("Host", req.Host)
	req.Header.Set("X-Amz-Date", amzDate)
	if req.Header.Get("x-amz-content-sha256") == "" {
		req.Header.Set("x-amz-content-sha256", emptyPayloadSHA256)
	}

	scope := now.Format(shortDateFormat) + "/" + c.region + "/" + serviceName + "/aws4_request"
	canonHeaders, signed := canonicalAndSignedHeaders(req)

	// 规范请求（Canonical Request）
	canonical := strings.Join([]string{
		req.Method,
		canonicalURI(req),
		canonicalQuery(req),
		canonHeaders, // 每行以 \n 结尾
		signed,
		req.Header.Get("x-amz-content-sha256"),
	}, "\n")

	// 待签字符串（String to Sign）
	stringToSign := strings.Join([]string{
		algorithm,
		amzDate,
		scope,
		sha256Hex([]byte(canonical)),
	}, "\n")

	signature := hex.EncodeToString(hmacSHA256(c.signingKey(now), []byte(stringToSign)))

	req.Header.Set("Authorization", fmt.Sprintf(
		"%s Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		algorithm, c.accessKey, scope, signed, signature,
	))
}

// signingKey 派生 SigV4 签名密钥，按天缓存（密钥只随日期/区域变化）
func (c *S3) signingKey(t time.Time) []byte {
	date := t.Format(shortDateFormat)

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.keyDate == date && c.keyCache != nil {
		return c.keyCache
	}

	k := hmacSHA256([]byte("AWS4"+c.secretKey), []byte(date))
	k = hmacSHA256(k, []byte(c.region))
	k = hmacSHA256(k, []byte(serviceName))
	k = hmacSHA256(k, []byte("aws4_request"))
	c.keyDate = date
	c.keyCache = k
	return k
}

// canonicalURI 返回去除查询串后、经清理的资源路径（已是 URL 编码形式）
func canonicalURI(req *http.Request) string {
	p := req.URL.RequestURI()
	if req.URL.RawQuery != "" {
		p = p[:len(p)-len(req.URL.RawQuery)-1]
	}
	trailing := strings.HasSuffix(p, "/")
	p = path.Clean(p) // 必须用 path.Clean 而非 filepath.Clean，后者在 Windows 上会用反斜杠
	if p != "/" && trailing {
		p += "/"
	}
	return p
}

// canonicalQuery 返回按键、值排序并按 RFC3986 编码的查询串
func canonicalQuery(req *http.Request) string {
	query := req.URL.Query()
	keys := make([]string, 0, len(query))
	for k := range query {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	for _, k := range keys {
		values := query[k]
		sort.Strings(values)
		for _, v := range values {
			if b.Len() > 0 {
				b.WriteByte('&')
			}
			b.WriteString(uriEncode(k, true))
			b.WriteByte('=')
			b.WriteString(uriEncode(v, true))
		}
	}
	return b.String()
}

// canonicalAndSignedHeaders 一次遍历产出规范头（每行 name:value\n）与签名头列表（name;name;…）
func canonicalAndSignedHeaders(req *http.Request) (canonical, signed string) {
	keys := make([]string, 0, len(req.Header))
	for k := range req.Header {
		keys = append(keys, strings.ToLower(k))
	}
	sort.Strings(keys)

	var cb, sb strings.Builder
	for i, k := range keys {
		values := req.Header.Values(http.CanonicalHeaderKey(k))
		sort.Strings(values)
		cb.WriteString(k)
		cb.WriteByte(':')
		cb.WriteString(strings.Join(values, ","))
		cb.WriteByte('\n')

		if i > 0 {
			sb.WriteByte(';')
		}
		sb.WriteString(k)
	}
	return cb.String(), sb.String()
}

// uriEncode 按 AWS SigV4 要求对字符串做 RFC3986 百分号编码
// 未保留字符 A-Za-z0-9-_.~ 不编码；encodeSlash 为 false 时 '/' 也保留（用于路径）
func uriEncode(s string, encodeSlash bool) string {
	const upperHex = "0123456789ABCDEF"

	var b strings.Builder
	b.Grow(len(s))
	for i := range len(s) {
		ch := s[i]
		switch {
		case ch >= 'A' && ch <= 'Z', ch >= 'a' && ch <= 'z', ch >= '0' && ch <= '9',
			ch == '-', ch == '_', ch == '.', ch == '~':
			b.WriteByte(ch)
		case ch == '/':
			if encodeSlash {
				b.WriteString("%2F")
			} else {
				b.WriteByte('/')
			}
		default:
			b.WriteByte('%')
			b.WriteByte(upperHex[ch>>4])
			b.WriteByte(upperHex[ch&0xf])
		}
	}
	return b.String()
}

func hmacSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
