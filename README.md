# RadioGoGo - Command Line Online Radio Player

[![CI](https://github.com/omaciel/radiogogo/actions/workflows/ci.yml/badge.svg)](https://github.com/omaciel/radiogogo/actions/workflows/ci.yml)
[![CodeQL](https://github.com/omaciel/radiogogo/actions/workflows/codeql.yml/badge.svg)](https://github.com/omaciel/radiogogo/actions/workflows/codeql.yml)
[![OpenSSF Scorecard](https://api.securityscorecards.dev/projects/github.com/omaciel/radiogogo/badge)](https://scorecard.dev/viewer/?uri=github.com/omaciel/radiogogo)
[![Go Reference](https://pkg.go.dev/badge/github.com/omaciel/radiogogo.svg)](https://pkg.go.dev/github.com/omaciel/radiogogo)
[![Go Report Card](https://goreportcard.com/badge/github.com/omaciel/radiogogo)](https://goreportcard.com/report/github.com/omaciel/radiogogo)

RadioGoGo is a simple command-line tool that allows users to play online radio streams directly from their terminal. It supports a wide range of stream formats and provides keyboard shortcuts to control playback.

## Features

- Play online radio streams from the command line
- Supports **MP3**, **AAC**, and other common streaming formats
- Uses `mpg123` or `ffplay` for playback
- Terminal controls for volume, skipping tracks, and quitting

## Installation

### Linux (Ubuntu/Debian)

```sh
sudo apt update
sudo apt install mpg123 ffmpeg
```

### Fedora Linux

```sh
sudo dnf install mpg123 ffmpeg
```

### macOS (via Homebrew)

```sh
brew install mpg123 ffmpeg
```

## Install RadioGoGo

### Build it from source code

You can build `RadioGoGo` locally:

```sh
go build -o radiogogo ./cmd/radiogogo
```

Or install it directly:

```sh
go install github.com/omaciel/radiogogo/cmd/radiogogo@latest
```

## Usage

To play an online radio stream, run:

```sh
radiogogo http://stream.radioparadise.com/mp3-192
```

If you don't provide a radio stream, a random one will be chosen for you:

```sh
radiogogo
No URL provided. A random radio station will be chosen.
Selected station: Radio Swiss Classic (https://stream.srg-ssr.ch/m/rsc_de/mp3_128)
```

### Running from Source Code

If you want to run `radiogogo` from the source code, navigate to the project root directory and execute:

```sh
go run ./cmd/radiogogo http://stream.radioparadise.com/mp3-192
```

### Example

Play **Radio Paradise** using `radiogogo`:

```sh
go run ./cmd/radiogogo http://stream.radioparadise.com/mp3-192
```

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

## Terminal Playback Controls

While `radiogogo` is running, you can control playback using the following keys:

| Key  | Action |
|------|--------|
| `h`  | Show help menu with available commands |
| `+`  | Increase volume |
| `-`  | Decrease volume |
| `>`  | Skip track |
| `q`  | Quit playback |

## Notes

- **mpg123** provides additional keyboard controls, accessible by pressing `h` during playback.
- **ffplay** is used as a fallback if `mpg123` fails to play a stream.

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

## License

MIT License

## Contributing

Pull requests and suggestions are welcome!
