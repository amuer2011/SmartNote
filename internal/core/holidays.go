package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	holidayMinYear = 1900
	holidayMaxYear = 2100
	holidayMaxBody = 4 << 20
	holidayUA      = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36"
	holidayAccept  = "application/json, text/plain, */*"
)

// HolidayDay describes a single day entry returned by the holiday API.
type HolidayDay struct {
	Holiday bool   `json:"holiday"`
	Name    string `json:"name"`
	Date    string `json:"date"`
	Wage    int    `json:"wage"`
}

// HolidayType describes the day classification returned by the holiday API.
type HolidayType struct {
	Type int    `json:"type"`
	Name string `json:"name"`
	Week int    `json:"week"`
}

// HolidayYear is the cached holiday calendar for one Gregorian year.
type HolidayYear struct {
	Year    int                    `json:"year"`
	Holiday map[string]HolidayDay  `json:"holiday"`
	Type    map[string]HolidayType `json:"type"`
}

type holidayAPIResponse struct {
	Code    int                    `json:"code"`
	Holiday map[string]HolidayDay  `json:"holiday"`
	Type    map[string]HolidayType `json:"type"`
}

// The endpoint and client are package variables so tests can substitute a local
// server. Production always talks to the fixed public timor.tech address.
var (
	holidayEndpoint = "https://timor.tech/api/holiday/year"
	holidayClient   = &http.Client{Timeout: 15 * time.Second}
	// holidayFetchMu serializes network fetches process-wide so concurrent
	// callers (including separate Service instances) share one request per year.
	holidayFetchMu sync.Mutex
)

// GetHolidayYear returns the holiday calendar for year, fetching and caching it
// under s.root/calendar on first use. A valid cache is never refreshed.
func (s *Service) GetHolidayYear(year int) (HolidayYear, error) {
	if year < holidayMinYear || year > holidayMaxYear {
		return HolidayYear{}, fmt.Errorf("节假日年份 %d 超出支持范围 %d-%d", year, holidayMinYear, holidayMaxYear)
	}
	path := s.holidayCachePath(year)
	if cached, ok := loadHolidayCache(path, year); ok {
		return cached, nil
	}
	// Hold the global lock only for the fetch: Service.mu is never taken here,
	// so other Service operations are not blocked by network latency.
	holidayFetchMu.Lock()
	defer holidayFetchMu.Unlock()
	if cached, ok := loadHolidayCache(path, year); ok {
		return cached, nil
	}
	fetched, err := fetchHolidayYear(year)
	if err != nil {
		return HolidayYear{}, err
	}
	if err = writeJSON(path, fetched); err != nil {
		return HolidayYear{}, err
	}
	return fetched, nil
}

func (s *Service) holidayCachePath(year int) string {
	return filepath.Join(s.root, "calendar", fmt.Sprintf("holidays-%d.json", year))
}

// loadHolidayCache reports whether path holds a complete, correctly-shaped
// calendar for year. Any malformed or partial file is treated as absent.
func loadHolidayCache(path string, year int) (HolidayYear, bool) {
	var cached HolidayYear
	if err := readJSON(path, &cached); err != nil {
		return HolidayYear{}, false
	}
	if cached.Year != year || cached.Holiday == nil || cached.Type == nil {
		return HolidayYear{}, false
	}
	if len(cached.Holiday) == 0 || len(cached.Type) == 0 {
		return HolidayYear{}, false
	}
	if err := validateHolidayYear(cached, year); err != nil {
		return HolidayYear{}, false
	}
	return cached, true
}

// validateHolidayYear enforces that every entry belongs to year and that the
// holiday key (MM-DD) agrees with the entry date (YYYY-MM-DD).
func validateHolidayYear(v HolidayYear, year int) error {
	published := false
	for _, day := range v.Holiday {
		if day.Holiday && day.Name != "" && day.Name != "周六" && day.Name != "周日" && day.Name != "星期六" && day.Name != "星期日" && day.Name != "周末" {
			published = true
			break
		}
	}
	if !published {
		return fmt.Errorf("%d 年法定节假日安排尚未公布", year)
	}
	prefix := fmt.Sprintf("%04d-", year)
	for key, day := range v.Holiday {
		if len(key) != 5 || key[2] != '-' {
			return fmt.Errorf("节假日键 %q 应为 MM-DD 格式", key)
		}
		if len(day.Date) != 10 || !strings.HasPrefix(day.Date, prefix) || day.Date[5:] != key {
			return fmt.Errorf("节假日 %q 的日期 %q 不属于 %d 年或与键不一致", key, day.Date, year)
		}
		if _, err := time.Parse("2006-01-02", day.Date); err != nil {
			return fmt.Errorf("节假日 %q 的日期 %q 无效", key, day.Date)
		}
	}
	for key := range v.Type {
		if len(key) != 10 || !strings.HasPrefix(key, prefix) {
			return fmt.Errorf("类型键 %q 应为 %d 年的 YYYY-MM-DD 格式", key, year)
		}
		if _, err := time.Parse("2006-01-02", key); err != nil {
			return fmt.Errorf("类型键 %q 无效", key)
		}
	}
	return nil
}

func fetchHolidayYear(year int) (HolidayYear, error) {
	url := fmt.Sprintf("%s/%d?type=Y&week=Y", strings.TrimRight(holidayEndpoint, "/"), year)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return HolidayYear{}, fmt.Errorf("构造节假日请求失败: %w", err)
	}
	req.Header.Set("User-Agent", holidayUA)
	req.Header.Set("Accept", holidayAccept)
	resp, err := holidayClient.Do(req)
	if err != nil {
		return HolidayYear{}, fmt.Errorf("请求节假日数据失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return HolidayYear{}, fmt.Errorf("节假日接口返回 HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, holidayMaxBody+1))
	if err != nil {
		return HolidayYear{}, fmt.Errorf("读取节假日数据失败: %w", err)
	}
	if len(body) > holidayMaxBody {
		return HolidayYear{}, errors.New("节假日数据超过 4 MB 限制")
	}
	var payload holidayAPIResponse
	if err = json.Unmarshal(body, &payload); err != nil {
		return HolidayYear{}, fmt.Errorf("解析节假日数据失败: %w", err)
	}
	if payload.Code != 0 {
		return HolidayYear{}, fmt.Errorf("节假日接口返回错误码 %d", payload.Code)
	}
	if payload.Holiday == nil || payload.Type == nil {
		return HolidayYear{}, fmt.Errorf("%d 年节假日数据不完整", year)
	}
	if len(payload.Holiday) == 0 || len(payload.Type) == 0 {
		return HolidayYear{}, fmt.Errorf("%d 年节假日安排尚未公布", year)
	}
	out := HolidayYear{Year: year, Holiday: payload.Holiday, Type: payload.Type}
	if err = validateHolidayYear(out, year); err != nil {
		return HolidayYear{}, err
	}
	return out, nil
}
