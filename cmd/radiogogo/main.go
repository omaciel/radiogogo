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
	"runtime/debug"
	"syscall"
	"text/tabwriter"

	"github.com/omaciel/radiogogo/internal/m3u"
	"github.com/omaciel/radiogogo/internal/player"
	"github.com/omaciel/radiogogo/internal/station"
)

// version is stamped at build time with -ldflags -X main.version=...
var version = "dev"

// resolveVersion picks the version string to display. An explicit ldflag
// version (anything other than the "dev" default) always wins. Otherwise it
// falls back to the module version recorded in the build info, unless that is
// empty or Go's "(devel)" placeholder for a non-release build. This is what
// lets `go install ...@latest`, which sets no ldflags, still report the tag.
func resolveVersion(ldflagVersion, buildInfoVersion string) string {
	if ldflagVersion != "dev" {
		return ldflagVersion
	}
	if buildInfoVersion != "" && buildInfoVersion != "(devel)" {
		return buildInfoVersion
	}
	return ldflagVersion
}

// readBuildInfoVersion returns the main module's version as recorded by the Go
// toolchain (e.g. set via `go install pkg@version`), or "" if build info is
// unavailable.
func readBuildInfoVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	return info.Main.Version
}

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
  --help           print this message and exit

Only http and https URLs are accepted.
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run is main's testable body: it takes its arguments and writers rather than
// reading globals, and returns an exit code rather than calling os.Exit.
func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("radiogogo", flag.ContinueOnError)
	// flag.FlagSet.Parse calls Usage itself on any parse error, including
	// -h/--help, before we can distinguish the two below. Silence that
	// automatic call and print the usage text ourselves so -h lands on
	// stdout while a real parse error still lands on stderr.
	fs.SetOutput(io.Discard)
	fs.Usage = func() {}

	showVersion := fs.Bool("version", false, "print the version and exit")
	list := fs.Bool("list", false, "list the built-in stations")
	stationName := fs.String("station", "", "play a named station")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fmt.Fprint(stdout, usage)
			return exitOK
		}
		fmt.Fprint(stderr, usage)
		return exitUsage
	}

	// --version always works first, unconditionally: asking a program its
	// version should never be blocked by other flag conflicts.
	if *showVersion {
		fmt.Fprintf(stdout, "radiogogo %s\n", resolveVersion(version, readBuildInfoVersion()))
		return exitOK
	}

	stationSet := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "station" {
			stationSet = true
		}
	})
	if stationSet && *stationName == "" {
		fmt.Fprintln(stderr, "radiogogo: --station requires a non-empty name")
		return exitUsage
	}

	if *stationName != "" && fs.NArg() > 0 {
		fmt.Fprintln(stderr, "radiogogo: pass either --station or a URL, not both")
		return exitUsage
	}
	if *list && (*stationName != "" || fs.NArg() > 0) {
		fmt.Fprintln(stderr, "radiogogo: --list cannot be combined with --station or a URL")
		return exitUsage
	}
	if fs.NArg() > 1 {
		fmt.Fprintf(stderr, "radiogogo: expected at most one URL, got %d\n", fs.NArg())
		return exitUsage
	}

	if *list {
		printStations(stdout)
		return exitOK
	}

	target, err := selectTarget(*stationName, fs.Args(), stdout)
	if err != nil {
		fmt.Fprintf(stderr, "radiogogo: %v\n", err)
		return exitUsage
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := play(ctx, target, stdout, m3u.New(), player.New()); err != nil {
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
func play(ctx context.Context, target string, stdout io.Writer, resolver *m3u.Resolver, p *player.Player) error {
	if err := player.Validate(target); err != nil {
		return err
	}

	if m3u.IsPlaylist(target) {
		fmt.Fprintln(stdout, "Fetching stream from M3U file...")
		streamURL, err := resolver.Resolve(ctx, target)
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
	return p.Play(ctx, target)
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
