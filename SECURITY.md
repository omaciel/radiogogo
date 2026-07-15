# Security Policy

RadioGoGo is a small hobby CLI maintained by one person in their spare
time. This policy is honest about what that means: reports are handled
best-effort, with no SLA.

## Scope

RadioGoGo takes a URL, optionally fetches an M3U playlist from it over
HTTP(S), and hands a stream URL to `mpg123` or `ffplay`. That's the whole
attack surface:

- Only `http` and `https` URLs are accepted. Anything else (`file://`,
  `javascript:`, no scheme at all, ...) is rejected before it reaches a
  player.
- A URL beginning with `-` is rejected, since `mpg123`/`ffplay` would
  otherwise read it as a command-line flag instead of a stream.
- Playlist parsing only ever extracts `http://`/`https://` lines; it does
  not execute or evaluate anything in the playlist body.

If you find a way around these checks — a URL that slips through
validation and reaches a player in an unsafe way, or an M3U response that
does something other than yield a stream URL — that's a real finding.

## Supported Versions

Only the latest release is supported. If you're running something older,
please upgrade before reporting; the issue may already be fixed.

## Reporting a Vulnerability

Please use GitHub's private vulnerability reporting instead of a public
issue: go to the repository's **Security** tab and click **Report a
vulnerability**. This keeps details out of public view until there's a
fix.

Don't open a public issue or pull request for a vulnerability — use
private reporting so it isn't disclosed before there's a fix.

There's no guaranteed response time. Reports will get looked at and
acknowledged as soon as possible, but this is not a funded or staffed
security team — it's one maintainer.

## Verifying Releases

Release checksums are signed keylessly with
[cosign](https://docs.sigstore.dev/). See the "Verifying this release"
section of each release's notes for the exact `cosign verify-blob`
command and expected identity.
