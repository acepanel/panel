package s3sdk

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestList 验证 V1 列举的翻页：有 NextMarker 时用它，没有时退回本页最后一个键
func TestList(t *testing.T) {
	var markers []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Has("list-type") || q.Has("continuation-token") {
			t.Errorf("unexpected V2 list parameter: %s", r.URL.RawQuery)
		}
		if q.Get("prefix") != "backup/" || q.Get("delimiter") != "/" {
			t.Errorf("unexpected query: %s", r.URL.RawQuery)
		}
		markers = append(markers, q.Get("marker"))
		switch q.Get("marker") {
		case "":
			_, _ = io.WriteString(w, `<ListBucketResult><IsTruncated>true</IsTruncated><NextMarker>backup/b.zip</NextMarker>
<Contents><Key>backup/a.zip</Key><Size>1</Size><LastModified>2026-01-02T03:04:05.000Z</LastModified></Contents>
<Contents><Key>backup/b.zip</Key><Size>2</Size></Contents></ListBucketResult>`)
		case "backup/b.zip":
			_, _ = io.WriteString(w, `<ListBucketResult><IsTruncated>true</IsTruncated>
<Contents><Key>backup/c.zip</Key><Size>3</Size></Contents>
<CommonPrefixes><Prefix>backup/sub/</Prefix></CommonPrefixes></ListBucketResult>`)
		case "backup/sub/":
			_, _ = io.WriteString(w, `<ListBucketResult><IsTruncated>false</IsTruncated>
<Contents><Key>backup/z.zip</Key><Size>4</Size></Contents></ListBucketResult>`)
		default:
			t.Errorf("unexpected marker %q", q.Get("marker"))
		}
	}))
	defer srv.Close()

	c := New(Config{Region: "us-east-1", Bucket: "b", AccessKey: "ak", SecretKey: "sk", Endpoint: srv.URL, PathStyle: true})
	var keys []string
	var sizes int64
	for obj, err := range c.List(t.Context(), "backup/", "/") {
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		keys = append(keys, obj.Key)
		sizes += obj.Size
	}

	want := []string{"backup/a.zip", "backup/b.zip", "backup/c.zip", "backup/z.zip"}
	if len(keys) != len(want) {
		t.Fatalf("keys = %v, want %v", keys, want)
	}
	for i := range want {
		if keys[i] != want[i] {
			t.Errorf("keys[%d] = %q, want %q", i, keys[i], want[i])
		}
	}
	if sizes != 10 {
		t.Errorf("sizes = %d, want 10", sizes)
	}
	if len(markers) != 3 {
		t.Errorf("requests = %d, want 3", len(markers))
	}
}
