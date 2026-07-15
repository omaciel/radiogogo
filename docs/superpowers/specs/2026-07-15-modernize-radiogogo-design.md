# Modernizing RadioGoGo

**Date:** 2026-07-15
**Status:** Approved

## Context

RadioGoGo is a ~90-line command-line radio player. It has no third-party
dependencies, no tests, and no CI. The code lives in `src/`, pins Go 1.23.5, and
shells out to `mpg123` with `ffplay` as a fallback.

The request was to update Go and dependencies, add security scanning and
Dependabot, and provide a Makefile for development and releases. Two parts of
that need restating against what is actually in the repository:

- **There are no dependencies to update.** `go.mod` declares the module, the Go
  version, and nothing else. A Dependabot `gomod` entry is still worth adding so
  the first dependency is covered on arrival, but the entry that does work today
  is `github-actions`.
- **The Go standard library is therefore the entire dependency surface.** That
  raises `govulncheck` from a routine addition to the highest-value scanner in
  the set: it is the only tool here that reports a CVE in the `net/http` code
  this program calls.

Reading the code surfaced defects beyond the requested scope. They are included
because security scanning that reports known-unfixed findings on day one trains
everyone to ignore the dashboard.

## Goals

- Bump Go 1.23.5 to 1.26.0.
- Fix the security defects in playlist fetching and player invocation.
- Restructure to an idiomatic layout so `go install ...@latest` works.
- Add tests, with priority on the security-critical paths.
- Add security scanning: govulncheck, golangci-lint, CodeQL, OpenSSF Scorecard.
- Add Dependabot for `gomod` and `github-actions`.
- Add a Makefile for development and release.
- Ship tag-triggered releases with SBOM, signing, and provenance.

## Non-Goals

- A configuration file for stations. The hardcoded map is adequate.
- Any refactoring beyond the layout change described here.
- Testing actual audio playback. Tests assert on the command that would be
  executed, never on sound.

## Architecture

```
cmd/radiogogo/main.go     flag parsing, wiring, exit codes — no logic
internal/station/         catalog, lookup, random selection
internal/m3u/             playlist fetch: timeout, size cap, status check
internal/player/          URL validation, player invocation + fallback
```

The split is driven by testability of the two dangerous paths, not by tidiness.
The current code cannot be tested at all: `playStream` unconditionally spawns a
subprocess and `fetchM3U` unconditionally hits the network.

- `player` accepts an injectable command runner, so tests assert on the argv that
  would be executed without spawning `mpg123`.
- `m3u` accepts an `*http.Client`, so `httptest` drives it in-process.

Each package is independently understandable: `station` knows nothing about HTTP,
`m3u` knows nothing about subprocesses, and `player` knows nothing about
playlists. `main` wires them together and owns all exit codes.

### Data flow

```
args → main → station.Random() or station.Lookup(name) or literal URL
            → player.Validate(url)          ← rejects before any I/O
            → m3u.Resolve(url) if .m3u path ← returns first stream URL
            → player.Validate(streamURL)    ← re-validated after indirection
            → player.Play(url)              → mpg123, fallback ffplay
```

Validation runs twice by design. The URL returned from a playlist is attacker-
influenced content from the network and gets the same treatment as user input.

## Security defects being fixed

1. **No HTTP timeout.** `fetchM3U` calls bare `http.Get`. A slow or hostile
   server hangs the process indefinitely. Fix: `http.Client{Timeout: 10s}` plus a
   context.

2. **Unbounded body read.** The response is scanned with no size limit. Fix:
   `io.LimitReader` capped at 1 MB. A playlist is a few hundred bytes.

3. **Ignored status code.** `fetchM3U` never checks `resp.StatusCode`. A 404 HTML
   error page is scanned for lines beginning with `http`, and the first link
   found in that error page is played as audio. Fix: require 2xx before parsing.

4. **Argument injection.** A URL is passed straight to `exec.Command`. There is
   no shell, so this is not shell injection, but a URL beginning with `-` is
   parsed as a *flag* by `mpg123`/`ffplay`. There is also no scheme check, so
   `file:///etc/passwd` reaches the player. Fix, defense in depth:
   - parse the URL and require scheme `http` or `https`;
   - reject any argument with a leading `-`;
   - pass `--` before the URL so the player cannot reinterpret it.
   Any one of these suffices; they fail differently, so all three are kept.

5. **Fragile playlist detection.** `strings.HasSuffix(url, ".m3u")` misses
   `stream.m3u?token=abc`. Fix: parse and test the URL *path* suffix.

Housekeeping in the same pass: wrap errors with `%w`; replace
`rand.New(rand.NewSource(time.Now().UnixNano()))` with `math/rand/v2`; change the
Radio Paradise URL from `http://` to `https://`.

## CLI surface

Existing invocations are unchanged:

```
radiogogo                      random station
radiogogo <url>                play url
```

Added:

```
radiogogo --list               print stations
radiogogo --station WUNC       play a named station
radiogogo --version            version, stamped via ldflags
```

`--version` is required rather than optional: GoReleaser stamps the binary, and
without it a released artifact cannot be traced to a commit.

Rejected inputs (previously accepted):

```
radiogogo -x                   leading dash
radiogogo file:///etc/passwd   scheme not allowed
```

## Error handling

Errors wrap with `%w` and surface at `main`, which is the only component that
calls `os.Exit`. Library packages return errors and never exit or print. Exit
codes: `0` success, `1` runtime failure, `2` usage error.

The `mpg123` → `ffplay` fallback keeps its current behavior: if `mpg123` exits
non-zero, try `ffplay`. If both fail, report both errors rather than only the
last, so a missing binary is distinguishable from a bad stream.

## Testing

Table-driven, weighted toward validation because that is where the security is.

- **player**: scheme accept/reject, leading-dash rejection, `--` separator
  presence, argv construction, fallback-to-ffplay via fake runner, both-fail
  error reporting.
- **m3u**: valid playlist, comments-only, 404, oversized body, timeout,
  query-string suffix, no-valid-url.
- **station**: deterministic selection under an injected seed, lookup hit/miss.

CI runs tests on ubuntu, macos, and windows. Because tests use the fake runner
and `httptest`, no CI runner needs `mpg123` or network access.

## Tooling

**Makefile.** Development: `build test test-race cover lint fmt vet vuln tidy
clean run install`. Release: `release-snapshot release-check release version`.
`release-snapshot` exercises the full release pipeline locally without tagging.

**Workflows** (`.github/workflows/`):

| File | Runs |
|---|---|
| `ci.yml` | test matrix, golangci-lint, govulncheck |
| `codeql.yml` | semantic analysis, PR + weekly |
| `scorecard.yml` | repo hygiene, badge |
| `release.yml` | goreleaser, cosign, SBOM, attestation |

Actions are pinned to commit SHA with minimal `permissions:` blocks. Scorecard
grades both, so the pinning is part of the deliverable rather than a style
preference.

**Dependabot** (`.github/dependabot.yml`): `gomod` and `github-actions`, with
grouped minor/patch updates to limit PR volume.

**Release** is tag-triggered: cross-compiled binaries for linux/darwin/windows on
amd64+arm64, checksums, syft SBOM, cosign keyless signing, and GitHub build
provenance attestation.

## Decisions and risks

- **`go 1.26.0` floor.** With `GOTOOLCHAIN=auto` older installs fetch 1.26
  automatically, so the practical cost is low. This would be the wrong call for a
  library, where the directive is a hard floor on consumers. Accepted for a
  distributed CLI.
- **10s HTTP timeout.** Covers playlist fetch only, never playback — the player
  subprocess is unaffected and streams indefinitely as before. A radio server too
  slow to return a few hundred bytes of playlist in 10s is not worth waiting on.
- **Rejecting `file://` is a breaking change.** Anyone playing local files via
  RadioGoGo loses that. Judged acceptable: it is an online radio player, and the
  local-file path is exactly the injection vector.
- **`src/` → `cmd/` breaks `go build ./src`.** README is updated in the same
  change.
