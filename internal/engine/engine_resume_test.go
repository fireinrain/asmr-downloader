package engine

// doDownload 断点续传行为的单元测试：
// 1. 全新下载 → 文件内容完整
// 2. 本地残留半截文件 → 携带 Range 续传，最终内容与源字节级一致
// 3. 服务器忽略 Range 返回 200 → 截断重下，最终内容完整（不会拼接出坏文件）

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"asmroner/internal/model"

	"github.com/go-resty/resty/v2"
)

const testFileSize = 64 * 1024

// buildTestData 生成确定性测试数据（字节值循环递增，拼接错误必可检出）
func buildTestData() []byte {
	data := make([]byte, testFileSize)
	for i := range data {
		data[i] = byte(i % 251)
	}
	return data
}

// newRangeServer 启动支持 Range 的测试服务器，返回服务器与收到的 Range 头记录通道
func newRangeServer(t *testing.T, data []byte) (*httptest.Server, *[]string) {
	t.Helper()
	var ranges []string
	mux := http.NewServeMux()
	mux.HandleFunc("/file", func(w http.ResponseWriter, r *http.Request) {
		start := 0
		if rs := r.Header.Get("Range"); rs != "" {
			ranges = append(ranges, rs)
			var s int64
			if _, err := fmt.Sscanf(rs, "bytes=%d-", &s); err == nil && s >= 0 && s < int64(len(data)) {
				start = int(s)
			}
		}
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)-start))
		if start > 0 {
			w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, len(data)-1, len(data)))
			w.WriteHeader(http.StatusPartialContent)
		}
		_, _ = w.Write(data[start:])
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server, &ranges
}

func newTestManager() *EngineManager {
	return &EngineManager{Client: resty.New(), Config: &model.Config{}}
}

func TestDoDownload_FreshDownload(t *testing.T) {
	data := buildTestData()
	server, _ := newRangeServer(t, data)
	m := newTestManager()

	storePath := filepath.Join(t.TempDir(), "01.mp3")
	n, err := m.doDownload(server.URL+"/file", storePath, "01.mp3", int64(len(data)))
	if err != nil {
		t.Fatalf("doDownload failed: %v", err)
	}
	got, err := os.ReadFile(storePath)
	if err != nil {
		t.Fatal(err)
	}
	if n != int64(len(data)) || !bytes.Equal(got, data) {
		t.Fatalf("fresh download content mismatch: n=%d len=%d", n, len(got))
	}
}

func TestDoDownload_ResumeFromPartialFile(t *testing.T) {
	data := buildTestData()
	server, ranges := newRangeServer(t, data)
	m := newTestManager()

	storePath := filepath.Join(t.TempDir(), "02.wav")
	// 模拟中断残留：前 1000 字节已下载
	if err := os.WriteFile(storePath, data[:1000], 0644); err != nil {
		t.Fatal(err)
	}

	n, err := m.doDownload(server.URL+"/file", storePath, "02.wav", int64(len(data)))
	if err != nil {
		t.Fatalf("doDownload failed: %v", err)
	}
	if n != int64(len(data))-1000 {
		t.Errorf("resumed bytes = %d, want %d", n, len(data)-1000)
	}
	if len(*ranges) != 1 || (*ranges)[0] != "bytes=1000-" {
		t.Errorf("Range header = %v, want [bytes=1000-]", *ranges)
	}
	// 续传后内容必须与源字节级一致
	got, err := os.ReadFile(storePath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, data) {
		t.Fatalf("resumed content mismatch: len=%d want=%d", len(got), len(data))
	}
}

func TestDoDownload_ServerIgnoresRange(t *testing.T) {
	data := buildTestData()
	// 忽略 Range 的服务器：始终返回 200 全量内容
	mux := http.NewServeMux()
	mux.HandleFunc("/file", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
		_, _ = w.Write(data)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	m := newTestManager()
	storePath := filepath.Join(t.TempDir(), "03.flac")
	// 残留半截文件，且残留内容与源不同（拼接错误必可检出）
	junk := bytes.Repeat([]byte{0xFF}, 1000)
	if err := os.WriteFile(storePath, junk, 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := m.doDownload(server.URL+"/file", storePath, "03.flac", int64(len(data))); err != nil {
		t.Fatalf("doDownload failed: %v", err)
	}
	// 服务器不支持续传时应截断重下，最终内容完整
	got, err := os.ReadFile(storePath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, data) {
		t.Fatalf("content should be truncated and re-downloaded: len=%d want=%d", len(got), len(data))
	}
}
