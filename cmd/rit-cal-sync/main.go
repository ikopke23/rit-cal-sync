// Command rit-cal-sync scrapes the RIT academic calendar and reconciles its
// Fall/Spring entries as all-day events on a Google Calendar. It runs once and
// exits, so it can be driven by hand or by any external scheduler.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/term"

	"github.com/ikopke/rit-cal-sync/internal/config"
	"github.com/ikopke/rit-cal-sync/internal/gcal"
	"github.com/ikopke/rit-cal-sync/internal/scrape"
)

// version is set at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx); err != nil {
		slog.Error("rit-cal-sync failed", "err", err)
		stop()
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	dryRun := flag.Bool("dry-run", false, "read Google and print planned actions without writing")
	parseOnly := flag.Bool("parse-only", false, "scrape and parse only; no Google access or credentials needed")
	dumpJSON := flag.String("dump-json", "", "write parsed events as JSON to `path` (- for stdout)")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()
	if *showVersion {
		fmt.Fprintln(os.Stdout, version)
		return nil
	}

	cfg, err := config.Load(!*parseOnly)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: cfg.LogLevel})))

	page, err := scrape.Fetch(ctx, &http.Client{Timeout: cfg.HTTPTimeout}, cfg.SourceURL)
	if err != nil {
		return err
	}
	events, skipped, err := scrape.Parse(bytes.NewReader(page), cfg.Seasons)
	if err != nil {
		return err
	}
	for _, s := range skipped {
		slog.Warn("skipping unparseable entry", "term", s.TermName, "raw_date", s.RawDate, "title", s.Title, "reason", s.Reason)
	}
	if len(events) == 0 {
		return errors.New("no events parsed; page layout may have changed")
	}
	if *dumpJSON != "" {
		if err := writeJSON(*dumpJSON, events); err != nil {
			return err
		}
	}
	// The summary moves to stderr when stdout carries the JSON dump, so the
	// dump stays pipeable into jq.
	var summary io.Writer = os.Stdout
	if *dumpJSON == "-" {
		summary = os.Stderr
	}
	if *parseOnly {
		fmt.Fprintf(summary, "parsed=%d skipped=%d\n", len(events), len(skipped))
		return nil
	}
	return syncEvents(ctx, cfg, events, len(skipped), *dryRun, summary)
}

// syncEvents reconciles events against the calendar. Writes go deletes first,
// then updates, then creates; a failure part-way is safe because the next run
// reconciles whatever is left.
func syncEvents(ctx context.Context, cfg config.Config, events []scrape.Event, skipped int, dryRun bool, summary io.Writer) error {
	slog.Info("syncing", "calendar", cfg.CalendarID, "events", len(events), "today", time.Now().In(cfg.Timezone).Format(time.DateOnly))
	svc, err := gcal.NewService(ctx, cfg.CredentialsFile, cfg.TokenFile, os.Stdin, os.Stderr, term.IsTerminal(int(os.Stdin.Fd())))
	if err != nil {
		return err
	}
	client := gcal.NewClient(svc, cfg.CalendarID)
	existing, err := client.ListMarked(ctx)
	if err != nil {
		return err
	}
	a := gcal.Plan(events, existing)
	for _, x := range a.Delete {
		slog.Info("delete", "title", x.Title, "term", x.TermID, "start", x.Start.Format(time.DateOnly), "end", x.End.Format(time.DateOnly))
	}
	for _, u := range a.Update {
		slog.Info("update", "title", u.Event.Title, "term", u.Event.TermID, "start", u.Event.Start.Format(time.DateOnly), "end", u.Event.End.Format(time.DateOnly))
	}
	for _, e := range a.Create {
		slog.Info("create", "title", e.Title, "term", e.TermID, "start", e.Start.Format(time.DateOnly), "end", e.End.Format(time.DateOnly))
	}
	if dryRun {
		fmt.Fprintf(summary, "dry-run: create=%d update=%d delete=%d unchanged=%d skipped=%d\n",
			len(a.Create), len(a.Update), len(a.Delete), a.Unchanged, skipped)
		return nil
	}

	for _, x := range a.Delete {
		if err := client.Delete(ctx, x.ID); err != nil {
			return fmt.Errorf("deleting event %q: %w", x.Title, err)
		}
	}
	for _, u := range a.Update {
		if err := client.Update(ctx, u.ID, u.Event); err != nil {
			return fmt.Errorf("updating event %q: %w", u.Event.Title, err)
		}
	}
	for _, e := range a.Create {
		if err := client.Insert(ctx, e); err != nil {
			return fmt.Errorf("creating event %q: %w", e.Title, err)
		}
	}
	fmt.Fprintf(summary, "created=%d updated=%d deleted=%d unchanged=%d skipped=%d\n",
		len(a.Create), len(a.Update), len(a.Delete), a.Unchanged, skipped)
	return nil
}

// writeJSON dumps events to stdout when path is "-", otherwise to a file.
func writeJSON(path string, events []scrape.Event) error {
	b, err := json.MarshalIndent(events, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding events: %w", err)
	}
	b = append(b, '\n')
	if path == "-" {
		_, err = os.Stdout.Write(b)
	} else {
		err = os.WriteFile(path, b, 0o644)
	}
	if err != nil {
		return fmt.Errorf("writing json dump: %w", err)
	}
	return nil
}
