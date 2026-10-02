package gcal

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"time"

	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/googleapi"

	"github.com/ikopke/rit-cal-sync/internal/scrape"
)

// dateLayout is the format of Google's all-day start.date and end.date.
const dateLayout = "2006-01-02"

// retryDelays are the waits between attempts; its length plus one is the
// attempt count. It is a variable so tests can run without sleeping.
var retryDelays = []time.Duration{time.Second, 2 * time.Second}

// retryJitter bounds the random extra wait added to each delay so that
// concurrent clients don't retry in lockstep.
var retryJitter = 250 * time.Millisecond

// Client is a thin wrapper over one Google calendar. It keeps the Calendar
// API surface in one place and adds retries for transient errors.
type Client struct {
	svc   *calendar.Service
	calID string
}

// NewClient returns a Client for the calendar calID.
func NewClient(svc *calendar.Service, calID string) *Client {
	return &Client{svc: svc, calID: calID}
}

// ListMarked returns every all-day event on the calendar that carries this
// tool's marker. Google's search narrows the list; ParseMarker decides.
func (c *Client) ListMarked(ctx context.Context) ([]Existing, error) {
	var out []Existing
	err := withRetry(ctx, func() error {
		out = out[:0]
		return c.svc.Events.List(c.calID).Q(markerLine).ShowDeleted(false).SingleEvents(true).
			MaxResults(250).Context(ctx).Pages(ctx, func(evs *calendar.Events) error {
			for _, ev := range evs.Items {
				x, ok, err := toExisting(ev)
				if err != nil {
					return err
				}
				if ok {
					out = append(out, x)
				}
			}
			return nil
		})
	})
	if err != nil {
		return nil, fmt.Errorf("listing marked events: %w", err)
	}
	return out, nil
}

// toExisting converts a Google event, reporting false for events that are not
// tool-marked or not all-day.
func toExisting(ev *calendar.Event) (Existing, bool, error) {
	termID, ok := ParseMarker(ev.Description)
	if !ok || ev.Start == nil || ev.Start.Date == "" || ev.End == nil {
		return Existing{}, false, nil
	}
	start, err := time.Parse(dateLayout, ev.Start.Date)
	if err != nil {
		return Existing{}, false, fmt.Errorf("parsing start date of event %s: %w", ev.Id, err)
	}
	end, err := time.Parse(dateLayout, ev.End.Date)
	if err != nil {
		return Existing{}, false, fmt.Errorf("parsing end date of event %s: %w", ev.Id, err)
	}
	return Existing{
		ID: ev.Id, TermID: termID, Title: ev.Summary, Description: ev.Description,
		Start: start, End: end.AddDate(0, 0, -1),
	}, true, nil
}

// Insert creates e as an all-day event.
func (c *Client) Insert(ctx context.Context, e scrape.Event) error {
	return withRetry(ctx, func() error {
		_, err := c.svc.Events.Insert(c.calID, toGoogle(e)).Context(ctx).Do()
		return err
	})
}

// Update replaces the event id with e. A full replace (not a patch) keeps the
// event exactly as this tool would have created it.
func (c *Client) Update(ctx context.Context, id string, e scrape.Event) error {
	return withRetry(ctx, func() error {
		_, err := c.svc.Events.Update(c.calID, id, toGoogle(e)).Context(ctx).Do()
		return err
	})
}

// Delete removes the event id.
func (c *Client) Delete(ctx context.Context, id string) error {
	return withRetry(ctx, func() error {
		return c.svc.Events.Delete(c.calID, id).Context(ctx).Do()
	})
}

// toGoogle builds the all-day Google event for e. Google's end.date is
// exclusive, so the inclusive end gains a day. UseDefault must be force-sent,
// or the false value is omitted and the calendar's default reminders apply.
func toGoogle(e scrape.Event) *calendar.Event {
	return &calendar.Event{
		Summary:     e.Title,
		Description: Description(e),
		Start:       &calendar.EventDateTime{Date: e.Start.Format(dateLayout)},
		End:         &calendar.EventDateTime{Date: e.End.AddDate(0, 0, 1).Format(dateLayout)},
		Reminders:   &calendar.EventReminders{UseDefault: false, ForceSendFields: []string{"UseDefault"}},
	}
}

// withRetry runs fn, retrying rate-limit and server errors with backoff.
// Other errors are returned at once because retrying them cannot help.
func withRetry(ctx context.Context, fn func() error) error {
	for attempt := 0; ; attempt++ {
		err := fn()
		if err == nil || attempt >= len(retryDelays) || !retryable(err) {
			return err
		}
		d := retryDelays[attempt]
		if retryJitter > 0 {
			d += rand.N(retryJitter)
		}
		t := time.NewTimer(d)
		select {
		case <-ctx.Done():
			t.Stop()
			return ctx.Err()
		case <-t.C:
		}
	}
}

// retryable reports whether err is a transient Google API error.
func retryable(err error) bool {
	var gerr *googleapi.Error
	return errors.As(err, &gerr) && (gerr.Code == 429 || gerr.Code >= 500)
}
