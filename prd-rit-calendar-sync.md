# PRD: RIT Academic Calendar → Google Calendar Sync

- **Date:** 2026-10-02
- **Status:** Draft
- **Related ticket:** none

## Problem Statement

RIT publishes its academic calendar at https://www.rit.edu/calendar as one long HTML page of dates. It is hard to read and can't be overlaid on a personal schedule. Members of the Computer Science House (CSH) want those dates (term starts, add/drop deadlines, breaks, holidays, finals) as all-day events on the shared CSH Google Calendar. Today that means copying them by hand, which is tedious and goes stale when RIT updates the page.

## Goals

1. Automatically pull every Fall and Spring entry from rit.edu/calendar and turn each into an all-day event on a configured Google Calendar.
2. Make re-runs safe: repeated runs produce no duplicates, apply RIT's edits, and remove entries RIT deleted, without touching events people added by hand.
3. Keep all deployment details (calendar ID, credentials paths, URL, etc.) in environment variables so they can change without code changes.
4. Run equally well by hand and as a one-shot Docker container started by a scheduler (expected about once a year).

**Success looks like:** after one run, the CSH calendar shows every Fall/Spring 2026–2027 entry from the RIT page on the correct dates. A second run reports zero changes.

## Non-Goals (Out of Scope)

- Summer full term and Summer Short Sessions 1 & 2 (deferred; may be added later).
- Past-year calendars (e.g. the linked 2025–2026 page) or the multi-year "future school years" chart. Only the page at the configured URL is synced.
- Graduate 7-week online sessions, RIT's events calendar, the employee holiday calendar.
- Filtering events by type. Every Fall/Spring entry is synced.
- Prefixes or term labels in event titles.
- Event colors, availability (free/busy) settings, or descriptions.
- Webhook, email, or chat alerting.
- A long-running daemon or built-in scheduler.
- Supporting calendars other than Google Calendar, or ICS export.
- A guard that refuses to run when few events are parsed (the user accepted the risk because runs are rare and watched), except the minimal zero-event guard in FR-5.4.

## Functional Requirements

### FR-1 Scraping

- **FR-1.1** MUST fetch the HTML at the URL given by config (default `https://www.rit.edu/calendar`).
- **FR-1.2** MUST identify term sections by their headings (e.g. `FALL 2026 (Term ID: 2261)`, `SPRING 2027 (Term ID: 2265)`) and record each term's name, year and Term ID.
- **FR-1.3** MUST only process term sections whose season is in the configured set (default: Fall, Spring). It MUST ignore Summer and the Summer short sessions by default.
- **FR-1.4** MUST pull out every (date expression, event line) pair in an included section.
- **FR-1.5** When one date expression has several event lines (e.g. Aug 24: "Classes begin" and "First day of Add/Drop period"), MUST produce one separate event per line.
- **FR-1.6** MUST treat sub-labels such as `Graduation:` as grouping labels, not as events. They MUST NOT become events on their own and MUST NOT be added to titles.
- **FR-1.7** MUST strip footnote markers (`†`, `*`) and extra whitespace from event titles.
- **FR-1.8** MUST ignore non-event content: navigation, "Back to Top" links, footnote paragraphs, the "Last updated" line, footer links, and the cookie banner.

### FR-2 Date Parsing

- **FR-2.1** MUST parse a single date with or without a weekday (`September 7 (Monday)`, `August 29`, `September 13`).
- **FR-2.2** MUST parse a first entry where the year sits on its own line (`August 24, / 2026 / (Monday)`).
- **FR-2.3** MUST work out the year for dates that don't state one:
  - Use the term heading's year, or the first entry's year.
  - A Fall term entry in January belongs to the next year (e.g. `Dec. 19 - Jan. 10` in Fall 2026 ends 2027-01-10).
- **FR-2.4** MUST parse same-month day ranges (`October 12-13`, `March 7-14`, `May 7-8`).
- **FR-2.5** MUST parse cross-month and cross-year ranges with abbreviated months (`Dec. 19 - Jan. 10`, `Dec. 25 - Jan. 1`).
- **FR-2.6** MUST parse comma-separated day lists, including lists that switch month (`Dec. 9,10,11,14,15,16`; `Apr. 28,29,30, May 3,4,5`).
- **FR-2.7** MUST accept full month names, abbreviations with or without a period (`Dec.`, `Dec`, `Apr.`), and both hyphen and en-dash range separators, with or without spaces.
- **FR-2.8** If a date expression can't be parsed, MUST log a warning naming the term, the raw date text and the event text, skip that entry, and continue.
  - At the end, MUST print the number of skipped entries.
  - Skipped entries MUST NOT make the run fail; it exits 0 if the sync succeeded. *(decided during planning: warn only)*

### FR-3 Event Model

- **FR-3.1** Every parsed entry MUST become exactly one **all-day** event.
- **FR-3.2** A date range or day list MUST become **one multi-day all-day event** that runs from the first date through the last date, inclusive. For example, Final exams `Dec. 9,10,11,14,15,16` becomes one event from Dec 9 through Dec 16, weekend included.
- **FR-3.3** The event title MUST be the cleaned raw text of the line (e.g. `Classes begin`, `Labor Day - University Closed`), with no prefix.
- **FR-3.4** Events MUST be created with default reminders turned off and no reminders of their own.
- **FR-3.5** An event's identity MUST be **Term ID + title**. A change of date for the same Term ID and title MUST update the existing event, not create a new one.
- **FR-3.6** If two entries in the same term would have the same identity (same title), MUST keep both and tell them apart deterministically, e.g. by order of appearance, so neither is silently dropped. MUST log a warning when this happens. *(added during refinement; no duplicates exist in the 2026–27 Fall/Spring data)*
- **FR-3.7** Each event the tool creates MUST carry a marker in its **description**, with line 1 `made by rit-cal-tool` and line 2 `term: <Term ID> (<Term name>)`. The marker MUST NOT appear in the title. It MUST:
  - (a) identify the event as created by this tool, and
  - (b) store its identity (Term ID + title).

### FR-4 Google Calendar Sync

- **FR-4.1** MUST write to the Google Calendar whose ID is set in config (the CSH calendar).
- **FR-4.2** MUST compare the parsed events with the tool-marked events already on the calendar, then:
  - **create** parsed events that don't exist yet;
  - **update** existing events whose dates differ;
  - **delete** tool-marked events that are no longer on the page, only inside the deletion window (FR-4.3).
- **FR-4.3** **Deletion window:** deletions MUST only consider tool-marked events whose Term ID belongs to a term on the current page.
  - Events from terms no longer on the page (past academic years) MUST be kept.
  - This means the yearly rollover never deletes history.
- **FR-4.4** MUST NOT create, change, or delete any event without the tool's marker. The calendar is shared with events people add by hand.
- **FR-4.5** A run where nothing changed MUST make zero create/update/delete calls.
- **FR-4.6** At the end of a run, MUST print counts of created, updated, deleted, unchanged and skipped events.

### FR-5 CLI & Modes

- **FR-5.1** MUST be a single Go binary that does one full scrape-and-sync run and then exits.
- **FR-5.2** MUST support a **dry-run** flag. In dry-run it scrapes and compares against the live calendar, prints every planned create, update and delete (title, dates, action), and makes no write calls.
- **FR-5.3** MUST support a **JSON dump** flag. It writes the parsed events (term, Term ID, title, start date, end date, raw date text) as JSON to stdout or to a given file path. SHOULD work without Google credentials when combined with a "parse only" or no-sync option.
- **FR-5.4** If zero events are parsed overall, MUST stop before syncing and exit non-zero. This prevents an empty page from deleting the current terms' events. *(added during refinement; a minimal guard, not a threshold)*
- **FR-5.5** Exit codes MUST follow this convention: `0` success, including runs with skipped entries; non-zero on fetch failure, auth failure, Google API failure, or zero events parsed.

### FR-6 Authentication

- **FR-6.1** MUST authenticate to Google as a user via OAuth 2.0, using an OAuth client credentials file whose path is set in config.
- **FR-6.2** MUST store the resulting token, including the refresh token, at a file path set in config, and reuse it on later runs.
- **FR-6.3** If no valid token file exists, MUST start an **interactive consent prompt on the first run**:
  - print the authorization URL;
  - accept the authorization code or redirect;
  - save the token;
  - continue the run.
- **FR-6.4** The consent flow MUST work without a local browser on the host, e.g. inside `docker run -it`.
- **FR-6.5** When the run is not interactive (no TTY) and there's no valid token, MUST fail fast with a clear error explaining how to do the first-run consent, instead of hanging. *(added during refinement)*
- **FR-6.6** MUST request only the narrowest Google Calendar scope needed to read and write events.

### FR-7 Configuration

- **FR-7.1** All deployment settings MUST come from environment variables.
- **FR-7.2** MUST optionally load a `.env` file. Real environment variables MUST take precedence over `.env` values.
- **FR-7.3** A `.env.example` MUST be committed. It lists every variable with a comment and a placeholder or default.
- **FR-7.4** At minimum, these settings MUST be configurable. Names have **no prefix** (decided during planning): `CALENDAR_ID`, `GOOGLE_CREDENTIALS_FILE`, `GOOGLE_TOKEN_FILE`, `CALENDAR_URL`, `TERM_SEASONS`, `TIMEZONE`, `HTTP_TIMEOUT`, `LOG_LEVEL`.

  | Setting | Required | Default |
  |---|---|---|
  | Google Calendar ID (CSH calendar) | yes | — |
  | OAuth client credentials file path | yes | — |
  | OAuth token file path | yes | — |
  | Source URL | no | `https://www.rit.edu/calendar` |
  | Included term seasons | no | `Fall,Spring` |
  | Timezone for all-day dates | no | `America/New_York` |
  | HTTP timeout | no | e.g. 30s |
  | Log level | no | `info` |

- **FR-7.5** If a required setting is missing, MUST exit non-zero before any network call, with an error naming the missing variable.

### FR-8 Packaging

- **FR-8.1** MUST include a Dockerfile that builds a small image running the binary as a **one-shot** container. It runs once, exits with the binary's exit code, and contains no internal scheduler.
- **FR-8.2** Credentials and token files MUST be supplied to the container via mounted volume. The token file's location MUST be writable so first-run consent and token refresh are saved.
- **FR-8.3** Secrets (OAuth client file, token file, `.env`) MUST NOT be baked into the image or committed to the repo. `.gitignore` and `.dockerignore` MUST exclude them.
- **FR-8.4** SHOULD include a README covering:
  - creating the OAuth client in Google Cloud Console;
  - first-run consent with `docker run -it`;
  - an example host cron line;
  - every env var.

## Non-Functional Requirements

- **NFR-1 Performance:** a full run (fetch + sync of about 40 events) SHOULD finish in under 30 seconds on a normal connection.
- **NFR-2 Politeness:** MUST make exactly one HTTP GET to the source URL per run, with a descriptive User-Agent identifying the tool.
- **NFR-3 Reliability:** Google API calls SHOULD retry transient errors (HTTP 429/5xx) with backoff, up to a small fixed number of attempts, before failing.
- **NFR-4 Correctness of all-day events:** events MUST use date-only (all-day) start and end values, with Google's exclusive end date (the last day + 1). They MUST appear on the correct calendar days for viewers in America/New_York.
- **NFR-5 Security:**
  - The token file SHOULD be written with owner-only permissions (0600).
  - Logs MUST NOT print tokens, auth codes, or credential contents.
- **NFR-6 Observability:** MUST log to stdout/stderr with one line per action (create/update/delete/skip) and a final summary. Exit code is the only alerting mechanism.
- **NFR-7 Maintainability:** the scraping/parsing step MUST be separate from the Google sync step, so the parser can be tested without Google.
- **NFR-8 Testing:**
  - MUST include parser unit tests against a committed HTML fixture of the current rit.edu/calendar page (2026–2027).
  - Tests MUST assert the exact list of parsed Fall/Spring events (titles, start and end dates), including the tricky cases: the first entry with the year on its own line, cross-year ranges, comma lists across months, multi-line dates, `Graduation:` labels, and footnote markers.
  - Tests MUST NOT call the network or Google.

## User Workflows

### W1: First-time setup (happy path)
1. User creates an OAuth client (Desktop app) in Google Cloud Console and downloads the credentials JSON.
2. User copies `.env.example` to `.env` and fills in the calendar ID, credentials path and token path.
3. User runs `docker run -it` with the volume mount and `--dry-run`.
4. Tool finds no token, prints the consent URL. User signs in with an account that has write access to the CSH calendar and pastes back the code.
5. Tool saves the token, scrapes, and prints the planned creates (about 40 events).
6. User checks the output, then runs again without `--dry-run`.
7. Tool creates the events and prints `created=N updated=0 deleted=0 unchanged=0 skipped=0`, exit 0.

### W2: Routine re-run (manual or cron)
1. Scheduler runs the one-shot container non-interactively.
2. Tool loads the token, scrapes, compares.
3. Nothing changed: it makes no writes, prints `unchanged=N`, exit 0.
4. RIT moved a date: one update. RIT removed an entry: one delete. RIT added one: one create.

### W3: Academic-year rollover
1. RIT replaces the page with 2027–2028 (new Term IDs).
2. Tool creates the new terms' events. 2026–2027 events stay, because their Term IDs aren't on the page (FR-4.3).

### W4: Debugging the parser
1. User runs with the JSON dump flag (and no sync).
2. User checks the JSON for wrong dates or missing entries without touching Google.

### Error paths
- **E1 Missing env var:** exits non-zero immediately and names the variable.
- **E2 No token under cron (no TTY):** exits non-zero with instructions to run `docker run -it` once.
- **E3 Token revoked or expired beyond refresh:** exits non-zero with an auth error. User deletes the token file and repeats the consent from W1 step 3.
- **E4 rit.edu unreachable / non-200:** exits non-zero, no Google calls made.
- **E5 Page layout changed, zero events parsed:** exits non-zero, no Google calls made.
- **E6 Some dates unparseable:** warns for each one, syncs the rest, exits 0 (FR-2.8).
- **E7 Google API error mid-run:** retries transient errors. If it still fails, exits non-zero. A partial run is fine because the next run reconciles.

## Integration Points

- **rit.edu/calendar (HTTP GET, HTML).** No API or contract; the page structure can change without notice. Term headings follow the pattern `<SEASON> <YEAR> (Term ID: <id>)`. Entries are date-expression / event-line pairs inside each term section.
- **Google Calendar API v3.** Events list, insert, update and delete on one calendar. All-day events use `start.date`/`end.date`. Reminders are turned off. Each tool event carries a hidden marker with its identity (e.g. private extended properties; the mechanism is left to planning).
- **Google OAuth 2.0.** Installed/desktop-app flow with a manual code (works without a browser). Credentials JSON file plus a token JSON file on disk.
- **Docker.** One-shot container. Config comes from env/`.env`, and secrets from mounted files.
- **Host scheduler (cron, k8s CronJob, etc.).** Outside the tool. It only relies on the exit code and stdout/stderr.

## Acceptance Criteria

1. **Given** the committed 2026–2027 HTML fixture, **when** the parser runs with default seasons, **then** it outputs only Fall 2026 and Spring 2027 events, and no Summer events.
2. **Given** the fixture, **when** parsed, **then** Aug 24, 2026 produces two separate events, "Classes begin" and "First day of Add/Drop period", each on 2026-08-24, with no `†` in either title.
3. **Given** the fixture, **when** parsed, **then** "Break between Fall Semester and Spring Semester" spans 2026-12-19 through 2027-01-10 inclusive as one event.
4. **Given** the fixture, **when** parsed, **then** Fall "Final exams" is one event spanning 2026-12-09 through 2026-12-16, and Spring "Final exams" spans 2027-04-28 through 2027-05-05.
5. **Given** the fixture, **when** parsed, **then** no event is titled `Graduation:`, and "Deadline to apply online for Fall 2026 graduation" exists on 2026-12-07.
6. **Given** an empty CSH test calendar, **when** the tool runs, **then** every parsed event exists as an all-day event with the correct dates and no reminders.
7. **Given** the tool just ran successfully, **when** it runs again on the same page, **then** it makes zero write calls and reports all events unchanged.
8. **Given** a hand-made event on the same calendar with the same title as a tool event, **when** the tool runs, **then** the hand-made event is unchanged.
9. **Given** a tool event whose date was changed by hand in Google, **when** the tool runs, **then** the event's dates are restored to match the page and no duplicate is created.
10. **Given** tool events for a Term ID that's no longer on the page, **when** the tool runs, **then** those events are not deleted.
11. **Given** a tool event for a current Term ID whose title is no longer on the page, **when** the tool runs, **then** that event is deleted.
12. **Given** `--dry-run`, **when** the tool runs, **then** it prints the planned actions and the calendar is unchanged.
13. **Given** the JSON dump flag, **when** the tool runs, **then** it outputs valid JSON with term, Term ID, title, start and end dates for every parsed event.
14. **Given** the calendar ID variable is unset, **when** the tool starts, **then** it exits non-zero naming that variable, with no network calls.
15. **Given** no token file and no TTY, **when** the tool runs, **then** it exits non-zero with consent instructions, without hanging.
16. **Given** the Docker image, **when** it's inspected, **then** it contains no credentials, token or `.env` file.

## Open Questions

1. **Interactive first-run vs. one-shot Docker:** the user chose consent on first run rather than a separate `auth` subcommand. That requires the first container run to use `docker run -it`. Confirm this is acceptable, or add an `auth` subcommand later.
2. **Exact env var names and defaults:** deferred by the user ("I will get the specifics when we actually need them"). The CSH calendar ID and the Google Cloud project/OAuth client are still to be provided.
3. **Which Google account does the OAuth consent:** it needs write access to the CSH calendar. If that person leaves CSH, the token stops working. Consider a shared CSH account. (Low-confidence decision: user-OAuth was chosen over a service account.)
4. ~~Google OAuth app publishing status~~ Resolved: the app stays in "Testing" mode. Its 7-day token expiry is acceptable because the tool is expected to be run about once, interactively. Re-consent on a later run is fine.
5. ~~Skipped-entry exit code~~ Resolved: warn only, exit 0.
6. **Event identity = Term ID + title:** if RIT rewords a title, the old event is deleted and a new one created. Accepted, but subscribers may see churn.
7. **Multi-day finals spanning weekends/non-exam days:** the user chose single spanning events, so a calendar view shows "Final exams" on Dec 12–13 even though there are no exams those days. Accepted trade-off; revisit if confusing.
8. **Summer terms:** out of scope now. The season filter is configurable, but summer parsing isn't covered by acceptance tests. Short Session 2's "Last day to Add/Drop classes" wording differs and would be the first thing to check.
9. **Timezone default:** all-day events are date-only, so timezone mostly matters for year/date inference and logging. Confirm `America/New_York` as the default.
