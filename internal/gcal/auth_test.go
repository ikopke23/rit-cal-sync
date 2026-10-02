package gcal

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"google.golang.org/api/googleapi"
)

func TestExtractCode(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "full URL", input: "http://127.0.0.1/?state=abc&code=4/0Ab-xyz&scope=x\n", want: "4/0Ab-xyz"},
		{name: "raw code", input: "  4/0Ab-xyz\n", want: "4/0Ab-xyz"},
		{name: "state mismatch", input: "http://127.0.0.1/?state=evil&code=4/0Ab", wantErr: true},
		{name: "URL without code", input: "http://127.0.0.1/?state=abc&error=access_denied", wantErr: true},
		{name: "empty", input: "\n", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractCode(tt.input, "abc")
			if (err != nil) != tt.wantErr {
				t.Fatalf("got err %v, want err %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestWithRetry(t *testing.T) {
	retryDelays = []time.Duration{0, 0}
	retryJitter = 0
	t.Cleanup(func() {
		retryDelays = []time.Duration{time.Second, 2 * time.Second}
		retryJitter = 250 * time.Millisecond
	})

	tests := []struct {
		name      string
		errs      []error // returned by successive calls; nil after exhausted
		wantCalls int
		wantErr   bool
	}{
		{name: "retries 503 then succeeds", errs: []error{&googleapi.Error{Code: 503}}, wantCalls: 2},
		{name: "retries 429", errs: []error{&googleapi.Error{Code: 429}, &googleapi.Error{Code: 500}}, wantCalls: 3},
		{name: "does not retry 400", errs: []error{&googleapi.Error{Code: 400}}, wantCalls: 1, wantErr: true},
		{name: "does not retry plain error", errs: []error{errors.New("boom")}, wantCalls: 1, wantErr: true},
		{
			name:      "gives up after 3",
			errs:      []error{&googleapi.Error{Code: 503}, &googleapi.Error{Code: 503}, &googleapi.Error{Code: 503}, nil},
			wantCalls: 3, wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			err := withRetry(t.Context(), func() error {
				calls++
				if calls <= len(tt.errs) {
					return tt.errs[calls-1]
				}
				return nil
			})
			if (err != nil) != tt.wantErr {
				t.Errorf("got err %v, want err %v", err, tt.wantErr)
			}
			if calls != tt.wantCalls {
				t.Errorf("got %d calls, want %d", calls, tt.wantCalls)
			}
		})
	}
}

func TestLoadToken(t *testing.T) {
	dir := t.TempDir()
	tests := []struct {
		name    string
		content *string
		wantNil bool
		wantErr bool
	}{
		{name: "missing", content: nil, wantNil: true},
		{name: "empty", content: new(""), wantNil: true},
		{name: "whitespace", content: new(" \n"), wantNil: true},
		{name: "valid", content: new(`{"access_token":"a","refresh_token":"r"}`)},
		{name: "corrupt", content: new("{not json"), wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(dir, tt.name+".json")
			if tt.content != nil {
				if err := os.WriteFile(path, []byte(*tt.content), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			tok, err := loadToken(path)
			if (err != nil) != tt.wantErr {
				t.Fatalf("got err %v, want err %v", err, tt.wantErr)
			}
			if !tt.wantErr && (tok == nil) != tt.wantNil {
				t.Errorf("got nil token %v, want %v", tok == nil, tt.wantNil)
			}
		})
	}
}
