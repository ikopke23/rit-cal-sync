package gcal

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

// ErrConsentRequired is returned when no usable token exists and stdin is not
// a terminal, so the caller can exit with instructions instead of hanging.
var ErrConsentRequired = errors.New("no valid OAuth token; run once interactively: " +
	"docker run -it --rm --env-file .env -v $PWD/secrets:/secrets rit-cal-sync (see README)")

// redirectURL is the loopback redirect for a Desktop OAuth client. Nothing
// listens there; the user pastes the resulting URL back, which works inside
// docker run -it without publishing ports.
const redirectURL = "http://127.0.0.1"

// NewService returns an authorized Calendar service. It reuses the token in
// tokenFile, refreshing it eagerly so an expired grant (common for apps in
// Google's Testing mode) is detected now and triggers console consent when
// interactive is true, rather than failing mid-sync.
func NewService(ctx context.Context, credsFile, tokenFile string, in io.Reader, out io.Writer, interactive bool) (*calendar.Service, error) {
	b, err := os.ReadFile(credsFile)
	if err != nil {
		return nil, fmt.Errorf("reading oauth credentials: %w", err)
	}
	cfg, err := google.ConfigFromJSON(b, calendar.CalendarEventsScope)
	if err != nil {
		return nil, fmt.Errorf("parsing oauth credentials: %w", err)
	}
	cfg.RedirectURL = redirectURL

	tok, err := loadToken(tokenFile)
	if err != nil {
		return nil, err
	}
	needConsent := tok == nil
	if tok != nil {
		fresh, err := cfg.TokenSource(ctx, tok).Token()
		var rerr *oauth2.RetrieveError
		switch {
		case errors.As(err, &rerr) && rerr.ErrorCode == "invalid_grant":
			needConsent = true
		case err != nil:
			return nil, fmt.Errorf("refreshing oauth token: %w", err)
		case fresh.AccessToken != tok.AccessToken:
			if err := saveToken(tokenFile, fresh); err != nil {
				return nil, err
			}
			tok = fresh
		}
	}

	if needConsent {
		if !interactive {
			return nil, ErrConsentRequired
		}
		if tok, err = consent(ctx, cfg, in, out); err != nil {
			return nil, err
		}
		if err := saveToken(tokenFile, tok); err != nil {
			return nil, err
		}
	}

	svc, err := calendar.NewService(ctx, option.WithTokenSource(cfg.TokenSource(ctx, tok)))
	if err != nil {
		return nil, fmt.Errorf("creating calendar service: %w", err)
	}
	return svc, nil
}

// loadToken reads a saved token, returning nil without error if none exists.
func loadToken(path string) (*oauth2.Token, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading oauth token: %w", err)
	}
	var tok oauth2.Token
	if err := json.Unmarshal(b, &tok); err != nil {
		return nil, fmt.Errorf("decoding oauth token: %w", err)
	}
	return &tok, nil
}

// saveToken writes tok with owner-only permissions, since it grants calendar
// write access.
func saveToken(path string, tok *oauth2.Token) error {
	b, err := json.Marshal(tok)
	if err != nil {
		return fmt.Errorf("encoding oauth token: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("creating token directory: %w", err)
	}
	if err := os.WriteFile(path, b, 0o600); err != nil {
		return fmt.Errorf("writing oauth token: %w", err)
	}
	return nil
}

// consent runs the console OAuth flow: print the auth URL, read back the
// pasted redirect URL (or bare code), and exchange it using PKCE.
func consent(ctx context.Context, cfg *oauth2.Config, in io.Reader, out io.Writer) (*oauth2.Token, error) {
	var sb [16]byte
	if _, err := rand.Read(sb[:]); err != nil {
		return nil, fmt.Errorf("generating oauth state: %w", err)
	}
	state := hex.EncodeToString(sb[:])
	verifier := oauth2.GenerateVerifier()
	authURL := cfg.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.ApprovalForce, oauth2.S256ChallengeOption(verifier))

	fmt.Fprintf(out, "Open this URL in a browser and grant calendar access:\n\n%s\n\n"+
		"Afterwards the browser will show a \"can't connect to 127.0.0.1\" (or \"site can't be reached\") page.\n"+
		"That is expected. Copy that page's full URL from the address bar and paste it here.\n"+
		"Paste URL (or just the code): ", authURL)

	line, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && !(errors.Is(err, io.EOF) && line != "") {
		return nil, fmt.Errorf("reading authorization response: %w", err)
	}
	code, err := extractCode(line, state)
	if err != nil {
		return nil, err
	}
	tok, err := cfg.Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		return nil, fmt.Errorf("exchanging authorization code: %w", err)
	}
	return tok, nil
}

// extractCode pulls the authorization code from the user's pasted input,
// which is either the full redirect URL or the bare code. A URL's state must
// match, guarding against pasting a response from a different attempt.
func extractCode(input, state string) (string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", errors.New("no authorization code entered")
	}
	u, err := url.Parse(input)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return input, nil
	}
	q := u.Query()
	code := q.Get("code")
	if code == "" {
		return "", errors.New("pasted URL has no code parameter")
	}
	if q.Get("state") != state {
		return "", errors.New("state mismatch")
	}
	return code, nil
}
