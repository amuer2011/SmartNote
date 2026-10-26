package core

import (
	"errors"
	"fmt"
	"github.com/6tail/lunar-go/calendar"
	"path/filepath"
	"sync"
	"time"
)

type LunarDay struct {
	Month string `json:"month"`
	Day   string `json:"day"`
	Term  string `json:"term"`
}
type LunarYear struct {
	Year    int                 `json:"year"`
	Version int                 `json:"version"`
	Days    map[string]LunarDay `json:"days"`
}

var lunarCacheMu sync.Mutex

// GetLunarYear computes locally using lunar-go and persists one versioned file per year.
func (s *Service) GetLunarYear(year int) (LunarYear, error) {
	if year < 1900 || year > 2100 {
		return LunarYear{}, errors.New("日历支持 1900—2100 年")
	}
	lunarCacheMu.Lock()
	defer lunarCacheMu.Unlock()
	path := filepath.Join(s.root, "calendar", fmt.Sprintf("lunar-%d.json", year))
	var cached LunarYear
	first := time.Date(year, 1, 1, 12, 0, 0, 0, time.UTC)
	last := first.AddDate(1, 0, 0)
	valid := readJSON(path, &cached) == nil && cached.Year == year && cached.Version == 1
	if valid {
		for d := first; d.Before(last); d = d.AddDate(0, 0, 1) {
			v, ok := cached.Days[d.Format("2006-01-02")]
			if !ok || v.Day == "" || v.Month == "" {
				valid = false
				break
			}
		}
	}
	if valid {
		return cached, nil
	}
	out := LunarYear{Year: year, Version: 1, Days: map[string]LunarDay{}}
	for d := first; d.Before(last); d = d.AddDate(0, 0, 1) {
		l := calendar.NewSolarFromYmd(d.Year(), int(d.Month()), d.Day()).GetLunar()
		out.Days[d.Format("2006-01-02")] = LunarDay{Month: l.GetMonthInChinese() + "月", Day: l.GetDayInChinese(), Term: l.GetJieQi()}
	}
	return out, writeJSON(path, out)
}
