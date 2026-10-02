package scrape

import (
	"fmt"
	"io"
	"log/slog"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"
)

// termRe matches full-term headings such as "FALL 2026 (Term ID: 2261)".
// Requiring the Term ID is what excludes the summer short sessions, whose
// headings carry none.
var termRe = regexp.MustCompile(`(?i)^(fall|spring|summer)\s+(\d{4})\s*\(term id:\s*(\d+)\)`)

// titleCleaner removes footnote daggers and non-breaking spaces from titles.
var titleCleaner = strings.NewReplacer("†", "", " ", " ")

// Parse extracts events from every term table whose season is in seasons
// (case-insensitive). Rows whose date cannot be parsed are returned as
// Skipped rather than failing, so one odd entry never blocks a sync. The
// error is reserved for HTML that cannot be read at all.
func Parse(r io.Reader, seasons []string) ([]Event, []Skipped, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return nil, nil, fmt.Errorf("parsing calendar html: %w", err)
	}
	var events []Event
	var skipped []Skipped
	for _, table := range findAll(doc, func(n *html.Node) bool { return n.Data == "table" && hasClass(n, "table") }) {
		heading := ""
		if thead := first(table, func(n *html.Node) bool { return n.Data == "thead" }); thead != nil {
			if span := first(thead, func(n *html.Node) bool { return n.Data == "span" && hasClass(n, "h3") }); span != nil {
				heading = normalize(text(span))
			}
		}
		m := termRe.FindStringSubmatch(heading)
		if m == nil {
			slog.Debug("skipping table without term id", "heading", heading)
			continue
		}
		if !slices.ContainsFunc(seasons, func(s string) bool { return strings.EqualFold(strings.TrimSpace(s), m[1]) }) {
			slog.Debug("skipping excluded season", "heading", heading)
			continue
		}
		year, _ := strconv.Atoi(m[2])
		termName := strings.ToUpper(m[1][:1]) + strings.ToLower(m[1][1:]) + " " + m[2]
		ev, sk := parseTerm(table, m[3], termName, year)
		events, skipped = append(events, ev...), append(skipped, sk...)
	}
	return events, skipped, nil
}

// parseTerm turns one term table's rows into events. Only rows with exactly
// two cells are entries, which drops the "Back to Top" row (one colspan cell).
func parseTerm(table *html.Node, termID, termName string, year int) ([]Event, []Skipped) {
	var events []Event
	var skipped []Skipped
	var termStart time.Time
	seen := map[string]int{}
	for _, tr := range findAll(table, func(n *html.Node) bool { return n.Data == "tr" && n.Parent != nil && n.Parent.Data == "tbody" }) {
		var tds []*html.Node
		for c := tr.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == html.ElementNode && c.Data == "td" {
				tds = append(tds, c)
			}
		}
		if len(tds) != 2 {
			continue
		}
		rawDate := normalize(text(tds[0]))
		lines := cellLines(tds[1])
		start, end, err := ParseDate(rawDate, year)
		if err != nil {
			for _, line := range lines {
				skipped = append(skipped, Skipped{TermName: termName, RawDate: rawDate, Title: line, Reason: err.Error()})
			}
			continue
		}
		start, end = inferYear(start, end, termStart)
		if termStart.IsZero() {
			termStart = start
		}
		for _, title := range lines {
			e := Event{TermID: termID, TermName: termName, Title: title, Start: start, End: end, RawDate: rawDate}
			seen[e.Key()]++
			if n := seen[e.Key()]; n > 1 {
				slog.Warn("duplicate event title in term", "term", termName, "title", title, "occurrence", n)
				e.Title = fmt.Sprintf("%s (%d)", title, n)
			}
			events = append(events, e)
		}
	}
	return events, skipped
}

// cellLines splits an event cell on <br> into cleaned titles. A <strong>
// ending in ":" is a grouping label like "Graduation:" and is not part of
// any title.
func cellLines(td *html.Node) []string {
	var raw []string
	var sb strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			switch {
			case c.Type == html.TextNode:
				sb.WriteString(c.Data)
			case c.Type != html.ElementNode:
			case c.Data == "br":
				raw = append(raw, sb.String())
				sb.Reset()
			case c.Data == "strong" && strings.HasSuffix(normalize(text(c)), ":"):
			default:
				walk(c)
			}
		}
	}
	walk(td)
	raw = append(raw, sb.String())
	var lines []string
	for _, l := range raw {
		l = strings.TrimSpace(strings.TrimRight(normalize(titleCleaner.Replace(l)), "*"))
		if l != "" {
			lines = append(lines, l)
		}
	}
	return lines
}

// text concatenates descendant text nodes, treating <br> as a space so
// "August 24,<br>2026" reads naturally. Comments are skipped.
func text(n *html.Node) string {
	var sb strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			switch {
			case c.Type == html.TextNode:
				sb.WriteString(c.Data)
			case c.Type == html.ElementNode && c.Data == "br":
				sb.WriteByte(' ')
			case c.Type == html.ElementNode:
				walk(c)
			}
		}
	}
	walk(n)
	return sb.String()
}

// normalize turns non-breaking spaces into spaces and collapses whitespace.
func normalize(s string) string {
	return strings.Join(strings.Fields(strings.ReplaceAll(s, " ", " ")), " ")
}

// findAll returns element descendants of n matching pred, in document order.
func findAll(n *html.Node, pred func(*html.Node) bool) []*html.Node {
	var out []*html.Node
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && pred(c) {
			out = append(out, c)
		}
		out = append(out, findAll(c, pred)...)
	}
	return out
}

// first returns the first element descendant of n matching pred, or nil.
func first(n *html.Node, pred func(*html.Node) bool) *html.Node {
	if all := findAll(n, pred); len(all) > 0 {
		return all[0]
	}
	return nil
}

// hasClass reports whether n's class attribute contains the token class.
func hasClass(n *html.Node, class string) bool {
	for _, a := range n.Attr {
		if a.Key == "class" && slices.Contains(strings.Fields(a.Val), class) {
			return true
		}
	}
	return false
}
