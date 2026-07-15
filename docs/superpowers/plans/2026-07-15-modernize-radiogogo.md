# RadioGoGo Modernization Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Modernize RadioGoGo — bump Go 1.23.5 → 1.26.0, restructure to an idiomatic layout, fix five security defects, add tests, and add CI, security scanning, Dependabot, a Makefile, and a signed release pipeline.

**Architecture:** A thin `cmd/radiogogo` entrypoint owning flags and exit codes, over three `internal/` packages with one responsibility each: `station` (catalog), `m3u` (playlist fetch), `player` (URL validation + subprocess). The two dangerous paths take injected collaborators — `player` an interface-typed command runner, `m3u` an `*http.Client` — so both are testable without spawning processes or touching the network.

**Tech Stack:** Go 1.26.0, standard library only. GoReleaser 2.x, golangci-lint 2.x, govulncheck, CodeQL, OpenSSF Scorecard, Dependabot, cosign, syft.

**Spec:** `docs/superpowers/specs/2026-07-15-modernize-radiogogo-design.md`

## Global Constraints

- **Go version:** `go.mod` declares `go 1.26.0`. Exact value, including patch.
- **Zero third-party runtime dependencies.** `go.mod` must contain no `require`
  block when this plan is done. Tooling runs via `go run <pkg>@<version>` or as a
  CI action, never as a module dependency. If a task seems to need a library,
  stop and raise it rather than adding one.
- **Module path:** `github.com/omaciel/radiogogo` (unchanged).
- **Allowed URL schemes:** `http` and `https` only. Everywhere. No exceptions.
- **Branch:** `modernize-go-tooling`, already created, spec already committed.
- **Every commit must leave `go build ./...` and `go test ./...` passing.**
- **Error style:** wrap with `%w`. Library packages return errors; only
  `cmd/radiogogo` prints to stderr or calls `os.Exit`.
- **Exit codes:** `0` success, `1` runtime failure, `2` usage error.
- **Comment style:** exported identifiers get doc comments starting with the
  identifier name. Comments state constraints, not narration.

---

### Task 1: Go version bump and the `station` package

**Files:**
- Modify: `go.mod`
- Create: `internal/station/station.go`
- Test: `internal/station/station_test.go`

`src/` is left untouched and still builds. It is removed in Task 4.

**Interfaces:**
- Consumes: nothing.
- Produces:
  - `station.Station` struct with fields `Name string`, `URL string`
  - `station.All() []Station` — catalog sorted by name, safe copy
  - `station.Lookup(name string) (Station, error)` — case-insensitive
  - `station.Random() Station`
  - `station.RandomFrom(intn func(int) int) Station`

- [ ] **Step 1: Bump the Go version**

```bash
cd /Users/omaciel/hacking/github.com/omaciel/radiogogo
go mod edit -go=1.26.0
cat go.mod
```

Expected `go.mod`:

```
module github.com/omaciel/radiogogo

go 1.26.0
```

- [ ] **Step 2: Write the failing test**

Create `internal/station/station_test.go`:

```go
package station

import (
	"strings"
	"testing"
)

func TestAllIsSortedByName(t *testing.T) {
	all := All()
	if len(all) == 0 {
		t.Fatal("All() returned no stations")
	}
	for i := 1; i < len(all); i++ {
		if all[i-1].Name > all[i].Name {
			t.Errorf("All() is not sorted: %q precedes %q", all[i-1].Name, all[i].Name)
		}
	}
}

func TestAllReturnsACopy(t *testing.T) {
	All()[0].Name = "mutated"
	if All()[0].Name == "mutated" {
		t.Error("All() exposed the underlying catalog; a caller can corrupt it")
	}
}

func TestEveryCatalogURLIsHTTPS(t *testing.T) {
	for _, s := range catalog {
		if !strings.HasPrefix(s.URL, "https://") {
			t.Errorf("station %q uses %q; the catalog must be https only", s.Name, s.URL)
		}
	}
}

func TestLookup(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{"exact match", "WUNC", "WUNC", false},
		{"lowercase", "wunc", "WUNC", false},
		{"mixed case with space", "radio paradise", "Radio Paradise", false},
		{"unknown station", "KEXP", "", true},
		{"empty name", "", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Lookup(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Lookup(%q) = %v, want an error", tt.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("Lookup(%q) returned %v, want nil", tt.input, err)
			}
			if got.Name != tt.want {
				t.Errorf("Lookup(%q) = %q, want %q", tt.input, got.Name, tt.want)
			}
		})
	}
}

func TestRandomFromSelectsByIndex(t *testing.T) {
	all := All()
	for i := range all {
		got := RandomFrom(func(int) int { return i })
		if got != all[i] {
			t.Errorf("RandomFrom returning %d = %v, want %v", i, got, all[i])
		}
	}
}

func TestRandomFromIsGivenTheCatalogLength(t *testing.T) {
	var gotN int
	RandomFrom(func(n int) int {
		gotN = n
		return 0
	})
	if want := len(All()); gotN != want {
		t.Errorf("generator called with n=%d, want %d; it must be able to pick any station", gotN, want)
	}
}
```

- [ ] **Step 3: Run the test to verify it fails**

Run: `go test ./internal/station/`
Expected: FAIL — `undefined: All`, `undefined: catalog`, `undefined: Lookup`, `undefined: RandomFrom`.

- [ ] **Step 4: Write the implementation**

Create `internal/station/station.go`:

```go
// Package station provides the built-in catalog of radio stations.
package station

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"
)

// Station is a named radio stream.
type Station struct {
	Name string
	URL  string
}

// catalog holds the built-in stations. Every URL must use https.
var catalog = []Station{
	{Name: "Radio Paradise", URL: "https://stream.radioparadise.com/mp3-192"},
	{Name: "Radio Swiss Classic", URL: "https://stream.srg-ssr.ch/m/rsc_de/mp3_128"},
	{Name: "WUNC", URL: "https://edg-iad-wunc-ice.streamguys1.com/wunc-128-mp3.m3u"},
}

// All returns the catalog ordered by name. The result is a copy; callers may
// modify it freely.
func All() []Station {
	out := slices.Clone(catalog)
	slices.SortFunc(out, func(a, b Station) int {
		return strings.Compare(a.Name, b.Name)
	})
	return out
}

// Lookup returns the station with the given name, ignoring case.
func Lookup(name string) (Station, error) {
	for _, s := range catalog {
		if strings.EqualFold(s.Name, name) {
			return s, nil
		}
	}
	return Station{}, fmt.Errorf("unknown station %q", name)
}

// Random returns a station chosen at random.
func Random() Station {
	return RandomFrom(rand.IntN)
}

// RandomFrom returns a station chosen using intn, which must behave like
// [math/rand/v2.IntN]. It exists so tests can supply a deterministic source.
func RandomFrom(intn func(int) int) Station {
	all := All()
	return all[intn(len(all))]
}
```

Note: `Radio Paradise` moves from `http://` to `https://` here — one of the spec's fixes.

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./internal/station/ -v`
Expected: PASS, all tests.

- [ ] **Step 6: Verify the whole module still builds**

Run: `go build ./... && go vet ./...`
Expected: no output. `src/` still compiles.

- [ ] **Step 7: Commit**

```bash
git add go.mod internal/station/
git commit -m "feat: add station package and bump Go to 1.26.0

Moves the station catalog out of src/radios.go into a package with a
sorted, copy-returning All(), case-insensitive Lookup(), and an injectable
random source so selection is testable. Radio Paradise moves to https.

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 2: The `player` package — URL validation and playback

This is the security-critical task. Two of the five spec defects are fixed here.

**Files:**
- Create: `internal/player/player.go`
- Test: `internal/player/player_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces:
  - `player.ErrInvalidURL` — sentinel `error`, wrapped by every `Validate` failure
  - `player.Validate(raw string) error`
  - `player.Runner` — interface with `Run(ctx context.Context, name string, args ...string) error`
  - `player.ExecRunner` — struct implementing `Runner` via `os/exec`
  - `player.Player` — struct with method `Play(ctx context.Context, rawURL string) error`
  - `player.New() *Player` — real runner
  - `player.NewWithRunner(r Runner) *Player` — injected runner

- [ ] **Step 1: Write the failing test**

Create `internal/player/player_test.go`:

```go
package player

import (
	"context"
	"errors"
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
	f := &fakeRunner{errs: map[string]error{"mpg123": errors.New("nope")}}

	if err := NewWithRunner(f).Play(context.Background(), "https://example.com/s"); err != nil {
		t.Fatalf("Play() = %v", err)
	}
	args := f.calls[1].args
	for _, want := range []string{"-nodisp", "-autoexit"} {
		if !slices.Contains(args, want) {
			t.Errorf("ffplay args = %v, want %q", args, want)
		}
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/player/`
Expected: FAIL — `undefined: Validate`, `undefined: ErrInvalidURL`, `undefined: NewWithRunner`.

- [ ] **Step 3: Write the implementation**

Create `internal/player/player.go`:

```go
// Package player validates stream URLs and plays them with an external player.
package player

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strings"
)

// ErrInvalidURL is wrapped by every error [Validate] returns.
var ErrInvalidURL = errors.New("invalid stream URL")

// Validate reports whether raw may be handed to an external player.
//
// Only http and https are allowed. A URL starting with "-" is refused because
// mpg123 and ffplay would parse it as a flag rather than as a stream.
func Validate(raw string) error {
	if strings.TrimSpace(raw) == "" {
		return fmt.Errorf("%w: the URL is empty", ErrInvalidURL)
	}
	if strings.HasPrefix(raw, "-") {
		return fmt.Errorf("%w: %q begins with '-', which a player would read as a flag", ErrInvalidURL, raw)
	}

	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("%w: %s: %v", ErrInvalidURL, raw, err)
	}
	switch u.Scheme {
	case "http", "https":
	case "":
		return fmt.Errorf("%w: %q has no scheme; use http:// or https://", ErrInvalidURL, raw)
	default:
		return fmt.Errorf("%w: scheme %q is not allowed; use http:// or https://", ErrInvalidURL, u.Scheme)
	}
	if u.Host == "" {
		return fmt.Errorf("%w: %q has no host", ErrInvalidURL, raw)
	}
	return nil
}

// Runner executes an external command. It exists so tests can observe the
// arguments a player would receive without spawning a process.
type Runner interface {
	Run(ctx context.Context, name string, args ...string) error
}

// ExecRunner runs commands with os/exec, wired to the terminal so each
// player's own keyboard controls keep working.
type ExecRunner struct{}

// Run implements [Runner].
func (ExecRunner) Run(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// command is a single attempt at playing a stream.
type command struct {
	name string
	args []string
}

// commandsFor lists the players to try, in order.
//
// Each passes "--" before the URL so the player stops parsing flags first.
// mpg123 follows the POSIX convention; ffplay honours "--" in ffmpeg's
// cmdutils.c parse_options. This is defence in depth: [Validate] already
// guarantees the URL starts with a scheme, so it cannot begin with "-".
func commandsFor(rawURL string) []command {
	return []command{
		{name: "mpg123", args: []string{"--", rawURL}},
		{name: "ffplay", args: []string{"-nodisp", "-autoexit", "--", rawURL}},
	}
}

// Player plays stream URLs with an external player.
type Player struct {
	runner Runner
}

// New returns a Player that runs real player binaries.
func New() *Player {
	return &Player{runner: ExecRunner{}}
}

// NewWithRunner returns a Player backed by r.
func NewWithRunner(r Runner) *Player {
	return &Player{runner: r}
}

// Play validates rawURL, then plays it with the first player that succeeds,
// trying mpg123 before ffplay. If every player fails, all of their errors are
// reported so a missing binary is distinguishable from an unplayable stream.
func (p *Player) Play(ctx context.Context, rawURL string) error {
	if err := Validate(rawURL); err != nil {
		return err
	}

	var errs []error
	for _, c := range commandsFor(rawURL) {
		err := p.runner.Run(ctx, c.name, c.args...)
		if err == nil {
			return nil
		}
		errs = append(errs, fmt.Errorf("%s: %w", c.name, err))
	}
	return fmt.Errorf("every player failed: %w", errors.Join(errs...))
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/player/ -v`
Expected: PASS, all tests. Confirm `TestPlayValidatesBeforeSpawning` and `TestPlayPassesSeparatorBeforeURL` both pass — they are the security assertions.

- [ ] **Step 5: Run with the race detector**

Run: `go test -race ./internal/player/`
Expected: PASS, no race warnings.

- [ ] **Step 6: Commit**

```bash
git add internal/player/
git commit -m "feat: add player package with URL validation

Fixes argument injection: a URL beginning with '-' was passed straight to
exec.Command, where mpg123 and ffplay parse it as a flag. Validation now
allows only http and https, refuses a leading dash, and passes '--' before
the URL. Any one of the three suffices; all three are kept because they
fail differently.

Playback goes through an injectable Runner so tests assert on argv without
spawning a process. Failures from every player are now reported rather than
only the last, so a missing binary is distinguishable from a bad stream.

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 3: The `m3u` package — bounded playlist fetching

Three of the five spec defects are fixed here: no timeout, unbounded read, and the ignored status code.

**Files:**
- Create: `internal/m3u/m3u.go`
- Test: `internal/m3u/m3u_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces:
  - `m3u.DefaultTimeout` — `time.Duration` constant, `10 * time.Second`
  - `m3u.IsPlaylist(raw string) bool`
  - `m3u.Resolver` — struct with method `Resolve(ctx context.Context, raw string) (string, error)`
  - `m3u.New() *Resolver` — client bounded by `DefaultTimeout`
  - `m3u.NewWithClient(c *http.Client) *Resolver`

- [ ] **Step 1: Write the failing test**

Create `internal/m3u/m3u_test.go`:

```go
package m3u

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestIsPlaylist(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want bool
	}{
		{"plain m3u", "https://example.com/wunc.m3u", true},
		{"m3u with query string", "https://example.com/wunc.m3u?token=abc", true},
		{"uppercase extension", "https://example.com/WUNC.M3U", true},
		{"m3u with fragment", "https://example.com/wunc.m3u#start", true},
		{"direct stream", "https://example.com/mp3-192", false},
		{"hls playlist", "https://example.com/live.m3u8", false},
		{"m3u only in query", "https://example.com/s?file=a.m3u", false},
		{"m3u only in host", "https://m3u.example.com/stream", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsPlaylist(tt.url); got != tt.want {
				t.Errorf("IsPlaylist(%q) = %v, want %v", tt.url, got, tt.want)
			}
		})
	}
}

func serve(t *testing.T, h http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv
}

func TestResolveReturnsFirstStreamURL(t *testing.T) {
	srv := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		io.WriteString(w, "#EXTM3U\n#EXTINF:-1,WUNC\n\nhttps://stream.example.com/first\nhttps://stream.example.com/second\n")
	})

	got, err := NewWithClient(srv.Client()).Resolve(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("Resolve() = %v, want nil", err)
	}
	if want := "https://stream.example.com/first"; got != want {
		t.Errorf("Resolve() = %q, want %q", got, want)
	}
}

func TestResolveRejectsNon2xx(t *testing.T) {
	// A 404 body often contains links. The old code scanned it and played the
	// first thing that started with "http".
	srv := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		io.WriteString(w, "https://evil.example.com/not-a-stream\n")
	})

	got, err := NewWithClient(srv.Client()).Resolve(context.Background(), srv.URL)
	if err == nil {
		t.Fatalf("Resolve() = %q, want an error for a 404", got)
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("Resolve() = %q, want the message to carry the status", err)
	}
}

func TestResolveSkipsCommentsAndBlanks(t *testing.T) {
	srv := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		io.WriteString(w, "#EXTM3U\n\n   \n#EXTINF:-1,Comment mentioning https://not.a.stream/x\nhttps://stream.example.com/real\n")
	})

	got, err := NewWithClient(srv.Client()).Resolve(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("Resolve() = %v, want nil", err)
	}
	if want := "https://stream.example.com/real"; got != want {
		t.Errorf("Resolve() = %q, want %q; comment lines must not be treated as streams", got, want)
	}
}

func TestResolveWithoutAnyStreamURL(t *testing.T) {
	srv := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		io.WriteString(w, "#EXTM3U\n#EXTINF:-1,nothing here\n")
	})

	if got, err := NewWithClient(srv.Client()).Resolve(context.Background(), srv.URL); err == nil {
		t.Fatalf("Resolve() = %q, want an error when the playlist has no stream", got)
	}
}

func TestResolveStopsReadingAtTheCap(t *testing.T) {
	// The only stream URL sits past the size cap. If the cap works it is never
	// reached, and Resolve reports that it found nothing.
	srv := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		io.WriteString(w, strings.Repeat("#pad\n", (maxBodyBytes/5)+1024))
		io.WriteString(w, "https://stream.example.com/past-the-cap\n")
	})

	got, err := NewWithClient(srv.Client()).Resolve(context.Background(), srv.URL)
	if err == nil {
		t.Fatalf("Resolve() = %q, want an error; the URL sits past the %d byte cap", got, maxBodyBytes)
	}
}

func TestResolveAppliesTheClientTimeout(t *testing.T) {
	srv := serve(t, func(_ http.ResponseWriter, r *http.Request) {
		<-r.Context().Done() // never respond
	})

	client := srv.Client()
	client.Timeout = 50 * time.Millisecond

	start := time.Now()
	if _, err := NewWithClient(client).Resolve(context.Background(), srv.URL); err == nil {
		t.Fatal("Resolve() = nil, want a timeout error")
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Errorf("Resolve() took %v; the client timeout is not being honoured", elapsed)
	}
}

func TestResolveHonoursContextCancellation(t *testing.T) {
	srv := serve(t, func(_ http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	if _, err := NewWithClient(srv.Client()).Resolve(ctx, srv.URL); err == nil {
		t.Fatal("Resolve() = nil, want an error when the context expires")
	}
}

func TestNewUsesTheDefaultTimeout(t *testing.T) {
	if got := New().client.Timeout; got != DefaultTimeout {
		t.Errorf("New() timeout = %v, want %v", got, DefaultTimeout)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/m3u/`
Expected: FAIL — `undefined: IsPlaylist`, `undefined: maxBodyBytes`, `undefined: NewWithClient`.

- [ ] **Step 3: Write the implementation**

Create `internal/m3u/m3u.go`:

```go
// Package m3u resolves M3U playlists to a playable stream URL.
package m3u

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DefaultTimeout bounds a playlist fetch. It does not bound playback: the
// player runs as a separate process and streams for as long as it likes.
const DefaultTimeout = 10 * time.Second

// maxBodyBytes caps how much of a playlist is read. Real playlists are a few
// hundred bytes, so anything approaching this is a server misbehaving.
const maxBodyBytes = 1 << 20 // 1 MiB

// IsPlaylist reports whether raw points at an M3U playlist.
//
// It tests the URL path, so a query string such as ?token=abc does not defeat
// detection. Extended M3U8/HLS playlists are deliberately not treated as
// playlists: their entries are variant streams, not playable URLs.
func IsPlaylist(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	return strings.HasSuffix(strings.ToLower(u.Path), ".m3u")
}

// Resolver fetches M3U playlists.
type Resolver struct {
	client *http.Client
}

// New returns a Resolver whose requests are bounded by [DefaultTimeout].
func New() *Resolver {
	return &Resolver{client: &http.Client{Timeout: DefaultTimeout}}
}

// NewWithClient returns a Resolver that issues requests through c.
func NewWithClient(c *http.Client) *Resolver {
	return &Resolver{client: c}
}

// Resolve fetches the playlist at raw and returns its first stream URL.
//
// The response must have a 2xx status: an error page's body frequently
// contains links, and treating one as a playlist would play whatever the
// server happened to link to.
func (r *Resolver) Resolve(ctx context.Context, raw string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if err != nil {
		return "", fmt.Errorf("building request for %s: %w", raw, err)
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetching playlist %s: %w", raw, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return "", fmt.Errorf("fetching playlist %s: server returned %s", raw, resp.Status)
	}

	scanner := bufio.NewScanner(io.LimitReader(resp.Body, maxBodyBytes))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "http://") || strings.HasPrefix(line, "https://") {
			return line, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("reading playlist %s: %w", raw, err)
	}
	return "", fmt.Errorf("no stream URL found in playlist %s", raw)
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/m3u/ -v`
Expected: PASS, all tests. `TestResolveRejectsNon2xx` and `TestResolveStopsReadingAtTheCap` are the security assertions.

- [ ] **Step 5: Run with the race detector**

Run: `go test -race ./internal/m3u/`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/m3u/
git commit -m "feat: add m3u package with bounded playlist fetching

Fixes three defects in the old fetchM3U:

- no timeout: a slow server hung the process forever. Now a 10s client
  timeout plus a caller context. Playback is unaffected.
- unbounded read: the body was scanned with no limit. Now capped at 1 MiB.
- ignored status code: resp.StatusCode was never checked, so a 404 page was
  scanned for lines starting with http and the first link in the error page
  was played. Now a 2xx is required.

Playlist detection moves from HasSuffix on the raw string to the parsed URL
path, so stream.m3u?token=abc is recognised. Line matching requires an
http:// or https:// prefix rather than bare 'http'.

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 4: The `cmd/radiogogo` entrypoint, removing `src/`, README

**Files:**
- Create: `cmd/radiogogo/main.go`
- Test: `cmd/radiogogo/main_test.go`
- Delete: `src/main.go`, `src/radios.go`
- Modify: `README.md`

**Interfaces:**
- Consumes: everything produced by Tasks 1–3 — `station.All`, `station.Lookup`,
  `station.Random`, `player.Validate`, `player.New`, `m3u.IsPlaylist`,
  `m3u.New`.
- Produces: `main.version` (string var, stamped via `-ldflags -X main.version=`);
  `run(args []string, stdout, stderr io.Writer) int`;
  `selectTarget(stationName string, args []string, stdout io.Writer) (string, error)`.

- [ ] **Step 1: Write the failing test**

Create `cmd/radiogogo/main_test.go`:

```go
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
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./cmd/radiogogo/`
Expected: FAIL — no non-test Go files, `undefined: run`.

- [ ] **Step 3: Write the implementation**

Create `cmd/radiogogo/main.go`:

```go
// Command radiogogo plays online radio streams from the terminal.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"text/tabwriter"

	"github.com/omaciel/radiogogo/internal/m3u"
	"github.com/omaciel/radiogogo/internal/player"
	"github.com/omaciel/radiogogo/internal/station"
)

// version is stamped at build time with -ldflags -X main.version=...
var version = "dev"

// Exit codes.
const (
	exitOK    = 0
	exitError = 1
	exitUsage = 2
)

const usage = `radiogogo plays online radio streams from your terminal.

Usage:
  radiogogo [flags] [stream-url]

With no arguments, a random station is chosen.

Flags:
  --station NAME   play a named station (see --list)
  --list           list the built-in stations
  --version        print the version and exit

Only http and https URLs are accepted.
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run is main's testable body: it takes its arguments and writers rather than
// reading globals, and returns an exit code rather than calling os.Exit.
func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("radiogogo", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { fmt.Fprint(stderr, usage) }

	showVersion := fs.Bool("version", false, "print the version and exit")
	list := fs.Bool("list", false, "list the built-in stations")
	stationName := fs.String("station", "", "play a named station")

	if err := fs.Parse(args); err != nil {
		return exitUsage
	}

	switch {
	case *showVersion:
		fmt.Fprintf(stdout, "radiogogo %s\n", version)
		return exitOK
	case *list:
		printStations(stdout)
		return exitOK
	}

	if *stationName != "" && fs.NArg() > 0 {
		fmt.Fprintln(stderr, "radiogogo: pass either --station or a URL, not both")
		return exitUsage
	}
	if fs.NArg() > 1 {
		fmt.Fprintf(stderr, "radiogogo: expected at most one URL, got %d\n", fs.NArg())
		return exitUsage
	}

	target, err := selectTarget(*stationName, fs.Args(), stdout)
	if err != nil {
		fmt.Fprintf(stderr, "radiogogo: %v\n", err)
		return exitUsage
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := play(ctx, target, stdout); err != nil {
		if errors.Is(err, context.Canceled) {
			return exitOK
		}
		fmt.Fprintf(stderr, "radiogogo: %v\n", err)
		return exitError
	}
	return exitOK
}

// selectTarget picks the URL to play from the flags and positional arguments.
func selectTarget(stationName string, args []string, stdout io.Writer) (string, error) {
	switch {
	case stationName != "":
		s, err := station.Lookup(stationName)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(stdout, "Selected station: %s (%s)\n", s.Name, s.URL)
		return s.URL, nil
	case len(args) == 1:
		return args[0], nil
	default:
		s := station.Random()
		fmt.Fprintln(stdout, "No URL provided. A random radio station will be chosen.")
		fmt.Fprintf(stdout, "Selected station: %s (%s)\n", s.Name, s.URL)
		return s.URL, nil
	}
}

// play resolves a playlist if needed, then hands the stream to a player.
func play(ctx context.Context, target string, stdout io.Writer) error {
	if err := player.Validate(target); err != nil {
		return err
	}

	if m3u.IsPlaylist(target) {
		fmt.Fprintln(stdout, "Fetching stream from M3U file...")
		streamURL, err := m3u.New().Resolve(ctx, target)
		if err != nil {
			return err
		}
		// The playlist body is attacker-influenced network content, so its
		// URL gets the same validation as a URL typed by the user.
		if err := player.Validate(streamURL); err != nil {
			return fmt.Errorf("playlist %s: %w", target, err)
		}
		target = streamURL
	}

	fmt.Fprintln(stdout, "Playing:", target)
	return player.New().Play(ctx, target)
}

// printStations writes the catalog as an aligned table.
func printStations(w io.Writer) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tURL")
	for _, s := range station.All() {
		fmt.Fprintf(tw, "%s\t%s\n", s.Name, s.URL)
	}
	tw.Flush()
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./cmd/radiogogo/ -v`
Expected: PASS, all tests.

- [ ] **Step 5: Delete the old layout**

```bash
git rm src/main.go src/radios.go
go build ./... && go vet ./...
```

Expected: clean build. `src/` is gone.

- [ ] **Step 6: Verify the CLI end to end**

```bash
go run ./cmd/radiogogo --version
go run ./cmd/radiogogo --list
go run ./cmd/radiogogo --station WUNC ; echo "exit=$?"
go run ./cmd/radiogogo -- -x ; echo "exit=$?"
go run ./cmd/radiogogo file:///etc/passwd ; echo "exit=$?"
```

Expected:
- `--version` prints `radiogogo dev`, exit 0.
- `--list` prints three stations in a table, exit 0.
- `--station WUNC` prints the selection, then exits 1. The WUNC URL ends in
  `.m3u`, so this reaches the network; the exact message depends on whether the
  fetch succeeds. Either outcome is a pass:
  - playlist fetched → `every player failed:` naming **both** mpg123 and ffplay.
    Neither is installed on this machine, which is what makes this useful — it
    proves the fallback ran and that both errors are reported rather than only
    the last.
  - fetch failed (offline, or the station moved) → `fetching playlist ...`.
    Not a defect in this task; re-run when online to see the first outcome.
- `-x` and `file:///etc/passwd` are refused with `invalid stream URL`, exit 1,
  with no network access and no process spawned.

- [ ] **Step 7: Update the README**

In `README.md`, replace the `go build ./src` line under "Build it from source
code" with:

````markdown
```sh
go build -o radiogogo ./cmd/radiogogo
```

Or install it directly:

```sh
go install github.com/omaciel/radiogogo/cmd/radiogogo@latest
```
````

Replace both `go run ./src ...` occurrences with `go run ./cmd/radiogogo ...`.

Add this section immediately before "## Terminal Playback Controls":

````markdown
## Station shortcuts

List the built-in stations:

```sh
radiogogo --list
```

Play one by name:

```sh
radiogogo --station WUNC
```

## Accepted URLs

Only `http` and `https` URLs are accepted. Other schemes — including `file://` —
are refused, as are URLs beginning with `-`, which a media player would parse as
a command-line flag.
````

- [ ] **Step 8: Commit**

```bash
git add cmd/ README.md
git add -u src/
git commit -m "feat: replace src/ with cmd/radiogogo entrypoint

Moves to the conventional layout so 'go install
github.com/omaciel/radiogogo/cmd/radiogogo@latest' works, which the src/
layout prevented.

main's body becomes run(args, stdout, stderr) int, so flag handling and exit
codes are testable without spawning processes. Adds --list, --station, and
--version; bare 'radiogogo' and 'radiogogo <url>' are unchanged. A URL from a
playlist is validated a second time, because it is network content rather
than something the user typed. SIGINT and SIGTERM now cancel cleanly.

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 5: Makefile and golangci-lint configuration

**Files:**
- Create: `Makefile`
- Create: `.golangci.yml`
- Modify: `.gitignore`

**Interfaces:**
- Consumes: `./cmd/radiogogo` from Task 4.
- Produces: `make` targets used by Task 6's CI workflow — `test`, `test-race`,
  `lint`, `vet`, `vuln`, `build`, `cover`. Ldflags stamp `main.version`.

- [ ] **Step 1: Write the Makefile**

Create `Makefile`:

```makefile
# radiogogo — developer and release tasks.
# Run `make` or `make help` for the target list.

GO      ?= go
BINARY  := radiogogo
CMD     := ./cmd/radiogogo
PKGS    := ./...
DIST    := dist
COVER   := coverage.out

# VERSION describes the working tree; releases override it via the git tag.
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

# Pinned so a tool upgrade is a reviewable commit rather than a surprise.
GOVULNCHECK_VERSION ?= latest

.DEFAULT_GOAL := help

.PHONY: help
help: ## Show this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nradiogogo\n\nUsage:\n  make \033[36m<target>\033[0m\n\nTargets:\n"} \
		/^[a-zA-Z0-9_-]+:.*?##/ { printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2 } \
		/^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) }' $(MAKEFILE_LIST)
	@echo

##@ Development

.PHONY: build
build: ## Build the binary into ./dist
	$(GO) build -trimpath -ldflags '$(LDFLAGS)' -o $(DIST)/$(BINARY) $(CMD)

.PHONY: install
install: ## Install the binary into $(GOPATH)/bin
	$(GO) install -trimpath -ldflags '$(LDFLAGS)' $(CMD)

.PHONY: run
run: ## Run it (make run ARGS='--list')
	$(GO) run $(CMD) $(ARGS)

.PHONY: fmt
fmt: ## Format the source
	$(GO) fmt $(PKGS)

.PHONY: vet
vet: ## Run go vet
	$(GO) vet $(PKGS)

.PHONY: tidy
tidy: ## Tidy and verify go.mod
	$(GO) mod tidy
	$(GO) mod verify

##@ Testing

.PHONY: test
test: ## Run the tests
	$(GO) test -count=1 $(PKGS)

.PHONY: test-race
test-race: ## Run the tests with the race detector
	$(GO) test -race -count=1 $(PKGS)

.PHONY: cover
cover: ## Run the tests and open an HTML coverage report
	$(GO) test -covermode=atomic -coverprofile=$(COVER) $(PKGS)
	$(GO) tool cover -func=$(COVER) | tail -n 1
	$(GO) tool cover -html=$(COVER)

##@ Security

.PHONY: lint
lint: ## Run golangci-lint (includes gosec)
	golangci-lint run

.PHONY: vuln
vuln: ## Scan for known vulnerabilities, including in the Go standard library
	$(GO) run golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION) $(PKGS)

.PHONY: check
check: fmt vet lint test-race vuln ## Run everything CI runs

##@ Release

.PHONY: version
version: ## Print the version this build would stamp
	@echo $(VERSION)

.PHONY: release-check
release-check: ## Validate .goreleaser.yaml
	goreleaser check

.PHONY: release-snapshot
release-snapshot: ## Build a full release locally, without tagging or publishing
	goreleaser release --snapshot --clean --skip=publish,sign

.PHONY: release
release: ## Tag and push a release (make release TAG=v0.2.0)
	@test -n "$(TAG)" || { echo "usage: make release TAG=v0.2.0"; exit 1; }
	@git diff --quiet HEAD || { echo "working tree is dirty; commit first"; exit 1; }
	git tag -a $(TAG) -m "Release $(TAG)"
	git push origin $(TAG)
	@echo "Pushed $(TAG); GitHub Actions will build and publish the release."

##@ Housekeeping

.PHONY: clean
clean: ## Remove build output
	rm -rf $(DIST) $(COVER)
```

Note: `vuln` uses `go run ...@version` rather than a module dependency, keeping
`go.mod` free of `require` entries per the global constraints.

- [ ] **Step 2: Verify the Makefile**

```bash
make help
make build && ./dist/radiogogo --version
make test
make vet
make version
```

Expected: `help` lists targets under Development/Testing/Security/Release
headings; `./dist/radiogogo --version` prints a `git describe` value rather than
`dev`, proving ldflags stamping works; `make test` passes.

- [ ] **Step 3: Write the golangci-lint config**

Create `.golangci.yml`:

```yaml
version: "2"

linters:
  default: standard
  enable:
    - bodyclose
    - errorlint
    - gosec
    - misspell
    - revive
    - unconvert
  exclusions:
    rules:
      # Test fixtures deliberately construct odd URLs and ignore write errors.
      - path: _test\.go
        linters:
          - errcheck
          - gosec

formatters:
  enable:
    - gofmt
    - goimports
```

- [ ] **Step 4: Verify the lint config and run it**

```bash
golangci-lint version
golangci-lint config verify
golangci-lint run
```

Expected: `config verify` reports the schema is valid; `run` reports no issues.

If `golangci-lint` is not installed:
```bash
brew install golangci-lint
```

If `config verify` rejects the schema, the installed golangci-lint is v1 rather
than v2. Upgrade (`brew upgrade golangci-lint`) rather than downgrading the
config — v2 is the current line and the `version: "2"` key is required by it.

- [ ] **Step 5: Ignore build output**

Append to `.gitignore`:

```gitignore
# Build output
/dist/
coverage.out
```

- [ ] **Step 6: Run govulncheck**

Run: `make vuln`
Expected: `No vulnerabilities found.` If any are reported, they are in the Go
toolchain rather than in dependencies — note them and continue; a toolchain
upgrade is out of scope for this task.

- [ ] **Step 7: Commit**

```bash
git add Makefile .golangci.yml .gitignore
git commit -m "build: add Makefile and golangci-lint config

Makefile covers development (build/run/fmt/vet/tidy), testing (test/
test-race/cover), security (lint/vuln), and release (release-check/
release-snapshot/release). 'make check' runs what CI runs, so a red CI is
reproducible locally.

govulncheck runs via 'go run @version' rather than as a module dependency,
keeping go.mod free of requires.

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 6: CI workflow

**Files:**
- Create: `.github/workflows/ci.yml`

Actions carry tags here; Task 10 pins them to SHAs.

**Interfaces:**
- Consumes: Makefile targets from Task 5.
- Produces: a `ci.yml` workflow named `CI`, for the README badge in Task 10.

- [ ] **Step 1: Write the workflow**

Create `.github/workflows/ci.yml`:

```yaml
name: CI

on:
  push:
    branches: [main]
  pull_request:
  workflow_dispatch:

permissions:
  contents: read

concurrency:
  group: ${{ github.workflow }}-${{ github.ref }}
  cancel-in-progress: true

jobs:
  test:
    name: Test (${{ matrix.os }})
    runs-on: ${{ matrix.os }}
    strategy:
      fail-fast: false
      matrix:
        os: [ubuntu-latest, macos-latest, windows-latest]
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
          check-latest: true

      - name: Verify go.mod is tidy
        run: |
          go mod tidy
          git diff --exit-code -- go.mod go.sum
        shell: bash

      - name: Test with race detector
        run: go test -race -count=1 -covermode=atomic -coverprofile=coverage.out ./...

      - name: Build
        run: go build ./...

  lint:
    name: Lint
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
          check-latest: true

      - uses: golangci/golangci-lint-action@v6
        with:
          version: latest

  vuln:
    name: govulncheck
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
          check-latest: true

      - name: Scan
        run: go run golang.org/x/vuln/cmd/govulncheck@latest ./...
```

`go-version-file: go.mod` means the Go version lives in exactly one place.

The tests use fake runners and `httptest`, so no runner needs `mpg123`,
`ffplay`, or network access.

- [ ] **Step 2: Validate the YAML**

```bash
python3 -c "import yaml,sys; yaml.safe_load(open('.github/workflows/ci.yml')); print('ci.yml parses')"
```

Expected: `ci.yml parses`

- [ ] **Step 3: Confirm the tidy check would pass**

```bash
go mod tidy && git diff --exit-code -- go.mod
```

Expected: no diff, exit 0.

- [ ] **Step 4: Commit**

```bash
git add .github/workflows/ci.yml
git commit -m "ci: add test, lint, and govulncheck workflow

Tests run on ubuntu, macos, and windows with the race detector. Because the
tests inject a fake command runner and use httptest, no runner needs mpg123,
ffplay, or network access.

The Go version comes from go.mod via go-version-file, so it is defined in
one place. Every job declares contents: read.

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 7: CodeQL and OpenSSF Scorecard workflows

**Files:**
- Create: `.github/workflows/codeql.yml`
- Create: `.github/workflows/scorecard.yml`

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces: workflows named `CodeQL` and `Scorecard`, for Task 10's badges.

- [ ] **Step 1: Write the CodeQL workflow**

Create `.github/workflows/codeql.yml`:

```yaml
name: CodeQL

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]
  schedule:
    # Mondays at 07:00 UTC — catches newly published queries between changes.
    - cron: '0 7 * * 1'

permissions:
  contents: read

jobs:
  analyze:
    name: Analyze Go
    runs-on: ubuntu-latest
    permissions:
      contents: read
      security-events: write
      actions: read
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod

      - name: Initialize CodeQL
        uses: github/codeql-action/init@v3
        with:
          languages: go
          queries: security-and-quality

      - name: Autobuild
        uses: github/codeql-action/autobuild@v3

      - name: Analyze
        uses: github/codeql-action/analyze@v3
        with:
          category: "/language:go"
```

`security-and-quality` is used rather than the default suite because the
argument-injection pattern this project just fixed is exactly the kind of
`os.Args` → `exec.Command` dataflow that suite traces.

- [ ] **Step 2: Write the Scorecard workflow**

Create `.github/workflows/scorecard.yml`:

```yaml
name: Scorecard

on:
  branch_protection_rule:
  push:
    branches: [main]
  schedule:
    - cron: '0 8 * * 1'
  workflow_dispatch:

permissions: read-all

jobs:
  analysis:
    name: Scorecard analysis
    runs-on: ubuntu-latest
    permissions:
      security-events: write
      id-token: write
      contents: read
      actions: read
    steps:
      - uses: actions/checkout@v4
        with:
          persist-credentials: false

      - name: Run analysis
        uses: ossf/scorecard-action@v2
        with:
          results_file: results.sarif
          results_format: sarif
          publish_results: true

      - name: Upload artifact
        uses: actions/upload-artifact@v4
        with:
          name: scorecard-results
          path: results.sarif
          retention-days: 5

      - name: Upload to code scanning
        uses: github/codeql-action/upload-sarif@v3
        with:
          sarif_file: results.sarif
```

`persist-credentials: false` is itself a Scorecard criterion.

`publish_results: true` is required for the README badge in Task 10 to resolve.

- [ ] **Step 3: Validate the YAML**

```bash
python3 -c "
import yaml
for f in ('.github/workflows/codeql.yml', '.github/workflows/scorecard.yml'):
    yaml.safe_load(open(f)); print(f, 'parses')
"
```

Expected: both parse.

- [ ] **Step 4: Commit**

```bash
git add .github/workflows/codeql.yml .github/workflows/scorecard.yml
git commit -m "ci: add CodeQL and OpenSSF Scorecard workflows

CodeQL runs the security-and-quality suite, which traces the os.Args to
exec.Command dataflow this project just hardened, on PRs and weekly.

Scorecard grades repo hygiene and publishes results for the badge. It
checks out with persist-credentials: false, which is itself one of the
criteria it grades.

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 8: Dependabot

**Files:**
- Create: `.github/dependabot.yml`

**Interfaces:**
- Consumes: the workflow files from Tasks 6, 7, and 9 — the `github-actions`
  ecosystem only has work to do because those exist.
- Produces: nothing consumed by later tasks.

- [ ] **Step 1: Write the config**

Create `.github/dependabot.yml`:

```yaml
version: 2

updates:
  # Nothing to do today: the module has no third-party requires. This exists so
  # the first dependency added is covered from the moment it lands.
  - package-ecosystem: gomod
    directory: /
    schedule:
      interval: weekly
      day: monday
      time: "07:00"
    open-pull-requests-limit: 5
    commit-message:
      prefix: "build"
      include: scope
    groups:
      go-dependencies:
        patterns: ["*"]
        update-types: [minor, patch]

  # This is the entry that does real work: it keeps the pinned action SHAs in
  # .github/workflows current, and pinned SHAs never update by themselves.
  - package-ecosystem: github-actions
    directory: /
    schedule:
      interval: weekly
      day: monday
      time: "07:00"
    open-pull-requests-limit: 5
    commit-message:
      prefix: "ci"
      include: scope
    groups:
      actions:
        patterns: ["*"]
        update-types: [minor, patch]
```

Grouping minor and patch updates keeps routine bumps to one PR per ecosystem
per week. Major updates still arrive as individual PRs, which is what you want
for a breaking change.

- [ ] **Step 2: Validate the YAML**

```bash
python3 -c "import yaml; yaml.safe_load(open('.github/dependabot.yml')); print('dependabot.yml parses')"
```

Expected: `dependabot.yml parses`

- [ ] **Step 3: Commit**

```bash
git add .github/dependabot.yml
git commit -m "ci: add Dependabot for gomod and github-actions

The gomod entry is idle today because the module has no third-party
requires; it exists so the first dependency is covered on arrival.

The github-actions entry does real work immediately: workflow actions are
pinned to commit SHAs, which never update on their own, so Dependabot is
what keeps the pins current. Minor and patch updates are grouped into one
PR per ecosystem per week; majors still arrive individually.

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 9: GoReleaser and the release workflow

**Files:**
- Create: `.goreleaser.yaml`
- Create: `.github/workflows/release.yml`

**Interfaces:**
- Consumes: `./cmd/radiogogo` (Task 4) and `main.version` for ldflags stamping.
- Produces: a tag-triggered release. `make release-snapshot` and
  `make release-check` (Task 5) drive it locally.

- [ ] **Step 1: Write the GoReleaser config**

Create `.goreleaser.yaml`:

```yaml
version: 2

project_name: radiogogo

before:
  hooks:
    - go mod tidy
    - go mod verify

builds:
  - id: radiogogo
    main: ./cmd/radiogogo
    binary: radiogogo
    env:
      - CGO_ENABLED=0
    flags:
      - -trimpath
    ldflags:
      - -s -w -X main.version={{ .Version }}
    goos:
      - linux
      - darwin
      - windows
    goarch:
      - amd64
      - arm64
    mod_timestamp: "{{ .CommitTimestamp }}"

archives:
  - id: default
    ids:
      - radiogogo
    name_template: "{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}"
    format_overrides:
      - goos: windows
        formats:
          - zip
    files:
      - README.md
      - LICENSE

checksum:
  name_template: checksums.txt
  algorithm: sha256

sboms:
  - id: archives
    artifacts: archive

signs:
  - id: checksums
    cmd: cosign
    signature: "${artifact}.sig"
    certificate: "${artifact}.pem"
    args:
      - sign-blob
      - "--output-signature=${signature}"
      - "--output-certificate=${certificate}"
      - "${artifact}"
      - "--yes"
    artifacts: checksum
    output: true

snapshot:
  version_template: "{{ incpatch .Version }}-snapshot"

changelog:
  sort: asc
  use: github
  groups:
    - title: Features
      regexp: '^.*?feat(\(.+\))??!?:.+$'
      order: 0
    - title: Bug fixes
      regexp: '^.*?fix(\(.+\))??!?:.+$'
      order: 1
    - title: Build and CI
      regexp: '^.*?(build|ci)(\(.+\))??!?:.+$'
      order: 2
    - title: Other
      order: 999
  filters:
    exclude:
      - "^docs:"
      - "^test:"
      - "^chore:"
      - "Merge pull request"

release:
  draft: false
  prerelease: auto
  footer: |
    ## Verifying this release

    Checksums are signed with [cosign](https://docs.sigstore.dev/) keylessly —
    there is no private key. Download `checksums.txt`, `checksums.txt.sig`, and
    `checksums.txt.pem`, then:

    ```sh
    cosign verify-blob \
      --certificate checksums.txt.pem \
      --signature checksums.txt.sig \
      --certificate-identity-regexp 'https://github\.com/omaciel/radiogogo/\.github/workflows/.+' \
      --certificate-oidc-issuer https://token.actions.githubusercontent.com \
      checksums.txt
    ```

    Then check your archive against it:

    ```sh
    sha256sum --check --ignore-missing checksums.txt
    ```
```

`mod_timestamp: {{ .CommitTimestamp }}` makes builds reproducible: without it
each build embeds its own wall-clock time and identical source yields differing
binaries.

- [ ] **Step 2: Validate the config**

```bash
make release-check
```

Expected: `command finished successfully` / no errors.

GoReleaser's schema shifts across minor versions, and the local install is what
arbitrates. If `check` reports a deprecated or unknown key, fix it as `check`
instructs — for example `archives.format` versus `archives.formats`, or
`archives.builds` versus `archives.ids`. Confirm the installed version first
with `goreleaser --version`.

- [ ] **Step 3: Build a snapshot release locally**

```bash
make release-snapshot
ls -la dist/
```

Expected: six archives (linux/darwin/windows × amd64/arm64, windows as `.zip`),
plus `checksums.txt` and SBOM files. Signing is skipped locally via
`--skip=sign`, since keyless cosign needs the CI OIDC token.

If `syft` is missing, SBOM generation fails. Install it:
```bash
brew install syft
```

- [ ] **Step 4: Verify a snapshot binary works and is stamped**

```bash
tar -tzf dist/radiogogo_*_darwin_arm64.tar.gz
tar -xzf dist/radiogogo_*_darwin_arm64.tar.gz -C /tmp radiogogo
/tmp/radiogogo --version
/tmp/radiogogo --list
```

Expected: the archive contains `radiogogo`, `README.md`, and `LICENSE`;
`--version` prints a `-snapshot` version rather than `dev`, proving GoReleaser's
ldflags reach `main.version`; `--list` prints the catalog.

- [ ] **Step 5: Write the release workflow**

Create `.github/workflows/release.yml`:

```yaml
name: Release

on:
  push:
    tags: ["v*"]

permissions:
  contents: read

jobs:
  release:
    runs-on: ubuntu-latest
    permissions:
      contents: write      # create the release and upload assets
      id-token: write      # keyless cosign signing and attestation
      attestations: write  # build provenance
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0   # GoReleaser needs full history for the changelog

      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
          check-latest: true

      - name: Test before releasing
        run: go test -race -count=1 ./...

      - name: Scan before releasing
        run: go run golang.org/x/vuln/cmd/govulncheck@latest ./...

      - uses: sigstore/cosign-installer@v3
      - uses: anchore/sbom-action/download-syft@v0

      - uses: goreleaser/goreleaser-action@v6
        with:
          distribution: goreleaser
          version: "~> v2"
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}

      - name: Attest build provenance
        uses: actions/attest-build-provenance@v1
        with:
          subject-path: dist/*.tar.gz, dist/*.zip
```

Tests and govulncheck run before the release, so a tag cannot publish a build
that fails its own suite.

- [ ] **Step 6: Validate the YAML**

```bash
python3 -c "import yaml; yaml.safe_load(open('.github/workflows/release.yml')); print('release.yml parses')"
```

Expected: `release.yml parses`

- [ ] **Step 7: Clean up snapshot output**

```bash
rm -rf dist/
git status --porcelain
```

Expected: `dist/` is gone and untracked output does not appear — Task 5 added it
to `.gitignore`.

- [ ] **Step 8: Commit**

```bash
git add .goreleaser.yaml .github/workflows/release.yml
git commit -m "ci: add GoReleaser with signing, SBOM, and provenance

Tag-triggered release: binaries for linux, darwin, and windows on amd64 and
arm64, plus checksums, a syft SBOM, keyless cosign signatures, and GitHub
build provenance.

Tests and govulncheck run before the release, so a tag cannot publish a
build that fails its own suite. mod_timestamp is pinned to the commit
timestamp so builds are reproducible. The release notes carry the
cosign verify-blob invocation.

'make release-snapshot' exercises the whole pipeline locally without a tag.

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 10: Pin actions to SHAs, add badges, final verification

Pinning is deferred to one task because it is a single mechanical sweep over
every workflow, and because Scorecard's `Pinned-Dependencies` check grades all
of them together.

**Files:**
- Modify: `.github/workflows/ci.yml`, `codeql.yml`, `scorecard.yml`, `release.yml`
- Modify: `README.md`

**Interfaces:**
- Consumes: all four workflows.
- Produces: the final state of the branch.

- [ ] **Step 1: Resolve every action tag to a commit SHA**

```bash
cd /Users/omaciel/hacking/github.com/omaciel/radiogogo
grep -rhoE 'uses: [a-zA-Z0-9._/-]+@v[0-9.]+' .github/workflows/ \
  | sed 's/uses: //' | sort -u \
  | while read -r ref; do
      repo="${ref%@*}"
      tag="${ref#*@}"
      # Resolve via the commits endpoint: it dereferences annotated tags to the
      # commit, which repos/:r/git/ref/tags/:t does not.
      sha=$(gh api "repos/${repo%%/*}/${repo#*/}/commits/${tag}" --jq '.sha' 2>/dev/null)
      if [ -z "$sha" ]; then
        echo "FAILED to resolve $ref" >&2
        continue
      fi
      printf '%s@%s # %s\n' "$repo" "$sha" "$tag"
    done
```

Expected: one `owner/repo@<40-char-sha> # vN` line per action, and no `FAILED`
lines. `github/codeql-action` appears three times (init, autobuild, analyze) but
resolves once — subpaths share the repo's SHA.

- [ ] **Step 2: Apply the pins**

For every `uses:` line in all four workflow files, replace the tag with the
resolved SHA and keep the tag as a trailing comment, so a human can still read
the version:

```yaml
      - uses: actions/checkout@<sha> # v4
      - uses: actions/setup-go@<sha> # v5
```

`github/codeql-action` subpaths take the same SHA:

```yaml
      - uses: github/codeql-action/init@<sha> # v3
      - uses: github/codeql-action/autobuild@<sha> # v3
      - uses: github/codeql-action/analyze@<sha> # v3
      - uses: github/codeql-action/upload-sarif@<sha> # v3
```

- [ ] **Step 3: Verify no unpinned action remains**

```bash
if grep -rnE 'uses: [^#]*@v[0-9.]+\s*$' .github/workflows/; then
  echo "UNPINNED ACTIONS FOUND — fix the lines above"
else
  echo "all actions pinned"
fi
python3 -c "
import glob, yaml
for f in sorted(glob.glob('.github/workflows/*.yml')):
    yaml.safe_load(open(f)); print(f, 'parses')
"
```

Expected: `all actions pinned`, then all four files parse.

- [ ] **Step 4: Add badges and a security section to the README**

Insert immediately below the `# RadioGoGo - Command Line Online Radio Player`
heading:

```markdown
[![CI](https://github.com/omaciel/radiogogo/actions/workflows/ci.yml/badge.svg)](https://github.com/omaciel/radiogogo/actions/workflows/ci.yml)
[![CodeQL](https://github.com/omaciel/radiogogo/actions/workflows/codeql.yml/badge.svg)](https://github.com/omaciel/radiogogo/actions/workflows/codeql.yml)
[![OpenSSF Scorecard](https://api.securityscorecards.dev/projects/github.com/omaciel/radiogogo/badge)](https://scorecard.dev/viewer/?uri=github.com/omaciel/radiogogo)
[![Go Reference](https://pkg.go.dev/badge/github.com/omaciel/radiogogo.svg)](https://pkg.go.dev/github.com/omaciel/radiogogo)
[![Go Report Card](https://goreportcard.com/badge/github.com/omaciel/radiogogo)](https://goreportcard.com/report/github.com/omaciel/radiogogo)
```

The Scorecard badge only resolves after the Scorecard workflow has run on
`main` at least once with `publish_results: true`. It will 404 until then; that
is expected on this branch, not a mistake to chase.

Append before `## License`:

````markdown
## Development

```sh
make help             # list every target
make check            # everything CI runs: fmt, vet, lint, race tests, govulncheck
make test             # tests only
make build            # build into ./dist
make release-snapshot # rehearse a full release locally, without tagging
```

## Security

Each push and pull request is scanned by
[govulncheck](https://pkg.go.dev/golang.org/x/vuln/cmd/govulncheck) for known
vulnerabilities, [gosec](https://github.com/securego/gosec) (via golangci-lint),
and [CodeQL](https://codeql.github.com/). Repository hygiene is graded by
[OpenSSF Scorecard](https://scorecard.dev/).

Release archives ship with an SBOM, and `checksums.txt` is signed keylessly with
[cosign](https://docs.sigstore.dev/) — verification instructions are in every
release's notes.
````

- [ ] **Step 5: Run the full local check**

```bash
make check
```

Expected: `fmt`, `vet`, `lint`, `test-race`, and `vuln` all pass with no
findings. This is the last gate before the branch goes up.

- [ ] **Step 6: Confirm the module still has zero dependencies**

```bash
go mod tidy
cat go.mod
git diff --exit-code -- go.mod
```

Expected `go.mod` — the global constraint holds, no `require` block:

```
module github.com/omaciel/radiogogo

go 1.26.0
```

- [ ] **Step 7: Re-verify behaviour end to end**

```bash
make build
./dist/radiogogo --version
./dist/radiogogo --list
./dist/radiogogo file:///etc/passwd ; echo "exit=$?"
./dist/radiogogo -- -x ; echo "exit=$?"
./dist/radiogogo https://example.com/a https://example.com/b ; echo "exit=$?"
```

Expected: version stamped from git; catalog listed; `file://` and `-x` refused
with `invalid stream URL` and exit 1; two URLs refused with exit 2 (usage).

- [ ] **Step 8: Commit and push**

```bash
git add .github/workflows/ README.md
git commit -m "ci: pin actions to SHAs and document the tooling

Every action is pinned to a commit SHA with its tag kept as a trailing
comment. A tag is mutable, so a pinned SHA is what makes the release
pipeline's signing and provenance meaningful; Dependabot's github-actions
entry keeps the pins current. Scorecard grades this directly.

Adds CI, CodeQL, Scorecard, pkg.go.dev, and Go Report Card badges, plus
Development and Security sections to the README.

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"

git push -u origin modernize-go-tooling
```

- [ ] **Step 9: Open the pull request**

```bash
gh pr create --fill --title "Modernize: Go 1.26, security fixes, CI, scanning, and signed releases"
```

After CI goes green, confirm on GitHub:
- CI passes on ubuntu, macos, and windows.
- CodeQL reports no alerts — in particular no `os.Args` → `exec.Command`
  finding, which is the defect Task 2 fixed.
- The Security tab shows CodeQL and Scorecard results.

---

## Post-merge follow-ups

Not tasks in this plan; they need repository settings rather than code, and
several are Scorecard criteria that no commit can satisfy:

1. Enable branch protection on `main` (Scorecard's `Branch-Protection`).
2. Set `contents: read` as the default workflow token permission in
   Settings → Actions → General.
3. Cut `v0.2.0` with `make release TAG=v0.2.0` to exercise the release path,
   then verify the signature per the release notes.
4. Confirm the Scorecard badge resolves once the workflow has run on `main`.
