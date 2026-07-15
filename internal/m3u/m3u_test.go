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

func TestResolveSkipsNonHTTPEntries(t *testing.T) {
	// A playlist is network content. Entries that are not http/https streams
	// must never be returned, and "httpfoo" must not pass for "http".
	srv := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		io.WriteString(w, "file:///etc/passwd\nftp://example.com/x\nhttpfoo\nhttps://stream.example.com/real\n")
	})

	got, err := NewWithClient(srv.Client()).Resolve(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("Resolve() = %v, want nil", err)
	}
	if want := "https://stream.example.com/real"; got != want {
		t.Errorf("Resolve() = %q, want %q; non-http entries must not be returned", got, want)
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
