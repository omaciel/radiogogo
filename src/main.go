package main

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"time"
)

// isValidStream checks if the URL is reachable
func isValidStream(url string) bool {
	client := http.Client{
		Timeout: 5 * time.Second,
	}
	resp, err := client.Head(url)
	if err != nil {
		fmt.Println("Error checking URL:", err)
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// playStream tries mpg123, then falls back to ffplay
func playStream(url string) error {
	// Try mpg123 first
	cmd := exec.Command("mpg123", url)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Println("mpg123 failed, trying ffplay...")
		// If mpg123 fails, try ffplay
		cmd = exec.Command("ffplay", "-nodisp", "-autoexit", url)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}
	return nil
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: radiogogo <stream_url>")
		os.Exit(1)
	}

	url := os.Args[1]

	if !isValidStream(url) {
		fmt.Println("Invalid or unavailable stream URL.")
		os.Exit(1)
	}

	fmt.Println("Playing:", url)
	if err := playStream(url); err != nil {
		fmt.Println("Error playing stream:", err)
		os.Exit(1)
	}
}
