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

	"github.com/ikopke/rit-cal-sync/internal/config"
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
	_ = dryRun

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
