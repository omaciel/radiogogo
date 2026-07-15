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
		return fmt.Errorf("%w: %q: %w", ErrInvalidURL, raw, err)
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
	// gosec flags every exec call whose arguments are not literals. Here name
	// is a constant from commandsFor, and Play rejects any URL that is not
	// http/https or that starts with '-' before a Runner ever sees it.
	//nolint:gosec // G204: name is constant; the URL is checked by Validate
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
		// A cancelled context is a normal stop, not a player failure: don't
		// try the next player and don't fold it in with real failures.
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		errs = append(errs, fmt.Errorf("%s: %w", c.name, err))
	}
	return fmt.Errorf("every player failed: %w", errors.Join(errs...))
}
