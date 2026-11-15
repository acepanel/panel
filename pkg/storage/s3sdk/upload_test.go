package s3sdk

import (
	"bytes"
	"encoding/xml"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
)

// TestPut 用 mock S3 服务端验证 Put 的两条路径：
// 小文件走单次 PUT，大文件走流式分片并能正确重组
func TestPut(t *testing.T) {
	var mu sync.Mutex
	parts := map[int][]byte{}
	var simpleBody []byte
	var completeCount int

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		body, _ := io.ReadAll(r.Body)
		switch {
		case r.Method == http.MethodPost && q.Has("uploads"):
			_, _ = io.WriteString(w, `<InitiateMultipartUploadResult><UploadId>uid</UploadId></InitiateMultipartUploadResult>`)
		case r.Method == http.MethodPut && q.Get("uploadId") != "":
			n, _ := strconv.Atoi(q.Get("partNumber"))
			mu.Lock()
			parts[n] = body
			mu.Unlock()
			w.Header().Set("ETag", `"etag"`)
		case r.Method == http.MethodPost && q.Get("uploadId") != "":
			var req struct {
				Parts []completedPart `xml:"Part"`
			}
			_ = xml.Unmarshal(body, &req)
			mu.Lock()
			completeCount = len(req.Parts)
			mu.Unlock()
			_, _ = io.WriteString(w, `<CompleteMultipartUploadResult></CompleteMultipartUploadResult>`)
		case r.Method == http.MethodPut:
			mu.Lock()
			simpleBody = body
			mu.Unlock()
		default:
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer srv.Close()

	c := New(Config{Region: "us-east-1", Bucket: "b", Endpoint: srv.URL, PathStyle: true, Concurrency: 3})

	t.Run("small file uses single PUT", func(t *testing.T) {
		data := bytes.Repeat([]byte("x"), 1024)
		if err := c.Put(t.Context(), "small.txt", bytes.NewReader(data), "text/plain"); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(simpleBody, data) {
			t.Errorf("单次 PUT body 不符: 收到 %d 字节, 期望 %d", len(simpleBody), len(data))
		}
		if len(parts) != 0 {
			t.Errorf("小文件不应触发分片, 却收到 %d 片", len(parts))
		}
	})

	t.Run("large file uses streaming multipart", func(t *testing.T) {
		mu.Lock()
		parts = map[int][]byte{}
		completeCount = 0
		mu.Unlock()

		// 12MB → 5MB + 5MB + 2MB = 3 片
		data := make([]byte, 12*1024*1024)
		for i := range data {
			data[i] = byte(i)
		}
		if err := c.Put(t.Context(), "big.bin", bytes.NewReader(data), "application/octet-stream"); err != nil {
			t.Fatal(err)
		}
		if len(parts) != 3 {
			t.Fatalf("期望 3 片, 收到 %d 片", len(parts))
		}
		if completeCount != 3 {
			t.Errorf("complete 收到 %d 片, 期望 3", completeCount)
		}
		// 按片号重组，校验内容完整无错位
		var got []byte
		for i := 1; i <= 3; i++ {
			got = append(got, parts[i]...)
		}
		if !bytes.Equal(got, data) {
			t.Errorf("重组后内容不符: 收到 %d 字节, 期望 %d", len(got), len(data))
		}
	})
}

func TestPayloadHash(t *testing.T) {
	type seen struct{ put, part, complete string }
	handler := func(got *seen, mu *sync.Mutex) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			q := r.URL.Query()
			_, _ = io.Copy(io.Discard, r.Body)
			hash := r.Header.Get("x-amz-content-sha256")
			mu.Lock()
			defer mu.Unlock()
			switch {
			case r.Method == http.MethodPost && q.Has("uploads"):
				_, _ = io.WriteString(w, `<InitiateMultipartUploadResult><UploadId>uid</UploadId></InitiateMultipartUploadResult>`)
			case r.Method == http.MethodPut && q.Get("uploadId") != "":
				got.part = hash
				w.Header().Set("ETag", `"etag"`)
			case r.Method == http.MethodPost && q.Get("uploadId") != "":
				got.complete = hash
				_, _ = io.WriteString(w, `<CompleteMultipartUploadResult></CompleteMultipartUploadResult>`)
			case r.Method == http.MethodPut:
				got.put = hash
			}
		})
	}
	upload := func(t *testing.T, c *S3) {
		t.Helper()
		if err := c.Put(t.Context(), "small.txt", bytes.NewReader([]byte("x")), ""); err != nil {
			t.Fatal(err)
		}
		if err := c.Put(t.Context(), "big.bin", bytes.NewReader(make([]byte, minPartSize+1)), ""); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("https", func(t *testing.T) {
		var got seen
		var mu sync.Mutex
		srv := httptest.NewTLSServer(handler(&got, &mu))
		defer srv.Close()
		upload(t, New(Config{Region: "us-east-1", Bucket: "b", Endpoint: srv.URL, PathStyle: true, Client: srv.Client()}))
		if got.put != unsignedPayload || got.part != unsignedPayload {
			t.Errorf("put = %q, part = %q, want %s", got.put, got.part, unsignedPayload)
		}
		if got.complete == "" || got.complete == unsignedPayload {
			t.Errorf("complete = %q, want payload hash", got.complete)
		}
	})

	t.Run("http", func(t *testing.T) {
		var got seen
		var mu sync.Mutex
		srv := httptest.NewServer(handler(&got, &mu))
		defer srv.Close()
		upload(t, New(Config{Region: "us-east-1", Bucket: "b", Endpoint: srv.URL, PathStyle: true}))
		if got.put != sha256Hex([]byte("x")) {
			t.Errorf("put = %q, want payload hash", got.put)
		}
		if got.part == "" || got.part == unsignedPayload {
			t.Errorf("part = %q, want payload hash", got.part)
		}
	})
}

type sizedReader struct {
	io.Reader
	size int64
}

func (r sizedReader) Stat() (fs.FileInfo, error) { return sizedInfo{size: r.size}, nil }

type sizedInfo struct {
	fs.FileInfo
	size int64
}

func (i sizedInfo) Size() int64 { return i.size }

// TestPlanParts 已知大小的大文件要放大分片以装进 maxParts 片，并压低并发控制内存
func TestPlanParts(t *testing.T) {
	c := New(Config{Region: "us-east-1", Bucket: "b", Concurrency: 5})
	const gb = int64(1) << 30

	if size, n := c.planParts(bytes.NewReader(nil)); size != defaultPartSize || n != 5 {
		t.Errorf("unknown size: got %d, %d", size, n)
	}
	if size, n := c.planParts(sizedReader{size: gb}); size != defaultPartSize || n != 5 {
		t.Errorf("1GB: got %d, %d", size, n)
	}
	for _, total := range []int64{100 * gb, 1024 * gb, 5 * 1024 * gb} {
		size, n := c.planParts(sizedReader{size: total})
		if defaultPartSize+size*(maxParts-1) < total {
			t.Errorf("%dGB: part size %d cannot hold the object", total/gb, size)
		}
		if n < 1 || n > 5 || (n > 1 && size*int64(n) > maxBuffered) {
			t.Errorf("%dGB: part size %d with concurrency %d exceeds the budget", total/gb, size, n)
		}
	}
}
