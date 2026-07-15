package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/omaciel/radiogogo/internal/station"
)

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
