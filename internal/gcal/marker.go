// Package gcal syncs scraped calendar events to Google Calendar. It owns the
// OAuth console flow, the description marker that identifies tool-created
// events, a thin retrying wrapper over the Calendar API, and a pure planner
// that turns desired and existing events into create/update/delete actions.
package gcal

import (
	"regexp"
	"strings"

	"github.com/ikopke/rit-cal-sync/internal/scrape"
)

// markerLine is the first description line of every event this tool owns.
// Listing searches for it, and ParseMarker requires it verbatim so that a
// hand-written event that merely mentions the tool is never touched.
const markerLine = "made by rit-cal-tool"

// termLineRe matches the description line carrying the owning term's ID.
var termLineRe = regexp.MustCompile(`^term: (\d+)(?: \(.*\))?$`)

// Description returns the ownership-marker description for e. It doubles as
// the event's visible description, so it also names the term for humans.
func Description(e scrape.Event) string {
	return markerLine + "\nterm: " + e.TermID + " (" + e.TermName + ")"
}

// ParseMarker reports whether desc carries this tool's ownership marker and,
// if so, which term the event belongs to. The check is strict because it is
// the authoritative ownership test: Google's search is fuzzy, and unmarked
// events must never be modified.
func ParseMarker(desc string) (termID string, ok bool) {
	sawMarker := false
	for line := range strings.Lines(desc) {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if !sawMarker {
			if line != markerLine {
				return "", false
			}
			sawMarker = true
			continue
		}
		if m := termLineRe.FindStringSubmatch(line); m != nil {
			return m[1], true
		}
	}
	return "", false
}
