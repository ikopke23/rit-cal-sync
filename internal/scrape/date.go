package scrape

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// months maps every month spelling seen on the RIT page (full names, three
// letter abbreviations and "Sept") to its time.Month. Keys are lowercase and
// have no trailing period.
var months = map[string]time.Month{
	"january": time.January, "february": time.February, "march": time.March,
	"april": time.April, "may": time.May, "june": time.June, "july": time.July,
	"august": time.August, "september": time.September, "october": time.October,
	"november": time.November, "december": time.December,
	"jan": time.January, "feb": time.February, "mar": time.March, "apr": time.April,
	"jun": time.June, "jul": time.July, "aug": time.August, "sep": time.September,
	"sept": time.September, "oct": time.October, "nov": time.November, "dec": time.December,
}

var (
	dashReplacer = strings.NewReplacer(" ", " ", "–", "-", "—", "-")
	weekdayRe    = regexp.MustCompile(`\s*\([A-Za-z]+\)\s*$`)
	// Each pattern captures month words and days; months are validated
	// against the months table afterwards so the regexes stay readable.
	monthRangeRe = regexp.MustCompile(`^([A-Za-z]+)\.?\s*(\d{1,2})\s*-\s*([A-Za-z]+)\.?\s*(\d{1,2})$`)
	dayRangeRe   = regexp.MustCompile(`^([A-Za-z]+)\.?\s*(\d{1,2})\s*-\s*(\d{1,2})$`)
	singleRe     = regexp.MustCompile(`^([A-Za-z]+)\.?\s*(\d{1,2})(?:\s*,\s*(\d{4}))?$`)
	listRe       = regexp.MustCompile(`^[A-Za-z]+\.?\s*\d{1,2}(?:\s*,\s*(?:[A-Za-z]+\.?\s*)?\d{1,2})+$`)
	listItemRe   = regexp.MustCompile(`^(?:([A-Za-z]+)\.?\s*)?(\d{1,2})$`)
)

// ParseDate parses one RIT date expression into an inclusive civil-date span.
// Years are rarely written on the page, so defaultYear (the term heading's
// year) fills them in; an explicit ", YYYY" wins. A range or list whose end
// falls before its start (Dec → Jan) rolls the end into the next year.
func ParseDate(raw string, defaultYear int) (start, end time.Time, err error) {
	s := strings.Join(strings.Fields(dashReplacer.Replace(raw)), " ")
	s = weekdayRe.ReplaceAllString(s, "")
	switch {
	case monthRangeRe.MatchString(s):
		m := monthRangeRe.FindStringSubmatch(s)
		if start, err = civil(raw, defaultYear, m[1], m[2]); err != nil {
			return time.Time{}, time.Time{}, err
		}
		if end, err = civil(raw, defaultYear, m[3], m[4]); err != nil {
			return time.Time{}, time.Time{}, err
		}
	case dayRangeRe.MatchString(s):
		m := dayRangeRe.FindStringSubmatch(s)
		if start, err = civil(raw, defaultYear, m[1], m[2]); err != nil {
			return time.Time{}, time.Time{}, err
		}
		if end, err = civil(raw, defaultYear, m[1], m[3]); err != nil {
			return time.Time{}, time.Time{}, err
		}
	case singleRe.MatchString(s):
		m := singleRe.FindStringSubmatch(s)
		year := defaultYear
		if m[3] != "" {
			year, _ = strconv.Atoi(m[3])
		}
		if start, err = civil(raw, year, m[1], m[2]); err != nil {
			return time.Time{}, time.Time{}, err
		}
		end = start
	case listRe.MatchString(s):
		var month string
		for i, item := range strings.Split(s, ",") {
			m := listItemRe.FindStringSubmatch(strings.TrimSpace(item))
			if m[1] != "" {
				month = m[1]
			}
			d, err := civil(raw, defaultYear, month, m[2])
			if err != nil {
				return time.Time{}, time.Time{}, err
			}
			if i == 0 {
				start = d
			}
			end = d
		}
	default:
		return time.Time{}, time.Time{}, fmt.Errorf("unrecognized date %q", raw)
	}
	if end.Before(start) {
		end = end.AddDate(1, 0, 0)
	}
	return start, end, nil
}

// civil builds a 00:00 UTC date, rejecting unknown month words and days that
// time.Date would silently normalize (e.g. February 30 → March 2).
func civil(raw string, year int, monthWord, dayStr string) (time.Time, error) {
	month, ok := months[strings.ToLower(monthWord)]
	if !ok {
		return time.Time{}, fmt.Errorf("unrecognized date %q", raw)
	}
	day, _ := strconv.Atoi(dayStr)
	t := time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
	if t.Month() != month || t.Day() != day {
		return time.Time{}, fmt.Errorf("invalid day in date %q", raw)
	}
	return t, nil
}

// inferYear moves a span into the next year when it starts more than 60 days
// before the term's first date. The page omits years, so a Fall term's
// January entries would otherwise land eleven months too early. A zero
// termStart means the term has no anchor yet and nothing is adjusted.
func inferYear(start, end, termStart time.Time) (time.Time, time.Time) {
	if termStart.IsZero() || !start.Before(termStart.AddDate(0, 0, -60)) {
		return start, end
	}
	return start.AddDate(1, 0, 0), end.AddDate(1, 0, 0)
}
