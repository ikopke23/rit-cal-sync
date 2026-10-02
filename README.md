# rit-cal-sync

Syncs the RIT academic calendar into a Google Calendar. It scrapes the Fall and
Spring term tables on [rit.edu/calendar](https://www.rit.edu/calendar) and
reconciles them as all-day events, with reminders off.

It's a one-shot CLI: run it, it fetches the page once, creates/updates/deletes
what changed, prints a summary, and exits. No daemon, no scheduler. Run it by
hand once a year, or from cron if you like.

```
internal/
  scrape/          # fetch rit.edu/calendar, parse term tables + date expressions
  gcal/            # OAuth console flow, Calendar API client, reconcile plan
  config/          # env vars + optional .env
cmd/rit-cal-sync/  # entry point: flags, wiring, exit codes
```

## How it decides what to touch

Every event the tool creates has a description like this:

```
made by rit-cal-tool
term: 2261 (Fall 2026)
```

That marker is how the tool knows an event is its own.

- **Unmarked events are never touched.** Anything you add by hand is safe.
- **Don't edit the description of a tool event.** If the first line no longer
  matches exactly, the tool treats it as foreign: it leaves it alone and creates
  a fresh copy next to it.
- **Deletes only happen for terms still on the page.** If an entry disappears
  from a term that RIT still lists, its event is deleted. Once a term rolls off
  the page, its events are left as they are, so past years are kept.
- Identity is Term ID + title. A changed date means an update; an unchanged run
  makes zero writes.

## Google Cloud setup

You need your own OAuth client. This is a one-time setup in the
[Google Cloud console](https://console.cloud.google.com/).

1. Create (or pick) a project and **enable the Google Calendar API**.
2. Configure the **OAuth consent screen**: user type External, publishing status
   Testing. Add your own Google account as a **test user**.
3. Create an **OAuth client ID** of type **Desktop app**.
4. Download the client JSON and save it as `secrets/credentials.json`.

The only scope requested is `calendar.events`.

**Tokens expire after 7 days in Testing mode.** That's fine for how this tool is
meant to be used: a later run just asks for consent again. It does mean cron
runs stop working a week after the last consent (see [Cron](#cron)).

### The consent flow

On the first run (or whenever the saved token is missing or expired) the tool
prints a Google sign-in URL and waits for input:

1. Open the URL and approve access.
2. Your browser ends up on `http://127.0.0.1/?code=...` and shows a "can't
   connect" error. **That's expected.** Nothing is listening there.
3. Copy that full URL from the address bar and paste it back into the terminal.

The token is saved to `GOOGLE_TOKEN_FILE` (mode 0600) and reused on later runs.
Because nothing listens on a port, this works inside `docker run -it` without
publishing any ports.

## Configuration

Config comes from environment variables, with no prefix. An optional `.env` file
in the working directory is loaded too; real environment variables win. Copy
`.env.example` to `.env` to get started.

| Variable | Default | Notes |
|---|---|---|
| `CALENDAR_ID` | | Required for sync. The target Google Calendar ID. |
| `GOOGLE_CREDENTIALS_FILE` | | Required for sync. Path to the Desktop OAuth client JSON. |
| `GOOGLE_TOKEN_FILE` | | Required for sync. Where the OAuth token is read and written. |
| `CALENDAR_URL` | `https://www.rit.edu/calendar` | Page to scrape. |
| `TERM_SEASONS` | `Fall,Spring` | Comma-separated seasons to include. |
| `TIMEZONE` | `America/New_York` | Used only for logs. |
| `HTTP_TIMEOUT` | `30s` | Go duration for the page fetch. |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn` or `error`. |

`-parse-only` doesn't need any of the Google variables.

## Running

Requires Go 1.26+.

```bash
make build                                  # → bin/rit-cal-sync
./bin/rit-cal-sync -parse-only -dump-json - # scrape + parse only, print JSON
./bin/rit-cal-sync -dry-run                 # read Google, show the plan, write nothing
./bin/rit-cal-sync                          # sync for real
```

### Docker

The image is a one-shot: the container runs the sync and exits. Secrets are
mounted at `/secrets`, never baked into the image.

```bash
docker build -t rit-cal-sync .
mkdir -p secrets && sudo chown 10001 secrets   # put credentials.json in here
```

The container runs as uid 10001 and must be able to write `token.json` into the
mounted directory. Either `chown 10001` it as above, or run the container as
yourself with `--user $(id -u)`.

In `.env`, point the Google files at the mount:

```bash
CALENDAR_ID=your-calendar-id@group.calendar.google.com
GOOGLE_CREDENTIALS_FILE=/secrets/credentials.json
GOOGLE_TOKEN_FILE=/secrets/token.json
```

Then run it. Keep `-it` so the consent prompt can read your paste:

```bash
docker run -it --rm --env-file .env -v $PWD/secrets:/secrets rit-cal-sync -dry-run
docker run -it --rm --env-file .env -v $PWD/secrets:/secrets rit-cal-sync
```

### Cron

Running non-interactively works as long as a valid token is already in
`secrets/`:

```cron
0 6 1 * * docker run --rm --env-file /home/me/rit-cal-sync/.env -v /home/me/rit-cal-sync/secrets:/secrets rit-cal-sync >> /var/log/rit-cal-sync.log 2>&1
```

Once the token expires (7 days in Testing mode), a cron run can't prompt for
consent. It fails fast with "no valid OAuth token" and exits 1. Run it once with
`-it` to re-consent.

## Flags

Standard Go flags: single dash, double dash also works.

| Flag | Effect |
|---|---|
| `-dry-run` | Read the calendar and print planned actions. No writes. |
| `-parse-only` | Fetch and parse only. No Google calls, no credentials needed. |
| `-dump-json <path\|->` | Write the parsed events as JSON to a file, or `-` for stdout. |
| `-version` | Print the version and exit. |

## Output and exit codes

The summary goes to stdout; logs go to stderr. With `-dump-json -` the JSON
owns stdout, so the summary moves to stderr and the dump stays pipeable to `jq`.

```
created=3 updated=1 deleted=0 unchanged=31 skipped=0
dry-run: create=3 update=1 delete=0 unchanged=31 skipped=0
parsed=35 skipped=0
```

The first line is a real sync, the second `-dry-run`, the third `-parse-only`.

| Code | Meaning |
|---|---|
| `0` | Success. Includes runs where some entries had unparseable dates: each one is logged as a warning and counted in `skipped`. |
| `1` | Any fatal error: page fetch, OAuth, Google API, zero events parsed, or missing config. |

## Development

```bash
make test        # go test -race ./...
make lint        # golangci-lint
make fmt         # gofumpt -w
```

Parser tests run offline against a saved copy of the calendar page in
`internal/scrape/testdata/`.
