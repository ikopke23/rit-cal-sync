package gcal

import (
	"testing"

	"github.com/ikopke/rit-cal-sync/internal/scrape"
)

func TestParseMarker(t *testing.T) {
	tests := []struct {
		name   string
		desc   string
		wantID string
		wantOK bool
	}{
		{name: "valid", desc: "made by rit-cal-tool\nterm: 2261 (Fall 2026)", wantID: "2261", wantOK: true},
		{name: "valid without name", desc: "made by rit-cal-tool\nterm: 2265", wantID: "2265", wantOK: true},
		{name: "leading blank lines", desc: "\n  \nmade by rit-cal-tool\r\nterm: 2261 (Fall 2026)\r\n", wantID: "2261", wantOK: true},
		{name: "missing marker", desc: "term: 2261 (Fall 2026)"},
		{name: "marker not first line", desc: "Reading day\nmade by rit-cal-tool\nterm: 2261"},
		{name: "marker mid-sentence", desc: "This was made by rit-cal-tool originally.\nterm: 2261"},
		{name: "missing term line", desc: "made by rit-cal-tool"},
		{name: "non-numeric term", desc: "made by rit-cal-tool\nterm: fall"},
		{name: "empty", desc: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, ok := ParseMarker(tt.desc)
			if id != tt.wantID || ok != tt.wantOK {
				t.Errorf("got (%q, %v), want (%q, %v)", id, ok, tt.wantID, tt.wantOK)
			}
		})
	}
}

func TestDescription_RoundTrip(t *testing.T) {
	e := scrape.Event{TermID: "2261", TermName: "Fall 2026", Title: "Classes begin"}
	id, ok := ParseMarker(Description(e))
	if !ok || id != e.TermID {
		t.Errorf("got (%q, %v), want (%q, true)", id, ok, e.TermID)
	}
}
