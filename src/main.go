package main

import (
	"bufio"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

// fetchM3U retrieves the first valid URL from an M3U file
func fetchM3U(m3uURL string) (string, error) {
	resp, err := http.Get(m3uURL)
	if err != nil {
		return "", fmt.Errorf("failed to fetch M3U file: %v", err)
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "http") { // First valid stream URL
			return line, nil
		}
	}

	return "", fmt.Errorf("no valid stream URL found in M3U file")
}

// playStream attempts to play a given stream URL
func playStream(url string) error {
	cmd := exec.Command("mpg123", url) // Use mpg123 first
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Println("mpg123 failed, trying ffplay...")
		cmd = exec.Command("ffplay", "-nodisp", "-autoexit", url) // Use ffplay as fallback
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin // Allow terminal control keys

		return cmd.Run()
	}
	return nil
}

// getRandomRadio selects a random radio station from the map
func getRandomRadio() (string, string) {
	r := rand.New(rand.NewSource(time.Now().UnixNano())) // Create a new random generator
	keys := make([]string, 0, len(radios))
	for k := range radios {
		keys = append(keys, k)
	}
	randomKey := keys[r.Intn(len(keys))]
	return randomKey, radios[randomKey]
}

func main() {
	var url string

	if len(os.Args) < 2 {
		fmt.Println("No URL provided. A random radio station will be chosen.")
		name, randomURL := getRandomRadio()
		fmt.Printf("Selected station: %s (%s)\n", name, randomURL)
		url = randomURL
	} else {
		url = os.Args[1]
	}

	// If it's an M3U file, fetch the actual stream URL
	if strings.HasSuffix(url, ".m3u") {
		fmt.Println("Fetching stream from M3U file...")
		streamURL, err := fetchM3U(url)
		if err != nil {
			fmt.Println("Error:", err)
			os.Exit(1)
		}
		url = streamURL
	}

	fmt.Println("Playing:", url)
	if err := playStream(url); err != nil {
		fmt.Println("Error playing stream:", err)
		os.Exit(1)
	}
}
