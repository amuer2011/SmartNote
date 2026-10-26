//go:build cgo && ((windows && amd64) || ((darwin || linux) && (amd64 || arm64)))

package embedding

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBundledModelRealOfflineInference(t *testing.T) {
	e := New(t.TempDir())
	defer e.Close()
	start := time.Now()
	out, err := e.Embed([]string{"数据库连接失败，排查SQL慢查询和数据库索引优化。", "SQL数据库的连接诊断、查询性能与索引调优。", "假期去海边旅行，预订酒店与安排景点行程。", "你好"})
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range out {
		if len(v) != Dimensions {
			t.Fatal(len(v))
		}
		var n float64
		for _, x := range v {
			n += x * x
		}
		if math.Abs(n-1) > 1e-6 {
			t.Fatalf("not normalized: %f", n)
		}
	}
	dot := func(a, b []float64) float64 {
		var v float64
		for i := range a {
			v += a[i] * b[i]
		}
		return v
	}
	related, unrelated := dot(out[0], out[1]), dot(out[0], out[2])
	if related < unrelated+0.1 {
		t.Fatalf("poor semantic separation: related %.3f unrelated %.3f", related, unrelated)
	}
	t.Logf("real CPU inference in %s, Chinese related=%.3f unrelated=%.3f", time.Since(start), related, unrelated)
	again, err := e.Embed([]string{"数据库连接失败，排查SQL慢查询和数据库索引优化。"})
	if err != nil || dot(out[0], again[0]) < 0.99999 {
		t.Fatal("cache changed vector", err)
	}
	if e.Status().State != "ready" || e.Status().Cached != 4 {
		t.Fatal(e.Status())
	}
	longText := strings.Repeat("数据库的查询与索引优化。", 100)
	if v, err := e.Embed([]string{longText}); err != nil || len(v[0]) != Dimensions {
		t.Fatal("long document chunking failed", err)
	}
	if err = e.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err = e.Embed([]string{"x"}); err == nil {
		t.Fatal("closed engine accepted inference")
	}
}
func TestResourcesRepairCorruptCache(t *testing.T) {
	root := t.TempDir()
	dir, err := unpack(root)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "vocab.txt")
	if err = os.WriteFile(path, []byte("corrupt"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = unpack(root); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(path)
	if !strings.HasPrefix(string(b), "[PAD]") {
		t.Fatal("corruption was not repaired")
	}
}
