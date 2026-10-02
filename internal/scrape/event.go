// Package scrape fetches the RIT academic calendar page and parses its term
// tables into all-day events. It never talks to Google, so the parser can be
// tested entirely offline against a committed HTML fixture.
package scrape

import (
	"encoding/json"
	"time"
)

// dateLayout is the civil-date format used for JSON output and Google all-day events.
const dateLayout = "2006-01-02"

// Event is one calendar entry. Start and End are civil dates at 00:00 UTC;
// End is inclusive so the model matches the page, and gcal converts it to
// Google's exclusive end.
type Event struct {
	TermID   string    `json:"term_id"`
	TermName string    `json:"term_name"`
	Title    string    `json:"title"`
	Start    time.Time `json:"start"`
	End      time.Time `json:"end"`
	RawDate  string    `json:"raw_date"`
}

// Skipped records an entry whose date could not be parsed, so the caller can
// warn about it without failing the run.
type Skipped struct {
	TermName string
	RawDate  string
	Title    string
	Reason   string
}

// Key returns the event's identity: Term ID plus title.
func (e Event) Key() string { return e.TermID + "|" + e.Title }

// MarshalJSON renders Start and End as plain dates rather than RFC 3339
// timestamps, since a time of day would be misleading for all-day events.
func (e Event) MarshalJSON() ([]byte, error) {
	type alias Event
	return json.Marshal(struct {
		alias
		Start string `json:"start"`
		End   string `json:"end"`
	}{alias(e), e.Start.Format(dateLayout), e.End.Format(dateLayout)})
}
