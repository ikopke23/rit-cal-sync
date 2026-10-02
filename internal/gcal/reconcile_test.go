package gcal

import (
	"reflect"
	"testing"
	"time"

	"github.com/ikopke/rit-cal-sync/internal/scrape"
)

func day(m time.Month, d int) time.Time { return time.Date(2026, m, d, 0, 0, 0, 0, time.UTC) }

func ev(term, title string, start, end time.Time) scrape.Event {
	return scrape.Event{TermID: term, TermName: "Term " + term, Title: title, Start: start, End: end}
}

// existingFor returns the calendar copy of e as this tool would have written it.
func existingFor(id string, e scrape.Event) Existing {
	return Existing{ID: id, TermID: e.TermID, Title: e.Title, Description: Description(e), Start: e.Start, End: e.End}
}

func TestPlan(t *testing.T) {
	a := ev("2261", "Classes begin", day(8, 24), day(8, 24))
	b := ev("2261", "Thanksgiving break", day(11, 25), day(11, 29))
	moved := b
	moved.Start, moved.End = day(11, 26), day(11, 30)
	oldTerm := ev("2251", "Old event", day(1, 5), day(1, 5))

	tests := []struct {
		name     string
		desired  []scrape.Event
		existing []Existing
		want     Actions
	}{
		{
			name:    "empty existing creates all",
			desired: []scrape.Event{b, a},
			want:    Actions{Create: []scrape.Event{a, b}},
		},
		{
			name:     "identical is unchanged",
			desired:  []scrape.Event{a, b},
			existing: []Existing{existingFor("1", a), existingFor("2", b)},
			want:     Actions{Unchanged: 2},
		},
		{
			name:     "date moved updates",
			desired:  []scrape.Event{a, b},
			existing: []Existing{existingFor("1", a), existingFor("2", moved)},
			want:     Actions{Update: []Update{{ID: "2", Event: b}}, Unchanged: 1},
		},
		{
			name:     "other term untouched",
			desired:  []scrape.Event{a},
			existing: []Existing{existingFor("1", a), existingFor("9", oldTerm)},
			want:     Actions{Unchanged: 1},
		},
		{
			name:     "removed from current term deletes",
			desired:  []scrape.Event{a},
			existing: []Existing{existingFor("1", a), existingFor("2", b)},
			want:     Actions{Delete: []Existing{existingFor("2", b)}, Unchanged: 1},
		},
		{
			name:     "duplicate existing keeps first",
			desired:  []scrape.Event{a},
			existing: []Existing{existingFor("1", a), existingFor("2", a), existingFor("3", a)},
			want:     Actions{Delete: []Existing{existingFor("2", a), existingFor("3", a)}, Unchanged: 1},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Plan(tt.desired, tt.existing)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}
