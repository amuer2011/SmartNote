package core

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// withHolidayServer points the package-level endpoint/client at a local test
// server for the duration of the test.
func withHolidayServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(handler)
	oldEndpoint, oldClient := holidayEndpoint, holidayClient
	holidayEndpoint = srv.URL
	holidayClient = srv.Client()
	t.Cleanup(func() {
		holidayEndpoint, holidayClient = oldEndpoint, oldClient
		srv.Close()
	})
	return srv
}

func sampleHolidayJSON(year int) string {
	return fmt.Sprintf(`{
  "code": 0,
  "holiday": {
    "01-01": {"holiday": true, "name": "元旦", "wage": 3, "date": "%04d-01-01"},
    "01-26": {"holiday": false, "name": "春节前补班", "wage": 1, "date": "%04d-01-26"}
  },
  "type": {
    "%04d-01-01": {"type": 0, "name": "元旦", "week": 4},
    "%04d-01-26": {"type": 3, "name": "补班", "week": 1}
  }
}`, year, year, year, year)
}

func newHolidayService(t *testing.T, root string) *Service {
	t.Helper()
	s, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func holidayCacheFile(root string, year int) string {
	return filepath.Join(root, "calendar", fmt.Sprintf("holidays-%d.json", year))
}

func TestGetHolidayYearFetchesParsesAndCaches(t *testing.T) {
	const year = 2026
	var hits int32
	var mu sync.Mutex
	var gotUA, gotAccept, gotQuery string
	withHolidayServer(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		mu.Lock()
		gotUA = r.Header.Get("User-Agent")
		gotAccept = r.Header.Get("Accept")
		gotQuery = r.URL.RawQuery
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, sampleHolidayJSON(year))
	})
	root := filepath.Join(t.TempDir(), "data")
	s := newHolidayService(t, root)

	out, err := s.GetHolidayYear(year)
	if err != nil {
		t.Fatal(err)
	}
	if out.Year != year {
		t.Fatalf("Year = %d, 期望 %d", out.Year, year)
	}
	if len(out.Holiday) != 2 || len(out.Type) != 2 {
		t.Fatalf("map 长度 = %d/%d, 期望 2/2", len(out.Holiday), len(out.Type))
	}
	// 调休补班（holiday=false）必须保留。
	rush := out.Holiday["01-26"]
	if rush.Holiday {
		t.Fatal("调休补班应保留 holiday=false")
	}
	if rush.Name != "春节前补班" || rush.Wage != 1 || rush.Date != "2026-01-26" {
		t.Fatalf("调休条目解析错误: %+v", rush)
	}
	if day := out.Holiday["01-01"]; !day.Holiday || day.Wage != 3 {
		t.Fatalf("元旦条目解析错误: %+v", day)
	}
	if typ := out.Type["2026-01-26"]; typ.Type != 3 || typ.Week != 1 || typ.Name != "补班" {
		t.Fatalf("type 条目解析错误: %+v", typ)
	}

	mu.Lock()
	defer mu.Unlock()
	if !strings.Contains(gotUA, "Mozilla") {
		t.Fatalf("缺少浏览器 User-Agent: %q", gotUA)
	}
	if !strings.Contains(gotAccept, "application/json") {
		t.Fatalf("缺少 Accept 头: %q", gotAccept)
	}
	if !strings.Contains(gotQuery, "type=Y") || !strings.Contains(gotQuery, "week=Y") {
		t.Fatalf("查询参数错误: %q", gotQuery)
	}

	if _, err = os.Stat(holidayCacheFile(root, year)); err != nil {
		t.Fatalf("缓存文件未写入: %v", err)
	}
	// 有效缓存命中后不再联网。
	again, err := s.GetHolidayYear(year)
	if err != nil {
		t.Fatal(err)
	}
	if again.Holiday["01-01"].Name != "元旦" {
		t.Fatalf("缓存读取错误: %+v", again.Holiday["01-01"])
	}
	if got := atomic.LoadInt32(&hits); got != 1 {
		t.Fatalf("网络请求次数 = %d, 期望 1", got)
	}
}

func TestGetHolidayYearConcurrentSingleFetch(t *testing.T) {
	const year = 2027
	var hits int32
	withHolidayServer(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		time.Sleep(30 * time.Millisecond)
		_, _ = io.WriteString(w, sampleHolidayJSON(year))
	})
	s := newHolidayService(t, filepath.Join(t.TempDir(), "data"))

	var wg sync.WaitGroup
	errCh := make(chan error, 16)
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := s.GetHolidayYear(year)
			errCh <- err
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatal(err)
		}
	}
	if got := atomic.LoadInt32(&hits); got != 1 {
		t.Fatalf("并发网络请求次数 = %d, 期望 1", got)
	}
}

func TestGetHolidayYearCrossServiceSingleFetch(t *testing.T) {
	const year = 2028
	var hits int32
	withHolidayServer(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		time.Sleep(30 * time.Millisecond)
		_, _ = io.WriteString(w, sampleHolidayJSON(year))
	})
	root := filepath.Join(t.TempDir(), "data")
	s1, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	defer s1.Close()
	s2, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()

	var wg sync.WaitGroup
	errCh := make(chan error, 2)
	for _, s := range []*Service{s1, s2} {
		wg.Add(1)
		go func(s *Service) {
			defer wg.Done()
			_, e := s.GetHolidayYear(year)
			errCh <- e
		}(s)
	}
	wg.Wait()
	close(errCh)
	for e := range errCh {
		if e != nil {
			t.Fatal(e)
		}
	}
	if got := atomic.LoadInt32(&hits); got != 1 {
		t.Fatalf("跨 Service 网络请求次数 = %d, 期望 1", got)
	}
}

func TestGetHolidayYearReusesCacheAfterRestart(t *testing.T) {
	const year = 2029
	var hits int32
	withHolidayServer(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		_, _ = io.WriteString(w, sampleHolidayJSON(year))
	})
	root := filepath.Join(t.TempDir(), "data")

	first, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = first.GetHolidayYear(year); err != nil {
		t.Fatal(err)
	}
	if err = first.Close(); err != nil {
		t.Fatal(err)
	}

	second, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	out, err := second.GetHolidayYear(year)
	if err != nil {
		t.Fatal(err)
	}
	if out.Holiday["01-01"].Name != "元旦" {
		t.Fatalf("重启后缓存内容错误: %+v", out.Holiday)
	}
	if got := atomic.LoadInt32(&hits); got != 1 {
		t.Fatalf("重启后网络请求次数 = %d, 期望 1", got)
	}
}

func TestGetHolidayYearErrorDoesNotPolluteCache(t *testing.T) {
	const year = 2030
	var hits int32
	var failing int32 = 1
	withHolidayServer(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		if atomic.LoadInt32(&failing) == 1 {
			http.Error(w, "boom", http.StatusInternalServerError)
			return
		}
		_, _ = io.WriteString(w, sampleHolidayJSON(year))
	})
	root := filepath.Join(t.TempDir(), "data")
	s := newHolidayService(t, root)

	if _, err := s.GetHolidayYear(year); err == nil {
		t.Fatal("HTTP 500 时应返回错误")
	}
	if _, err := os.Stat(holidayCacheFile(root, year)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("失败后不应写入缓存: %v", err)
	}

	// 失败不落盘，下一次请求应重试并成功。
	atomic.StoreInt32(&failing, 0)
	if _, err := s.GetHolidayYear(year); err != nil {
		t.Fatal(err)
	}
	if got := atomic.LoadInt32(&hits); got != 2 {
		t.Fatalf("网络请求次数 = %d, 期望 2（先失败后重试）", got)
	}
}

func TestGetHolidayYearUnpublishedYear(t *testing.T) {
	const year = 2099
	withHolidayServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"code":0,"holiday":{},"type":{}}`)
	})
	root := filepath.Join(t.TempDir(), "data")
	s := newHolidayService(t, root)

	_, err := s.GetHolidayYear(year)
	if err == nil || !strings.Contains(err.Error(), "尚未公布") {
		t.Fatalf("空 maps 应返回“尚未公布”错误, got %v", err)
	}
	if _, statErr := os.Stat(holidayCacheFile(root, year)); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("未公布年份不应写入缓存: %v", statErr)
	}
}

func TestGetHolidayYearRejectsForeignDates(t *testing.T) {
	const year = 2031
	withHolidayServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{
  "code": 0,
  "holiday": {"01-01": {"holiday": true, "name": "元旦", "wage": 3, "date": "2030-01-01"}},
  "type": {"2031-01-01": {"type": 0, "name": "元旦", "week": 1}}
}`)
	})
	root := filepath.Join(t.TempDir(), "data")
	s := newHolidayService(t, root)

	if _, err := s.GetHolidayYear(year); err == nil {
		t.Fatal("跨年份日期应被拒绝")
	}
	if _, statErr := os.Stat(holidayCacheFile(root, year)); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("非法数据不应写入缓存: %v", statErr)
	}
}

func TestGetHolidayYearRejectsNonZeroCode(t *testing.T) {
	const year = 2032
	withHolidayServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"code":-1,"holiday":{},"type":{}}`)
	})
	root := filepath.Join(t.TempDir(), "data")
	s := newHolidayService(t, root)

	if _, err := s.GetHolidayYear(year); err == nil {
		t.Fatal("code != 0 时应返回错误")
	}
	if _, statErr := os.Stat(holidayCacheFile(root, year)); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("code != 0 不应写入缓存: %v", statErr)
	}
}

func TestGetHolidayYearRejectsOversizedBody(t *testing.T) {
	const year = 2033
	withHolidayServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, strings.Repeat("x", holidayMaxBody+1))
	})
	root := filepath.Join(t.TempDir(), "data")
	s := newHolidayService(t, root)

	if _, err := s.GetHolidayYear(year); err == nil {
		t.Fatal("超过 4MB 的响应应返回错误")
	}
	if _, statErr := os.Stat(holidayCacheFile(root, year)); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("超大响应不应写入缓存: %v", statErr)
	}
}

func TestGetHolidayYearInvalidCacheIsRefetched(t *testing.T) {
	const year = 2034
	var hits int32
	withHolidayServer(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		_, _ = io.WriteString(w, sampleHolidayJSON(year))
	})
	root := filepath.Join(t.TempDir(), "data")
	cacheDir := filepath.Join(root, "calendar")
	if err := os.MkdirAll(cacheDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(holidayCacheFile(root, year), []byte(`{"year":2034,"holiday":{"01-01":{"holiday":true,"name":"旧","date":"2033-01-01","wage":3}},"type":{}}`), 0600); err != nil {
		t.Fatal(err)
	}
	s := newHolidayService(t, root)

	out, err := s.GetHolidayYear(year)
	if err != nil {
		t.Fatal(err)
	}
	if got := atomic.LoadInt32(&hits); got != 1 {
		t.Fatalf("无效缓存应触发一次联网, 实际 %d", got)
	}
	if out.Holiday["01-01"].Date != "2034-01-01" {
		t.Fatalf("缓存未被刷新: %+v", out.Holiday["01-01"])
	}
}

func TestGetHolidayYearBounds(t *testing.T) {
	s := newHolidayService(t, filepath.Join(t.TempDir(), "data"))
	for _, year := range []int{0, 1899, 2101, 9999} {
		if _, err := s.GetHolidayYear(year); err == nil {
			t.Fatalf("年份 %d 应被拒绝", year)
		}
	}
}

func TestWeekendOnlyYearNotCached(t *testing.T) {
	s := &Service{root: t.TempDir()}
	withHolidayServer(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"code":0,"holiday":{"01-02":{"holiday":true,"name":"周六","date":"2027-01-02"}},"type":{"2027-01-02":{"type":1,"name":"周六","week":6}}}`)
	})
	if _, e := s.GetHolidayYear(2027); e == nil {
		t.Fatal("unpublished year accepted")
	}
	if _, e := os.Stat(s.holidayCachePath(2027)); !os.IsNotExist(e) {
		t.Fatal("unpublished year cached")
	}
}
