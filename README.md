# RadioGoGo - Command Line Online Radio Player

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
go build ./src
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
go run ./src http://stream.radioparadise.com/mp3-192
```

### Example

Play **Radio Paradise** using `radiogogo`:

```sh
go run ./src http://stream.radioparadise.com/mp3-192
```

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

## License

MIT License

## Contributing

Pull requests and suggestions are welcome!
