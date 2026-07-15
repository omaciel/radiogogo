package main

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/omaciel/radiogogo/internal/m3u"
	"github.com/omaciel/radiogogo/internal/player"
	"github.com/omaciel/radiogogo/internal/station"
)

// fakeRunner records invocations instead of spawning a real player process.
type fakeRunner struct {
	calls int
}

func (f *fakeRunner) Run(_ context.Context, _ string, _ ...string) error {
	f.calls++
	return nil
}

func TestRunVersion(t *testing.T) {
	var out, errOut bytes.Buffer

	if code := run([]string{"--version"}, &out, &errOut); code != exitOK {
		t.Fatalf("exit = %d, want %d (stderr: %s)", code, exitOK, errOut.String())
	}
	if !strings.Contains(out.String(), "radiogogo") {
		t.Errorf("stdout = %q, want it to name the program", out.String())
	}
	if !strings.Contains(out.String(), version) {
		t.Errorf("stdout = %q, want it to contain the version %q", out.String(), version)
	}
}

func TestRunList(t *testing.T) {
	var out, errOut bytes.Buffer

	if code := run([]string{"--list"}, &out, &errOut); code != exitOK {
		t.Fatalf("exit = %d, want %d (stderr: %s)", code, exitOK, errOut.String())
	}
	for _, s := range station.All() {
		if !strings.Contains(out.String(), s.Name) {
			t.Errorf("--list output omitted station %q", s.Name)
		}
	}
}

func TestRunUsageErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"unknown flag", []string{"--nope"}},
		{"unknown station", []string{"--station", "KEXP"}},
		{"station and url together", []string{"--station", "WUNC", "https://example.com/s"}},
		{"too many urls", []string{"https://a.example.com/s", "https://b.example.com/s"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out, errOut bytes.Buffer
			if code := run(tt.args, &out, &errOut); code != exitUsage {
				t.Errorf("run(%v) = %d, want %d", tt.args, code, exitUsage)
			}
		})
	}
}

func TestSelectTargetNamedStation(t *testing.T) {
	var out bytes.Buffer

	got, err := selectTarget("wunc", nil, &out)
	if err != nil {
		t.Fatalf("selectTarget() = %v, want nil", err)
	}
	want, err := station.Lookup("WUNC")
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if got != want.URL {
		t.Errorf("selectTarget() = %q, want %q", got, want.URL)
	}
}

func TestSelectTargetExplicitURL(t *testing.T) {
	var out bytes.Buffer
	const u = "https://example.com/stream"

	got, err := selectTarget("", []string{u}, &out)
	if err != nil {
		t.Fatalf("selectTarget() = %v, want nil", err)
	}
	if got != u {
		t.Errorf("selectTarget() = %q, want %q", got, u)
	}
}

func TestSelectTargetFallsBackToRandom(t *testing.T) {
	var out bytes.Buffer

	got, err := selectTarget("", nil, &out)
	if err != nil {
		t.Fatalf("selectTarget() = %v, want nil", err)
	}
	if got == "" {
		t.Fatal("selectTarget() = empty, want a random station URL")
	}
	if !strings.Contains(out.String(), "random") {
		t.Errorf("stdout = %q, want it to say a random station was chosen", out.String())
	}
}

func TestSelectTargetUnknownStation(t *testing.T) {
	var out bytes.Buffer

	if got, err := selectTarget("KEXP", nil, &out); err == nil {
		t.Fatalf("selectTarget() = %q, want an error", got)
	}
}

func TestRunHelpPrintsUsageToStdoutAndExitsOK(t *testing.T) {
	for _, flag := range []string{"-h", "--help"} {
		t.Run(flag, func(t *testing.T) {
			var out, errOut bytes.Buffer

			code := run([]string{flag}, &out, &errOut)
			if code != exitOK {
				t.Fatalf("run(%q) = %d, want %d (stderr: %s)", flag, code, exitOK, errOut.String())
			}
			if !strings.Contains(out.String(), "radiogogo plays online radio streams") {
				t.Errorf("stdout = %q, want usage text", out.String())
			}
			if errOut.String() != "" {
				t.Errorf("stderr = %q, want nothing written to stderr for %s", errOut.String(), flag)
			}
		})
	}
}

func TestRunStationEmptyIsUsageError(t *testing.T) {
	var out, errOut bytes.Buffer

	code := run([]string{"--station", ""}, &out, &errOut)
	if code != exitUsage {
		t.Fatalf("run(--station \"\") = %d, want %d", code, exitUsage)
	}
	if !strings.Contains(errOut.String(), "--station") {
		t.Errorf("stderr = %q, want it to mention --station", errOut.String())
	}
}

func TestRunListWithStationIsUsageError(t *testing.T) {
	var out, errOut bytes.Buffer

	code := run([]string{"--list", "--station", "WUNC"}, &out, &errOut)
	if code != exitUsage {
		t.Fatalf("run(--list --station WUNC) = %d, want %d (stdout: %s)", code, exitUsage, out.String())
	}
}

func TestRunListWithURLIsUsageError(t *testing.T) {
	var out, errOut bytes.Buffer

	code := run([]string{"--list", "https://example.com/s"}, &out, &errOut)
	if code != exitUsage {
		t.Fatalf("run(--list <url>) = %d, want %d (stdout: %s)", code, exitUsage, out.String())
	}
}

func TestPlayValidatesPlaylistContent(t *testing.T) {
	// The URL a playlist returns is network content, not something the user
	// typed. It must be validated before it reaches a player.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("#EXTM3U\nhttp://\n"))
	}))
	defer srv.Close()

	resolver := m3u.NewWithClient(srv.Client())
	f := &fakeRunner{}
	p := player.NewWithRunner(f)

	var out bytes.Buffer
	err := play(context.Background(), srv.URL+"/station.m3u", &out, resolver, p)
	if !errors.Is(err, player.ErrInvalidURL) {
		t.Fatalf("play() = %v, want an error wrapping player.ErrInvalidURL", err)
	}
	if f.calls != 0 {
		t.Errorf("player ran %d times, want 0 for an invalid playlist entry", f.calls)
	}
}

func TestPlayReachesPlayerWithResolvedStreamURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("#EXTM3U\nhttps://stream.example.com/real\n"))
	}))
	defer srv.Close()

	resolver := m3u.NewWithClient(srv.Client())
	f := &fakeRunner{}
	p := player.NewWithRunner(f)

	var out bytes.Buffer
	if err := play(context.Background(), srv.URL+"/station.m3u", &out, resolver, p); err != nil {
		t.Fatalf("play() = %v, want nil", err)
	}
	if f.calls == 0 {
		t.Errorf("player ran %d times, want at least 1 for a valid playlist entry", f.calls)
	}
}
