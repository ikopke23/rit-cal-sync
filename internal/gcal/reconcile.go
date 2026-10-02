package gcal

import (
	"cmp"
	"slices"
	"time"

	"github.com/ikopke/rit-cal-sync/internal/scrape"
)

// Existing is a tool-marked event already on the calendar. End is inclusive,
// matching scrape.Event, so the two can be compared directly.
type Existing struct {
	ID, TermID, Title, Description string
	Start, End                     time.Time
}

// Update pairs a calendar event ID with the desired state it should take.
type Update struct {
	ID    string
	Event scrape.Event
}

// Actions is the set of writes needed to make the calendar match the page.
type Actions struct {
	Create    []scrape.Event
	Update    []Update
	Delete    []Existing
	Unchanged int
}

// Plan reconciles desired events against existing tool-marked events. It is
// pure so the sync rules can be tested without a live calendar. Existing
// events are matched by term ID plus title; duplicates beyond the first are
// deleted to heal partial runs, and unmatched events are deleted only when
// their term is still on the page, so older terms are left alone.
func Plan(desired []scrape.Event, existing []Existing) Actions {
	var a Actions
	byKey := make(map[string]Existing, len(existing))
	for _, x := range existing {
		k := x.TermID + "|" + x.Title
		if _, dup := byKey[k]; dup {
			a.Delete = append(a.Delete, x)
			continue
		}
		byKey[k] = x
	}

	currentTerms := make(map[string]bool)
	for _, e := range desired {
		currentTerms[e.TermID] = true
		x, ok := byKey[e.Key()]
		switch {
		case !ok:
			a.Create = append(a.Create, e)
		case !x.Start.Equal(e.Start) || !x.End.Equal(e.End) || x.Description != Description(e):
			a.Update = append(a.Update, Update{ID: x.ID, Event: e})
		default:
			a.Unchanged++
		}
		delete(byKey, e.Key())
	}
	for _, x := range byKey {
		if currentTerms[x.TermID] {
			a.Delete = append(a.Delete, x)
		}
	}

	slices.SortFunc(a.Create, func(p, q scrape.Event) int { return byStartTitle(p.Start, p.Title, q.Start, q.Title) })
	slices.SortFunc(a.Update, func(p, q Update) int {
		return byStartTitle(p.Event.Start, p.Event.Title, q.Event.Start, q.Event.Title)
	})
	slices.SortFunc(a.Delete, func(p, q Existing) int { return byStartTitle(p.Start, p.Title, q.Start, q.Title) })
	return a
}

// byStartTitle orders by start date, then title, for deterministic output.
func byStartTitle(s1 time.Time, t1 string, s2 time.Time, t2 string) int {
	return cmp.Or(s1.Compare(s2), cmp.Compare(t1, t2))
}
