package scrape

import (
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

func fall(title, raw string, start, end string) Event {
	return Event{TermID: "2261", TermName: "Fall 2026", Title: title, Start: mustDate(start), End: mustDate(end), RawDate: raw}
}

func spring(title, raw string, start, end string) Event {
	return Event{TermID: "2265", TermName: "Spring 2027", Title: title, Start: mustDate(start), End: mustDate(end), RawDate: raw}
}

func mustDate(s string) time.Time {
	t, err := time.Parse(dateLayout, s)
	if err != nil {
		panic(err)
	}
	return t
}

func parseFixture(t *testing.T, seasons []string) ([]Event, []Skipped) {
	t.Helper()
	f, err := os.Open("testdata/calendar-2026-2027.html")
	if err != nil {
		t.Fatalf("opening fixture: %v", err)
	}
	defer f.Close()
	events, skipped, err := Parse(f, seasons)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return events, skipped
}

func parseString(t *testing.T, doc string) ([]Event, []Skipped) {
	t.Helper()
	events, skipped, err := Parse(strings.NewReader(doc), []string{"Fall", "Spring"})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return events, skipped
}

func TestParse_Fixture(t *testing.T) {
	want := []Event{
		fall("Classes begin", "August 24, 2026 (Monday)", "2026-08-24", "2026-08-24"),
		fall("First day of Add/Drop period", "August 24, 2026 (Monday)", "2026-08-24", "2026-08-24"),
		fall("Saturday classes begin", "August 29", "2026-08-29", "2026-08-29"),
		fall("Last day of Add/Drop period", "August 31 (Monday)", "2026-08-31", "2026-08-31"),
		fall(`First day to drop from classes with a grade of "W"`, "September 1 (Tuesday)", "2026-09-01", "2026-09-01"),
		fall("Labor Day - University Closed", "September 7 (Monday)", "2026-09-07", "2026-09-07"),
		fall("21-Day Enrollment Reporting (Census Date)", "September 13", "2026-09-13", "2026-09-13"),
		fall("Fall Break - No Classes", "October 12-13", "2026-10-12", "2026-10-13"),
		fall(`Last day to drop from classes with a grade of "W"`, "November 6 (Friday)", "2026-11-06", "2026-11-06"),
		fall("No classes - University closes at 2 p.m.", "November 25", "2026-11-25", "2026-11-25"),
		fall("Thanksgiving Holiday - University closed", "November 26-27", "2026-11-26", "2026-11-27"),
		fall("Last day of classes", "December 7 (Monday)", "2026-12-07", "2026-12-07"),
		fall("Deadline to apply online for Fall 2026 graduation", "December 7 (Monday)", "2026-12-07", "2026-12-07"),
		fall("Reading Day", "December 8 (Tuesday)", "2026-12-08", "2026-12-08"),
		fall("Final exams", "Dec. 9,10,11,14,15,16", "2026-12-09", "2026-12-16"),
		fall("Final grades due", "December 18 (Friday)", "2026-12-18", "2026-12-18"),
		fall("Break between Fall Semester and Spring Semester", "Dec. 19 - Jan. 10", "2026-12-19", "2027-01-10"),
		fall("RIT Designated Employee Holidays", "Dec. 25 - Jan. 1", "2026-12-25", "2027-01-01"),
		spring("Classes begin", "January 11, 2027 (Monday)", "2027-01-11", "2027-01-11"),
		spring("First day of Add/Drop period", "January 11, 2027 (Monday)", "2027-01-11", "2027-01-11"),
		spring("Saturday classes begin", "January 16", "2027-01-16", "2027-01-16"),
		spring("Martin Luther King Jr Day - No classes", "January 18 (Monday)", "2027-01-18", "2027-01-18"),
		spring("Last day of Add/Drop period", "January 19 (Tuesday)", "2027-01-19", "2027-01-19"),
		spring(`First day to drop from classes with a grade of "W"`, "January 20 (Wednesday)", "2027-01-20", "2027-01-20"),
		spring("21-Day Enrollment Reporting (Census Date)", "January 31", "2027-01-31", "2027-01-31"),
		spring("Spring Break - No Classes", "March 7-14", "2027-03-07", "2027-03-14"),
		spring("Last day to apply to graduate to be included in the Commencement Book", "April 1 (Thursday)", "2027-04-01", "2027-04-01"),
		spring(`Last day to drop from classes with a grade of "W"`, "April 2 (Friday)", "2027-04-02", "2027-04-02"),
		spring("Last day of classes", "April 26 (Monday)", "2027-04-26", "2027-04-26"),
		spring("Deadline to apply online for Spring 2027 graduation", "April 26 (Monday)", "2027-04-26", "2027-04-26"),
		spring("Reading Day", "April 27 (Tuesday)", "2027-04-27", "2027-04-27"),
		spring("Final exams", "Apr. 28,29,30, May 3,4,5", "2027-04-28", "2027-05-05"),
		spring("Final grades due", "May 7 (Friday)", "2027-05-07", "2027-05-07"),
		spring("Convocation and Commencement ceremonies", "May 7-8", "2027-05-07", "2027-05-08"),
		spring("Break between Spring Semester and Summer Term", "May 9-11", "2027-05-09", "2027-05-11"),
	}
	got, skipped := parseFixture(t, []string{"Fall", "Spring"})
	if len(skipped) != 0 {
		t.Errorf("got %d skipped, want 0: %+v", len(skipped), skipped)
	}
	for i := range min(len(got), len(want)) {
		if !reflect.DeepEqual(got[i], want[i]) {
			t.Errorf("event %d: got %+v, want %+v", i, got[i], want[i])
		}
	}
	if len(got) != len(want) {
		t.Errorf("got %d events, want %d", len(got), len(want))
	}
}

func TestParse_SkipsShortSessions(t *testing.T) {
	const doc = `<table class="table"><thead><tr><th colspan="2"><span class="h3">FALL 2026:&nbsp;<span class="h3">Short Session 1</span></span></th></tr></thead>
<tbody><tr><td>August 24</td><td>Classes begin</td></tr></tbody></table>`
	got, _ := parseString(t, doc)
	if len(got) != 0 {
		t.Errorf("got %d events, want 0: %+v", len(got), got)
	}
}

func TestParse_SeasonFilter(t *testing.T) {
	got, _ := parseFixture(t, []string{"summer"})
	if len(got) != 15 {
		t.Errorf("got %d events, want 15", len(got))
	}
	for _, e := range got {
		if e.TermID != "2268" || e.TermName != "Summer 2027" {
			t.Errorf("got term %q (%s), want 2268 (Summer 2027) for %q", e.TermID, e.TermName, e.Title)
		}
	}
}

func TestParse_GraduationLabelDropped(t *testing.T) {
	const doc = `<table class="table"><thead><tr><th colspan="2"><span class="h3">FALL 2026&nbsp;(Term ID: 2261)</span></th></tr></thead>
<tbody><tr><td>December 7&nbsp;(Monday)</td><td>Last day of classes<br>
<strong>Graduation:</strong> Deadline to apply online for Fall 2026&nbsp;graduation</td></tr></tbody></table>`
	got, _ := parseString(t, doc)
	want := []string{"Last day of classes", "Deadline to apply online for Fall 2026 graduation"}
	var titles []string
	for _, e := range got {
		titles = append(titles, e.Title)
	}
	if !reflect.DeepEqual(titles, want) {
		t.Errorf("got %q, want %q", titles, want)
	}
}

func TestParse_FootnotesStripped(t *testing.T) {
	const doc = `<table class="table"><thead><tr><th colspan="2"><span class="h3">FALL 2026&nbsp;(Term ID: 2261)</span></th></tr></thead>
<tbody><tr><td>August 31</td><td>Last day of Add/Drop period †</td></tr>
<tr><td>November 26-27</td><td><strong>Thanksgiving Holiday - University closed*</strong></td></tr></tbody></table>`
	got, _ := parseString(t, doc)
	want := []string{"Last day of Add/Drop period", "Thanksgiving Holiday - University closed"}
	if len(got) != len(want) {
		t.Fatalf("got %d events, want %d", len(got), len(want))
	}
	for i, e := range got {
		if e.Title != want[i] {
			t.Errorf("event %d: got %q, want %q", i, e.Title, want[i])
		}
	}
}

func TestParse_UnparseableDateSkipped(t *testing.T) {
	const doc = `<table class="table"><thead><tr><th colspan="2"><span class="h3">SPRING 2027&nbsp;(Term ID: 2265)</span></th></tr></thead>
<tbody><tr><td>TBD</td><td>Mystery one<br>Mystery two</td></tr>
<tr><td>April 27</td><td>Reading Day</td></tr></tbody></table>`
	got, skipped := parseString(t, doc)
	if len(got) != 1 || got[0].Title != "Reading Day" {
		t.Errorf("got %+v, want only Reading Day", got)
	}
	want := []Skipped{
		{TermName: "Spring 2027", RawDate: "TBD", Title: "Mystery one", Reason: `unrecognized date "TBD"`},
		{TermName: "Spring 2027", RawDate: "TBD", Title: "Mystery two", Reason: `unrecognized date "TBD"`},
	}
	if !reflect.DeepEqual(skipped, want) {
		t.Errorf("got %+v, want %+v", skipped, want)
	}
}

func TestParse_DuplicateTitleSuffixed(t *testing.T) {
	const doc = `<table class="table"><thead><tr><th colspan="2"><span class="h3">FALL 2026&nbsp;(Term ID: 2261)</span></th></tr></thead>
<tbody><tr><td>December 8</td><td>Reading Day</td></tr>
<tr><td>December 12</td><td>Reading Day</td></tr></tbody></table>`
	got, _ := parseString(t, doc)
	want := []string{"Reading Day", "Reading Day (2)"}
	if len(got) != len(want) {
		t.Fatalf("got %d events, want %d", len(got), len(want))
	}
	for i, e := range got {
		if e.Title != want[i] {
			t.Errorf("event %d: got %q, want %q", i, e.Title, want[i])
		}
	}
}

func TestParse_BackToTopIgnored(t *testing.T) {
	const doc = `<table class="table"><thead><tr><th colspan="2"><span class="h3">FALL 2026&nbsp;(Term ID: 2261)</span></th></tr></thead>
<tbody><tr><td align="left" colspan="2"><a href="#top">Back to Top</a><br>&nbsp;</td></tr></tbody></table>`
	got, skipped := parseString(t, doc)
	if len(got) != 0 || len(skipped) != 0 {
		t.Errorf("got %d events and %d skipped, want 0 and 0", len(got), len(skipped))
	}
}
