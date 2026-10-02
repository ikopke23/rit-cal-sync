# Plan: RIT Academic Calendar → Google Calendar Sync

A one-shot Go CLI that scrapes the Fall/Spring tables on rit.edu/calendar and reconciles them as all-day events on the CSH Google Calendar.

- **Date:** 2026-10-02
- **Ticket:** none
- **Source PRD:** `prd-rit-calendar-sync.md` (same directory as this plan)
- **Target repo:** `~/Github/rit-cal-sync`, module `github.com/ikopke/rit-cal-sync`, `go 1.26`

---

## Context

### Codebase exploration
- **Greenfield.** No existing code for this tool.
- **No applicable style guide.** `~/repos/STYLEGUIDE.md` does not exist. The only `STYLEGUIDE.md` found (`~/Github/garnish/STYLEGUIDE.md`) is a frontend design system and doesn't apply. Conventions are instead taken from the author's Go repos, `~/Github/shellmate` (newer, preferred) and `~/Github/larder`. See [Style Notes](#style-notes-substitute-for-styleguidemd).
- **Closest prior art:**
  - `~/Github/larder/parser/url.go:21-118`: HTTP fetch with `http.NewRequestWithContext` + timeout, User-Agent, 2xx check, `io.LimitReader`, and a hand-walked DOM using `golang.org/x/net/html`.
  - `~/Github/larder/cmd/ingest.go`: an idempotent, cron-friendly one-shot command that prints a stats summary.

### Target page structure (verified against live HTML, 2026-10-02)
- **One table per term.** Each term is a `<table class="table">`; the page has 5 of them.
- **Heading:** the term heading is `thead th span.h3`, text like `FALL 2026&nbsp;(Term ID: 2261)`.
  - Short-session headings look like `SUMMER 2027:&nbsp;<span>Short Session 1</span>` and have **no Term ID**. This lets a heading regex that requires a Term ID exclude them naturally.
- **Event rows:** each `tbody tr` with **exactly two `td`s** is an event row.
  - `td[0]` is the date expression and may contain `<strong>2026</strong>` and `&nbsp;`.
  - `td[1]` holds the event lines, separated by `<br>`. Lines may be wrapped in `<strong>` and may contain `<strong>Graduation:</strong>` as an inline label.
- **Back to Top row:** a single `td colspan="2"`. Skip it because it doesn't have two cells.
- **Comments:** the old "Note:" paragraph is inside an HTML comment and must be ignored (comment nodes).
- **Date formats seen in Fall/Spring:**
  - `August 24, 2026 (Monday)`, `August 29`, `October 12-13`, `March 7-14`, `May 7-8`
  - `Dec. 9,10,11,14,15,16`
  - `Apr. 28,29,30, May 3,4,5`
  - `Dec. 19 - Jan. 10`, `Dec. 25 - Jan. 1`

---

## Requirements

Priorities are preserved from the PRD. ⚠ marks a change from the PRD made during planning.

### Parsing
| # | Req | Pri |
|---|---|---|
| R1 | Fetch the configured URL once per run, with a descriptive User-Agent and a timeout (FR-1.1, NFR-2) | MUST |
| R2 | Parse term tables. Include only seasons in `TERM_SEASONS` (default `Fall,Spring`) that have a Term ID (FR-1.2, 1.3) | MUST |
| R3 | One event per `<br>`-separated line. Drop `Graduation:` labels, strip `†`/`*`/`&nbsp;`, collapse whitespace (FR-1.4–1.8) | MUST |
| R4 | Parse every date format listed above, inferring years (Fall's January dates go to the next year) (FR-2.1–2.7) | MUST |
| R5 | ⚠ An unparseable date logs a warning (term, raw date, title) and is skipped. Print the skipped count. **Exit 0** (user chose warn-only, overriding the PRD's FR-2.8 SHOULD) | MUST |

### Event model
| # | Req | Pri |
|---|---|---|
| R6 | Each entry becomes one all-day event. Ranges and lists span first→last date inclusive. The title is the raw cleaned text (FR-3.1–3.3) | MUST |
| R7 | Reminders off: `useDefault=false` with no overrides (FR-3.4) | MUST |
| R8 | Identity = Term ID + title. A duplicate title within a term gets an occurrence suffix and a warning (FR-3.5, 3.6) | MUST |
| R9 | ⚠ The ownership marker goes in the **event description**: line 1 `made by rit-cal-tool`, line 2 `term: <id> (<name>)` (user's choice; replaces "hidden marker" in FR-3.7) | MUST |

### Sync
| # | Req | Pri |
|---|---|---|
| R10 | Reconcile into creates, updates (dates changed) and deletes. Deletes only touch marked events whose term ID is on the current page (FR-4.2, 4.3) | MUST |
| R11 | Never touch unmarked events (FR-4.4) | MUST |
| R12 | An unchanged run makes zero writes. Print created/updated/deleted/unchanged/skipped (FR-4.5, 4.6) | MUST |
| R13 | Retry transient Google errors (429/5xx) with backoff, 3 attempts (NFR-3) | SHOULD |

### CLI
| # | Req | Pri |
|---|---|---|
| R14 | `--dry-run` reads Google and prints the planned actions but makes no writes (FR-5.2) | MUST |
| R15 | `--dump-json <path\|->` and `--parse-only` (no Google, no credentials needed) (FR-5.3) | MUST |
| R16 | Zero events parsed means exit 1 before any Google call (FR-5.4) | MUST |
| R17 | Exit 0 on success, 1 on any fatal error (FR-5.5, minus skipped-entry failures) | MUST |

### Auth
| # | Req | Pri |
|---|---|---|
| R18 | User OAuth from the credentials file. The token file is reused and written 0600 (FR-6.1, 6.2, NFR-5) | MUST |
| R19 | ⚠ Missing token, **or a refresh that fails with `invalid_grant`**, starts the interactive console consent when stdin is a TTY. The user accepted re-consent on every run because a "Testing"-mode app's tokens expire after 7 days (FR-6.3, 6.4) | MUST |
| R20 | No TTY and no usable token means exit 1 with instructions (FR-6.5) | MUST |
| R21 | Scope is `calendar.CalendarEventsScope` only (FR-6.6) | MUST |

### Config
| # | Req | Pri |
|---|---|---|
| R22 | ⚠ Env vars **with no prefix**, plus an optional `.env` (real env wins) and a committed `.env.example` (FR-7) | MUST |

### Packaging
| # | Req | Pri |
|---|---|---|
| R23 | Two-stage Dockerfile, alpine, non-root, one-shot. Secrets are mounted, never baked in (FR-8.1–8.3) | MUST |
| R24 | README covering OAuth client setup, `docker run -it` consent, a cron line and the env table (FR-8.4) | SHOULD |
| R25 | Parser tests against a committed HTML fixture (NFR-8) | MUST |

### Out of scope (from the PRD's non-goals, plus this plan)
- Summer terms and short sessions
- Past-year pages
- Event-type filtering
- Colors, free/busy and title prefixes
- Webhooks
- A daemon
- An `auth` subcommand
- cobra
- `.golangci.yml`, CI workflows and Sonar config (only the Makefile is copied)

---

## Approach

The design is a **three-stage pipeline**: `fetch → parse → reconcile/apply`. Each stage lives in its own package and has a pure core.

1. **`internal/scrape`**
   - `Fetch(ctx, url)` returns the HTML bytes.
   - `Parse(r io.Reader, seasons) ([]Event, []Skipped, error)` is pure and uses `x/net/html`.
   - `ParseDate(raw string, termYear int, termStart time.Time)` is pure and table-tested.
2. **`internal/gcal`**
   - `auth.go` handles the OAuth console flow and token storage.
   - `client.go` is a thin wrapper over `calendar/v3` (list marked events, insert, update, delete, with retry).
   - `reconcile.go` has a pure `Plan(desired, existing, currentTermIDs) Actions`.
3. **`cmd/rit-cal-sync/main.go`** handles flags, config, logging and wiring, then exits with the right code.

**Why this shape:** it keeps all Google I/O behind a narrow seam. The parser is fully testable offline (NFR-7), and `--parse-only`/`--dump-json` fall out for free.

### Key decisions and rejected alternatives
- **`x/net/html` over goquery.** User choice. It matches larder, and the tables are trivial to walk.
- **stdlib `flag` over cobra.** User choice. It's a single command, and it matches shellmate.
- **Description marker over extended properties.** User choice.
  - Listing uses `Events.List(calID).Q("made by rit-cal-tool")` with no time bounds, paginated.
  - A strict local parse of the description then confirms each match. Google's `q` search is fuzzy, so the local check is authoritative.
  - *Rejected:* deterministic event IDs. Google won't let an ID be reused after a delete.
- **Console OAuth via loopback-redirect paste.** Google removed the OOB (`urn:ietf:wg:oauth:2.0:oob`) flow in 2022. How it works:
  - Use a Desktop-type client with `RedirectURL: "http://127.0.0.1"`. Print the auth URL, which includes PKCE via `oauth2.GenerateVerifier` and `oauth2.S256ChallengeOption`.
  - The user's browser lands on an unreachable `http://127.0.0.1/?code=…` page. The user pastes either that whole URL or just the code.
  - The tool extracts `code` and exchanges it with `oauth2.VerifierOption`.
  - This works in `docker run -it` without publishing ports (FR-6.4).
  - *Rejected:* a local callback server, because it needs port mapping in Docker.
- **Model dates as civil dates.** Use `time.Time` at 00:00 UTC, formatted with `"2006-01-02"`, and never convert timezones. The inclusive end lives in the model. `gcal` adds 1 day for Google's exclusive `end.date` (NFR-4).
  - `TIMEZONE` is used only to compute "today" for logs. It is optional and kept for the PRD.
- **Year inference.** Start every date with the term heading year. If the result falls more than 60 days before the term's first entry date, add 1 year. This handles Fall `Jan. 10`/`Jan. 1`.
  - The first entry's explicit `<strong>YYYY</strong>`, when present, overrides the heading year for that row.

---

## Style Notes (substitute for STYLEGUIDE.md)

There is no applicable style guide, so these conventions come from shellmate and larder. Every step must follow them.

| Rule | Source |
|---|---|
| Layout: `cmd/rit-cal-sync/main.go` + `internal/<pkg>`, short lowercase package names | shellmate `cmd/shellmate-server/main.go`, `internal/` |
| Errors: `fmt.Errorf("<lowercase gerund phrase>: %w", err)`, e.g. `"fetching calendar page: %w"`. No custom error package. Sentinels only where callers branch (`ErrNoTTY`, `ErrConsentRequired`) | larder `parser/url.go:31,38`; shellmate `internal/server/game.go:17` |
| Logging: `log/slog` TextHandler on stderr, level from `LOG_LEVEL`. **Always use the key `"err"`** (the repos are inconsistent; pick one) | larder `cmd/root.go:30-32` |
| Config: `_ = godotenv.Load() // optional; env vars already set take precedence`, then `os.Getenv`. A missing required var returns `fmt.Errorf("CALENDAR_ID environment variable is required")`. **Use a `Config` struct returned from `config.Load()`, not package globals** (larder `server.go:30-33` is the anti-pattern) | larder `db/db.go:151`, `server/server.go:42-49` |
| main: `run(ctx) error`. `main` logs the error and calls `os.Exit(1)` | shellmate `main.go:54-58` |
| Summary output goes to **stdout** via `fmt.Fprintf(os.Stdout, …)`. Logs go to stderr | larder `cmd/ingest.go:46-48` |
| Context is the first parameter everywhere. Never use `context.Background()` below `main` | both |
| Doc comments on all exported identifiers and packages, ending with periods (godot). Explain the *why* | larder `mealdb/mealdb.go:1-5` |
| Tests: stdlib `testing`, white-box (same package), `tests := []struct{name …}` with `t.Run(tt.name, …)`, failure messages `"got %v, want %v"`, names like `TestParseDate_CrossYearRange` | shellmate `internal/server/lichess_test.go:11-38` |
| Fixtures: `testdata/` for the 95 KB HTML snapshot. This departs from the inline-const habit, which is fine because the file is too big for a const | — |
| Makefile: copy shellmate's, changing `BINARY` and the build path | `~/Github/shellmate/Makefile` |
| Dockerfile: two-stage `golang:1.26-alpine` → `alpine:3.21`, non-root user, `CGO_ENABLED=0` | `~/Github/shellmate/Dockerfile:8-19` |

**Possible conflict:** shellmate's Makefile `lint` target expects `golangci-lint`, but we won't ship a `.golangci.yml`. Keep the target as-is (it uses the default linters). Don't add the config.

---

## Detailed Implementation Steps

### Phase A: Scaffold
1. **Create the repo.**
   - `mkdir ~/Github/rit-cal-sync && go mod init github.com/ikopke/rit-cal-sync`, then set `go 1.26` in `go.mod`.
   - `git init -b main`. See [Version Control](#version-control) for the commit points. The first commit contains only `.gitignore` (step 2).
2. **`.gitignore`**: `bin/`, `.env`, `credentials*.json`, `token*.json`, `secrets/`.
3. **`.dockerignore`**: the same entries plus `.git`, `testdata/` (not needed at build time) and `*.md` except the README.
4. **`Makefile`**: copy `~/Github/shellmate/Makefile` and set `BINARY := rit-cal-sync`. The build target becomes `go build -o bin/$(BINARY) ./cmd/rit-cal-sync`.
5. **Dependencies** (`go get`):
   - `golang.org/x/net/html`
   - `github.com/joho/godotenv`
   - `golang.org/x/oauth2`, `golang.org/x/oauth2/google`
   - `google.golang.org/api/calendar/v3`, `google.golang.org/api/option`, `google.golang.org/api/googleapi`
   - TTY detection uses `golang.org/x/term` (`term.IsTerminal(int(os.Stdin.Fd()))`).

### Phase B: Config (`internal/config`)
6. **`internal/config/config.go`**
   - Define the struct:
     ```go
     type Config struct {
         CalendarID      string        // CALENDAR_ID (required for sync)
         CredentialsFile string        // GOOGLE_CREDENTIALS_FILE (required for sync)
         TokenFile       string        // GOOGLE_TOKEN_FILE (required for sync)
         SourceURL       string        // CALENDAR_URL, default https://www.rit.edu/calendar
         Seasons         []string      // TERM_SEASONS, default "Fall,Spring"; trimmed, case-insensitive
         Timezone        *time.Location // TIMEZONE, default America/New_York
         HTTPTimeout     time.Duration // HTTP_TIMEOUT, default 30s (time.ParseDuration)
         LogLevel        slog.Level    // LOG_LEVEL debug|info|warn|error, default info
     }
     ```
   - `Load(requireGoogle bool) (Config, error)` calls `godotenv.Load()` (ignoring the error), reads the vars and applies defaults.
   - When `requireGoogle` is true, it validates the three Google vars and returns an error naming the first missing one. `--parse-only` passes false (FR-5.3, FR-7.5).
   - Invalid duration, level or timezone values return wrapped errors.
7. **`.env.example`** lists all 8 vars, one comment each. Required vars are left blank and defaults are shown.

### Phase C: Scraper (`internal/scrape`), the first milestone
8. **`internal/scrape/event.go`**
   - Define the model:
     ```go
     type Event struct {
         TermID   string    `json:"term_id"`   // "2261"
         TermName string    `json:"term_name"` // "Fall 2026"
         Title    string    `json:"title"`
         Start    time.Time `json:"start"`     // civil date, 00:00 UTC
         End      time.Time `json:"end"`       // inclusive civil date
         RawDate  string    `json:"raw_date"`
     }
     type Skipped struct{ TermName, RawDate, Title, Reason string }
     ```
   - Add custom `MarshalJSON` (or use string fields) so dates render as `"2006-01-02"`.
   - Add `func (e Event) Key() string { return e.TermID + "|" + e.Title }`.
9. **`internal/scrape/fetch.go`**: `Fetch(ctx context.Context, client *http.Client, url string) ([]byte, error)`.
   - Mirror `larder/parser/url.go:21-60`: `NewRequestWithContext`, `User-Agent: rit-cal-sync/<version> (+github.com/ikopke/rit-cal-sync)`, a 2xx check that errors with `"unexpected status %d"`, and `io.LimitReader(resp.Body, 2<<20)`.
   - Wrap errors as `"fetching calendar page: %w"`.
10. **`internal/scrape/date.go`**
    - `ParseDate(raw string, defaultYear int) (start, end time.Time, err error)`
    - `inferYears(dates, termStart)` for Fall's January dates.
    - Normalization: replace ` ` with a space, replace en/em dashes with `-`, drop the `(Weekday)` parenthetical, and collapse whitespace.
    - A month-token table maps full names and 3-letter abbreviations (optional trailing `.`, plus `Sept`) to `time.Month`.
    - Grammar to handle, in this order:
      1. **Range with two months:** `Mon D - Mon D`.
      2. **Same-month range:** `Mon D-D`.
      3. **List:** `Mon D,D,D[, Mon D,D]`. A month token switches the current month. The result is first→last.
      4. **Single:** `Mon D[, YYYY]`. An explicit year overrides the default.
    - Anything else returns `fmt.Errorf("unrecognized date %q", raw)`.
    - Validate the day against the month length with `time.Date` round-trip.
    - Cross-year: if `end < start` within one expression (Dec→Jan), the end year is +1.
11. **`internal/scrape/parse.go`**: `Parse(r io.Reader, seasons []string) ([]Event, []Skipped, error)`.
    - **Find term tables.** Walk the tree for `table` elements with class `table`. Inside each, find `thead` → text of the first `span.h3`.
      - Match `termRe = (?i)^(fall|spring|summer)\s+(\d{4})\s*\(term id:\s*(\d+)\)`.
      - No match (short sessions) means skip, logged at debug.
      - A season not in `seasons` means skip.
    - **Read rows.** For each `tbody > tr` with exactly 2 `td` element children:
      - Date text = `textContent(td[0])`, normalized.
      - Lines = split `td[1]`'s children on `<br>` nodes. Accumulate text per segment so that `<strong>` text is included. **Skip a `<strong>` whose trimmed text ends in `:`** (the `Graduation:` label).
      - Clean each line: strip `†`, a trailing `*`, and ` `; collapse whitespace; trim. Drop empty lines.
    - **Dates.** Parse the date once per row and attach it to every line in the row (FR-1.5). Track the term's first row date as `termStart` for year inference.
      - A parse failure appends one `Skipped` per line in that row and continues.
    - **Duplicates.** Within a term, a repeated `Key()` gets ` (2)` appended to the Title (then `(3)`, …) and a `slog.Warn` (R8).
    - Ignore comment nodes. Text extraction is a recursive `html.TextNode` concatenation, like larder's DOM walk.
12. **`internal/scrape/testdata/calendar-2026-2027.html`**: save the live page with `curl -sL https://www.rit.edu/calendar` and commit it.

### Phase D: CLI skeleton, milestone 1 shippable (`--parse-only`)
13. **`cmd/rit-cal-sync/main.go`**
    - Flags (stdlib `flag`): `-dry-run`, `-parse-only`, `-dump-json string` (`""` = off, `"-"` = stdout, otherwise a file path), `-version`.
    - Structure: `func main() { if err := run(ctx); err != nil { slog.Error("rit-cal-sync failed", "err", err); os.Exit(1) } }`.
    - `ctx` comes from `signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)`.
    - `run` does the following, in order:
      1. `config.Load(!parseOnly)`.
      2. Set up the slog handler.
      3. Fetch, then Parse.
      4. Log each skip at Warn.
      5. If `len(events)==0`, return `errors.New("no events parsed; page layout may have changed")` (R16).
      6. Write the JSON dump if requested (`json.MarshalIndent`; when writing to a file, use `os.WriteFile(path, b, 0o644)`).
      7. If `parseOnly`, print `parsed=N skipped=M` to stdout and return nil.
      8. Otherwise continue to Phase E's sync.
    - Skips never cause a non-zero exit (R5).

### Phase E: Google sync (`internal/gcal`), milestone 2
14. **`internal/gcal/auth.go`**
    - `func NewService(ctx context.Context, credsFile, tokenFile string, in io.Reader, out io.Writer, interactive bool) (*calendar.Service, error)`. The steps:
      1. Read creds and call `google.ConfigFromJSON(b, calendar.CalendarEventsScope)`, then set `cfg.RedirectURL = "http://127.0.0.1"`.
      2. Load the token from `tokenFile` (JSON-decode `oauth2.Token`). If missing, set `needConsent`.
      3. If a token exists, call `cfg.TokenSource(ctx, tok).Token()` **eagerly**. On a `*oauth2.RetrieveError` with `ErrorCode == "invalid_grant"`, set `needConsent` (R19). Other errors are wrapped: `"refreshing oauth token: %w"`.
      4. If `needConsent && !interactive`, return `ErrConsentRequired`, whose message reads: `no valid OAuth token; run once interactively: docker run -it … (see README)` (R20).
      5. If `needConsent && interactive`, call `consent(ctx, cfg, in, out)`:
         - `verifier := oauth2.GenerateVerifier()`.
         - Print `cfg.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.ApprovalForce, oauth2.S256ChallengeOption(verifier))` to `out`, where `state` is random hex.
         - Ask the user to paste the redirected URL or the code.
         - Parse it: if the input parses as a URL with a `code` query, check that `state` matches and take the code; otherwise treat the input as the raw code.
         - `cfg.Exchange(ctx, code, oauth2.VerifierOption(verifier))`.
         - Save the token: `os.WriteFile(tokenFile, b, 0o600)`, creating the parent dir with `0o700` (NFR-5).
      6. Return `calendar.NewService(ctx, option.WithTokenSource(cfg.TokenSource(ctx, tok)))`.
    - Never log the token, code or URL query (NFR-5).
    - `main` passes `interactive = term.IsTerminal(int(os.Stdin.Fd()))`.
15. **`internal/gcal/marker.go`**
    - `const markerLine = "made by rit-cal-tool"`.
    - `func Description(e scrape.Event) string` returns `markerLine + "\nterm: " + e.TermID + " (" + e.TermName + ")"`.
    - `func ParseMarker(desc string) (termID string, ok bool)` requires the first non-empty line to equal `markerLine` exactly and a `term: <digits>` line. This strict check is the authoritative ownership test (R9, R11).
16. **`internal/gcal/client.go`**
    - `type Client struct{ svc *calendar.Service; calID string }`.
    - `ListMarked(ctx) ([]Existing, error)` calls `svc.Events.List(calID).Q(markerLine).ShowDeleted(false).SingleEvents(true).MaxResults(250).Pages(ctx, fn)`.
      - Keep only items where `ParseMarker` succeeds and `Start.Date != ""` (all-day).
      - Map each to `Existing{ID, TermID, Title: Summary, Start, End(inclusive = end.date-1), Description}`.
    - `Insert`, `Update` and `Delete` build a `*calendar.Event`:
      - `Summary`, `Description`, `Start{Date}`, `End{Date: inclusiveEnd+1day}`.
      - `Reminders: &calendar.EventReminders{UseDefault: false, ForceSendFields: []string{"UseDefault"}}`. **`ForceSendFields` is required**, or `false` is omitted and Google applies the defaults (R7).
      - `Transparency` is left at the default (out of scope).
    - Each call is wrapped in `withRetry(ctx, fn)`: 3 attempts with backoff of 1s and 2s (plus jitter), retrying only when `errors.As(err, &gerr *googleapi.Error)` and the code is 429 or ≥500. It respects `ctx.Done()` (R13).
17. **`internal/gcal/reconcile.go`**: a pure function.
    ```go
    type Actions struct{ Create []scrape.Event; Update []Update; Delete []Existing; Unchanged int }
    type Update struct{ ID string; Event scrape.Event }
    func Plan(desired []scrape.Event, existing []Existing) Actions
    ```
    - `currentTerms` = the set of `TermID` values in `desired`.
    - Index `existing` by `TermID|Title`. If several existing events share a key (e.g. from a crashed partial run), keep the first and put the extras in `Delete`. This self-heals duplicates.
    - For each desired event:
      - no match → Create;
      - match with a different Start, End or Description → Update;
      - otherwise → Unchanged.
    - Any unmatched existing event whose `TermID ∈ currentTerms` → Delete. **Unmatched events from other terms are left alone** (FR-4.3, R10).
    - Sort each slice by (Start, Title) for deterministic output.
18. **Wire it into `main.go`** (continuing from step 13):
    1. `gcal.NewService`, then `ListMarked`, then `Plan`.
    2. Log each action at Info, e.g. `slog.Info("create", "title", e.Title, "start", …, "end", …)`.
    3. If `dryRun`, print `dry-run: create=… update=… delete=… unchanged=… skipped=…` and return.
    4. Otherwise apply the actions in order: Delete, Update, then Create. On the first error, return it wrapped, e.g. `"creating event %q: %w"`. A partial run is fine because the next run reconciles (PRD E7).
    5. Print `created=… updated=… deleted=… unchanged=… skipped=…` (R12).

### Phase F: Packaging, milestone 3
19. **`Dockerfile`**: copy the shape of `~/Github/shellmate/Dockerfile`.
    ```
    FROM golang:1.26-alpine AS build
    WORKDIR /src
    COPY go.mod go.sum ./
    RUN go mod download
    COPY . .
    RUN CGO_ENABLED=0 go build -ldflags "-s -w -X main.version=$(VERSION)" -o /out/rit-cal-sync ./cmd/rit-cal-sync

    FROM alpine:3.21
    RUN apk add --no-cache ca-certificates tzdata && adduser -D -u 10001 app
    USER app
    WORKDIR /app
    ENV CALENDAR_URL=https://www.rit.edu/calendar TERM_SEASONS=Fall,Spring TIMEZONE=America/New_York
    VOLUME ["/secrets"]
    ENTRYPOINT ["/usr/local/bin/rit-cal-sync"]
    ```
    - `tzdata` is required because `time.LoadLocation` would fail on bare alpine.
    - The README documents `GOOGLE_CREDENTIALS_FILE=/secrets/credentials.json` and `GOOGLE_TOKEN_FILE=/secrets/token.json`, with the mount `-v $PWD/secrets:/secrets`.
    - The mounted dir must be writable by uid 10001. Document `chown 10001` or `--user $(id -u)`.
20. **`README.md`** (R24). Sections:
    - What it does
    - Google Cloud setup: enable the Calendar API; create a Desktop OAuth client; download the JSON; add yourself as a test user; note that tokens expire after 7 days in Testing mode, so a later run will just prompt for consent again.
    - Env var table
    - Local use: `make build && ./bin/rit-cal-sync -parse-only -dump-json -`
    - Docker first/each run: `docker run -it --rm --env-file .env -v $PWD/secrets:/secrets rit-cal-sync -dry-run`
    - Example cron line, with the note that cron runs fail with "consent required" once the token expires
    - Flag reference and exit codes

---

## Testing Strategy

The user asked for parser tests against a fixture as the baseline. I'm also recommending small pure-function tests for `reconcile`, because acceptance criteria 7–11 otherwise need a live calendar. It's about 60 lines and needs no mocks.

### `internal/scrape/date_test.go`: table-driven `TestParseDate`
| name | raw | year | want start → end |
|---|---|---|---|
| single_weekday | `September 7 (Monday)` | 2026 | 2026-09-07 → same |
| single_plain | `August 29` | 2026 | 2026-08-29 |
| explicit_year | `August 24, 2026 (Monday)` | 2025 | 2026-08-24 (explicit wins) |
| same_month_range | `October 12-13` | 2026 | 10-12 → 10-13 |
| range_endash | `March 7–14` | 2027 | 03-07 → 03-14 |
| cross_year_range | `Dec. 19 - Jan. 10` | 2026 | 2026-12-19 → 2027-01-10 |
| list_one_month | `Dec. 9,10,11,14,15,16` | 2026 | 12-09 → 12-16 |
| list_two_months | `Apr. 28,29,30, May 3,4,5` | 2027 | 04-28 → 05-05 |
| abbrev_no_period | `Dec 25 - Jan 1` | 2026 | 2026-12-25 → 2027-01-01 |
| invalid_day | `February 30` | 2027 | error |
| garbage | `TBD` | 2027 | error |

### `internal/scrape/parse_test.go`
- `TestParse_Fixture` opens `testdata/calendar-2026-2027.html`, calls `Parse(f, []string{"Fall","Spring"})`, and compares against a hand-written `want []Event` for **every** Fall and Spring row: about 21 Fall and 22 Spring events, with exact title, start, end and term ID. The rows are visible in the PRD-phase scrape. This one test covers PRD AC-1 through AC-5.
- Extra targeted tests, each a small inline HTML `const` (larder style):
  - `TestParse_SkipsShortSessions`: a heading without a Term ID produces no events.
  - `TestParse_SeasonFilter`: `[]string{"summer"}` on the fixture gives only Summer 2027 (Term 2268) events and no short sessions.
  - `TestParse_GraduationLabelDropped`: no title equals or starts with `Graduation:`.
  - `TestParse_FootnotesStripped`: no title contains `†` or ends with `*`.
  - `TestParse_UnparseableDateSkipped`: a row with `TBD` produces one `Skipped` per line, and the other rows still parse.
  - `TestParse_DuplicateTitleSuffixed`: two `Reading Day` rows in one term give `Reading Day` and `Reading Day (2)`.
  - `TestParse_BackToTopIgnored`: a `colspan=2` row produces nothing.

### `internal/gcal/marker_test.go` and `reconcile_test.go` (recommended)
- `TestParseMarker`: the valid case, a missing marker, the marker not on the first line, a hand-written description that mentions the tool mid-sentence (must be rejected), and a missing term line.
- `TestPlan`, table-driven:
  - empty existing → all Create;
  - identical → all Unchanged, zero writes (AC-7);
  - date moved → Update (AC-9);
  - existing from another term ID not on the page → untouched (AC-10);
  - existing in a current term but not desired → Delete (AC-11);
  - duplicate existing keys → one kept, extras deleted.

### `internal/config/config_test.go`
- Use `t.Setenv` to cover required-var errors naming the variable (AC-14), the defaults, `requireGoogle=false` skipping validation, and a bad `HTTP_TIMEOUT`.

### Manual verification against a scratch Google calendar
Do this before pointing the tool at CSH.
| PRD AC | Procedure |
|---|---|
| AC-6 | Fresh calendar, real run: every event is all-day on the right days, with no bell icon (reminders off) |
| AC-7 | Run again: `created=0 updated=0 deleted=0` |
| AC-8 | Hand-create "Reading Day" with no description, then run: it is untouched |
| AC-9 | Drag a tool event to another day in the UI, then run: `updated=1`, and it's back in place |
| AC-11 | Hand-edit a fixture copy to remove a row and point `CALENDAR_URL` at `file://`, or use `python -m http.server`, then run: `deleted=1` |
| AC-12 | `-dry-run` after the AC-9 drag: the update is listed, and the calendar is unchanged |
| AC-13 | `-parse-only -dump-json - \| jq .` gives valid JSON |
| AC-15 | `docker run --rm` (no `-it`) with no token: exits 1 within seconds with the consent message |
| AC-16 | `docker run --rm --entrypoint sh` is not possible (no shell needed). Instead run `docker image inspect` plus `docker save \| tar -t` and grep for `token`/`credentials`: none found |

**Note on AC-11:** `Fetch` uses `net/http`, which doesn't support `file://`. Use a local `http.server`. Don't add file support.

`make test` runs `go test -race -count=1 ./...`.

---

## Version Control

- **Setup.**
  - `git init` happens in step 1, on branch `main`.
  - Commit `.gitignore` **first, before any other file** so `.env`, credentials and tokens can never be staged.
  - Add the GitHub remote (`github.com/ikopke/rit-cal-sync`) only when the user asks. **Do not push without being told to.**
- **When to commit.** Commit only at a "solid" checkpoint, meaning all three of these hold:
  - `go build ./...`, `go vet ./...` and `go test -race ./...` pass;
  - `gofmt`/`gofumpt` is clean;
  - the step's behavior works end to end.
  - Never commit a broken build or failing tests.
- **Message style.** Imperative subject under 72 chars, an optional body explaining *why*, and the trailer `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.
- **Before each commit, check for secrets.** Run `git status` and `git diff --cached --name-only`, and abort if anything matching `token*.json`, `credentials*.json` or `.env` is staged.

### Planned commits
| # | After step(s) | Commit subject | Gate |
|---|---|---|---|
| 1 | 1–2 | `Add .gitignore for secrets and build output` | file only |
| 2 | 3–5 | `Scaffold module, Makefile, and dependencies` | `go build ./...` |
| 3 | 6–7 | `Add env-based config loading with .env support` | config tests pass |
| 4 | 10 + date tests | `Parse RIT calendar date expressions` | `TestParseDate` passes |
| 5 | 8–9, 11–12 + parse tests | `Scrape Fall/Spring term tables into events` | fixture test passes |
| 6 | 13 | `Add CLI with -parse-only and -dump-json` (**M1**) | manual `-parse-only` run on live page |
| 7 | 14 | `Add OAuth console consent and token storage` | consent works once against a real account |
| 8 | 15–17 + tests | `Add description marker and reconcile planner` | marker/plan tests pass |
| 9 | 16, 18 | `Sync events to Google Calendar with dry-run` (**M2**) | manual AC-6 to AC-12 on a scratch calendar |
| 10 | 19 | `Add one-shot Docker image` | AC-15, AC-16 |
| 11 | 20 | `Document setup, Docker usage, and cron` (**M3**) | README reviewed |

If a step needs a fix after its commit, make a new commit. Don't amend once a later commit exists.

---

## Phasing / Milestones
1. **M1, parser:** Phases A–D. Done when `rit-cal-sync -parse-only -dump-json -` prints correct JSON and the scrape tests pass. No Google needed.
2. **M2, sync:** Phase E. Done when the manual AC-6 to AC-12 checks pass against a scratch calendar.
3. **M3, packaging:** Phase F. Done when AC-15 and AC-16 pass and the first real run against CSH happens with `-dry-run` first.

---

## Risks & Open Questions

1. **Description marker is user-editable.** Someone editing a tool event's description makes the tool treat it as foreign: it creates a duplicate and leaves the edited one alone. And a hand-typed exact marker would be adopted. This was accepted with the user's choice. Extended properties would remove the risk if it ever bites.
2. **The `Q()` search is fuzzy and indexed.** Google's full-text search may lag or tokenize oddly. The local strict `ParseMarker` prevents false positives, but a false *negative* (index lag right after creation) could cause a duplicate on two back-to-back runs. The duplicate handling in `Plan` heals this on the next run. Low risk for yearly runs.
3. ~~7-day token expiry (Testing mode)~~ **Accepted, not a risk.** The tool is expected to be run about once, interactively. Any later run simply re-prompts for consent (R19). Cron support stays in place (non-TTY fails fast) but isn't a primary use case.
4. **Loopback-paste OAuth UX.** The browser shows "can't connect to 127.0.0.1," which looks like an error. The README and the prompt text must say this is expected. Google could restrict loopback for Desktop clients, though it is currently the recommended flow.
5. **RIT page drift.** A markup change (e.g. dropping `table.table` or the `(Term ID: …)` text) yields zero events, which exits 1 safely. A subtler change, such as a new date format, shows up as skip warnings, but with exit 0 per the user's choice, so a skip under cron can go unnoticed. That's acceptable at yearly cadence with a human watching.
6. **Year inference heuristic** (60 days before term start means +1 year). It's correct for every 2026–27 Fall/Spring row. A Spring term that listed a December date would mis-infer; no such row exists. The fixture test locks the current behavior.
7. **Volume permissions in Docker.** The non-root uid 10001 must be able to write `/secrets/token.json`. This is documented, but it's the most likely first-run stumble.
8. **Fixture freshness.** The fixture is the 2026–27 page. When RIT rolls over, the tests keep passing on the old fixture. Re-snapshot and update `want` yearly if the parser changes.
9. **PRD deviations:** back-ported to `prd-rit-calendar-sync.md` (FR-2.8, FR-3.7, FR-5.5, FR-7.4, E6).
