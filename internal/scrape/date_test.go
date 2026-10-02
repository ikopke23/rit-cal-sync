package scrape

import (
	"testing"
	"time"
)

func day(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }

func TestParseDate(t *testing.T) {
	tests := []struct {
		name      string
		raw       string
		year      int
		wantStart time.Time
		wantEnd   time.Time
		wantErr   bool
	}{
		{name: "single_weekday", raw: "September 7 (Monday)", year: 2026, wantStart: day(2026, 9, 7), wantEnd: day(2026, 9, 7)},
		{name: "single_plain", raw: "August 29", year: 2026, wantStart: day(2026, 8, 29), wantEnd: day(2026, 8, 29)},
		{name: "explicit_year", raw: "August 24, 2026 (Monday)", year: 2025, wantStart: day(2026, 8, 24), wantEnd: day(2026, 8, 24)},
		{name: "same_month_range", raw: "October 12-13", year: 2026, wantStart: day(2026, 10, 12), wantEnd: day(2026, 10, 13)},
		{name: "range_endash", raw: "March 7–14", year: 2027, wantStart: day(2027, 3, 7), wantEnd: day(2027, 3, 14)},
		{name: "cross_year_range", raw: "Dec. 19 - Jan. 10", year: 2026, wantStart: day(2026, 12, 19), wantEnd: day(2027, 1, 10)},
		{name: "list_one_month", raw: "Dec. 9,10,11,14,15,16", year: 2026, wantStart: day(2026, 12, 9), wantEnd: day(2026, 12, 16)},
		{name: "list_two_months", raw: "Apr. 28,29,30, May 3,4,5", year: 2027, wantStart: day(2027, 4, 28), wantEnd: day(2027, 5, 5)},
		{name: "abbrev_no_period", raw: "Dec 25 - Jan 1", year: 2026, wantStart: day(2026, 12, 25), wantEnd: day(2027, 1, 1)},
		{name: "invalid_day", raw: "February 30", year: 2027, wantErr: true},
		{name: "garbage", raw: "TBD", year: 2027, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, end, err := ParseDate(tt.raw, tt.year)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseDate(%q): got nil error, want error", tt.raw)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseDate(%q): %v", tt.raw, err)
			}
			if !start.Equal(tt.wantStart) || !end.Equal(tt.wantEnd) {
				t.Errorf("got %v → %v, want %v → %v", start.Format(dateLayout), end.Format(dateLayout),
					tt.wantStart.Format(dateLayout), tt.wantEnd.Format(dateLayout))
			}
		})
	}
}

func TestInferYear(t *testing.T) {
	tests := []struct {
		name      string
		start     time.Time
		end       time.Time
		termStart time.Time
		wantStart time.Time
		wantEnd   time.Time
	}{
		{name: "fall_january_moves_forward", start: day(2026, 1, 10), end: day(2026, 1, 12), termStart: day(2026, 8, 24), wantStart: day(2027, 1, 10), wantEnd: day(2027, 1, 12)},
		{name: "within_term_unchanged", start: day(2026, 12, 19), end: day(2027, 1, 10), termStart: day(2026, 8, 24), wantStart: day(2026, 12, 19), wantEnd: day(2027, 1, 10)},
		{name: "zero_term_start_unchanged", start: day(2026, 1, 10), end: day(2026, 1, 10), wantStart: day(2026, 1, 10), wantEnd: day(2026, 1, 10)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, end := inferYear(tt.start, tt.end, tt.termStart)
			if !start.Equal(tt.wantStart) || !end.Equal(tt.wantEnd) {
				t.Errorf("got %v → %v, want %v → %v", start.Format(dateLayout), end.Format(dateLayout),
					tt.wantStart.Format(dateLayout), tt.wantEnd.Format(dateLayout))
			}
		})
	}
}
