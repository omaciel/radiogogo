package player

import (
	"context"
	"errors"
	"net/url"
	"slices"
	"strings"
	"testing"
)

func TestValidateRejects(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantMsg string
	}{
		{"leading dash", "-x", "begins with '-'"},
		{"long flag", "--version", "begins with '-'"},
		{"file scheme", "file:///etc/passwd", "not allowed"},
		{"ftp scheme", "ftp://example.com/a.mp3", "not allowed"},
		{"javascript scheme", "javascript:alert(1)", "not allowed"},
		{"no scheme", "example.com/stream", "no scheme"},
		{"empty", "", "empty"},
		{"whitespace only", "   ", "empty"},
		{"no host", "http://", "no host"},
		{"unparsable url", "http://exa\x7fmple.com", "invalid control character"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.url)
			if err == nil {
				t.Fatalf("Validate(%q) = nil, want an error", tt.url)
			}
			if !errors.Is(err, ErrInvalidURL) {
				t.Errorf("Validate(%q) = %v, want an error wrapping ErrInvalidURL", tt.url, err)
			}
			if !strings.Contains(err.Error(), tt.wantMsg) {
				t.Errorf("Validate(%q) = %q, want the message to mention %q", tt.url, err, tt.wantMsg)
			}
		})
	}
}

func TestValidateWrapsURLParseError(t *testing.T) {
	err := Validate("http://exa\x7fmple.com")
	if !errors.Is(err, ErrInvalidURL) {
		t.Fatalf("Validate() = %v, want an error wrapping ErrInvalidURL", err)
	}
	var urlErr *url.Error
	if !errors.As(err, &urlErr) {
		t.Fatalf("Validate() = %v, want errors.As to reach a *url.Error; the double %%w wrap exists to make this work", err)
	}
}

func TestValidateAccepts(t *testing.T) {
	urls := []string{
		"http://stream.example.com/mp3-192",
		"https://stream.example.com/mp3-192",
		"https://example.com/wunc.m3u?token=abc",
		"https://example.com:8000/stream",
	}
	for _, u := range urls {
		t.Run(u, func(t *testing.T) {
			if err := Validate(u); err != nil {
				t.Errorf("Validate(%q) = %v, want nil", u, err)
			}
		})
	}
}

// call records one invocation made through fakeRunner.
type call struct {
	name string
	args []string
}

// fakeRunner records invocations instead of spawning processes, and fails for
// any player named in errs.
type fakeRunner struct {
	calls []call
	errs  map[string]error
}

func (f *fakeRunner) Run(_ context.Context, name string, args ...string) error {
	f.calls = append(f.calls, call{name: name, args: slices.Clone(args)})
	return f.errs[name]
}

func TestPlayPassesSeparatorBeforeURL(t *testing.T) {
	const streamURL = "https://stream.example.com/mp3-192"
	f := &fakeRunner{}

	if err := NewWithRunner(f).Play(context.Background(), streamURL); err != nil {
		t.Fatalf("Play() = %v, want nil", err)
	}
	if len(f.calls) != 1 || f.calls[0].name != "mpg123" {
		t.Fatalf("calls = %+v, want exactly one mpg123 call", f.calls)
	}

	args := f.calls[0].args
	sep := slices.Index(args, "--")
	urlAt := slices.Index(args, streamURL)
	if sep == -1 {
		t.Fatalf("args = %v, want a -- separator so the player stops parsing flags", args)
	}
	if urlAt == -1 {
		t.Fatalf("args = %v, want the stream URL", args)
	}
	if sep > urlAt {
		t.Errorf("args = %v, want -- to come before the URL", args)
	}
}

func TestPlayFallsBackToFFplay(t *testing.T) {
	f := &fakeRunner{errs: map[string]error{"mpg123": errors.New("not installed")}}

	if err := NewWithRunner(f).Play(context.Background(), "https://example.com/s"); err != nil {
		t.Fatalf("Play() = %v, want nil after falling back", err)
	}
	got := []string{f.calls[0].name, f.calls[1].name}
	want := []string{"mpg123", "ffplay"}
	if !slices.Equal(got, want) {
		t.Errorf("tried %v, want %v", got, want)
	}
}

func TestPlayReportsEveryFailure(t *testing.T) {
	f := &fakeRunner{errs: map[string]error{
		"mpg123": errors.New("mpg123 exploded"),
		"ffplay": errors.New("ffplay exploded"),
	}}

	err := NewWithRunner(f).Play(context.Background(), "https://example.com/s")
	if err == nil {
		t.Fatal("Play() = nil, want an error when every player fails")
	}
	for _, want := range []string{"mpg123 exploded", "ffplay exploded"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("Play() = %q, want it to mention %q so a missing binary is distinguishable from a bad stream", err, want)
		}
	}
}

func TestPlayValidatesBeforeSpawning(t *testing.T) {
	f := &fakeRunner{}

	err := NewWithRunner(f).Play(context.Background(), "file:///etc/passwd")
	if !errors.Is(err, ErrInvalidURL) {
		t.Fatalf("Play() = %v, want ErrInvalidURL", err)
	}
	if len(f.calls) != 0 {
		t.Errorf("spawned %+v, want no player to run for an invalid URL", f.calls)
	}
}

func TestPlayFFplayKeepsItsFlags(t *testing.T) {
	const streamURL = "https://example.com/s"
	f := &fakeRunner{errs: map[string]error{"mpg123": errors.New("nope")}}

	if err := NewWithRunner(f).Play(context.Background(), streamURL); err != nil {
		t.Fatalf("Play() = %v", err)
	}
	args := f.calls[1].args
	for _, want := range []string{"-nodisp", "-autoexit", "--"} {
		if !slices.Contains(args, want) {
			t.Errorf("ffplay args = %v, want %q", args, want)
		}
	}
	sep, urlAt := slices.Index(args, "--"), slices.Index(args, streamURL)
	if sep == -1 || urlAt == -1 || sep > urlAt {
		t.Errorf("ffplay args = %v, want -- before the URL", args)
	}
}

func TestPlayStopsOnCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	f := &fakeRunner{errs: map[string]error{"mpg123": errors.New("signal: killed")}}
	cancel()

	err := NewWithRunner(f).Play(ctx, "https://example.com/s")
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Play() = %v, want context.Canceled; a normal stop must not look like a player failure", err)
	}
	if len(f.calls) != 1 {
		t.Errorf("ran %d players, want 1; ffplay must not be tried after cancellation", len(f.calls))
	}
}
