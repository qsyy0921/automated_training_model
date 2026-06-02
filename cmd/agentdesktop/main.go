package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

func main() {
	addr := flag.String("addr", "http://127.0.0.1:7870", "labelserver base URL")
	token := flag.String("token", firstEnv("ATM_GATEWAY_TOKEN", "GATEWAY_AUTH_TOKEN"), "Gateway bearer token")
	flag.Parse()

	url := strings.TrimRight(*addr, "/") + "/api/desktop/status"
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if strings.TrimSpace(*token) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(*token))
		req.Header.Set("X-Gateway-Token", strings.TrimSpace(*token))
	}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if resp.StatusCode >= 400 {
		fmt.Fprintf(os.Stderr, "%s: %s\n", resp.Status, string(raw))
		os.Exit(1)
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		fmt.Println(string(raw))
		return
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(value)
}

func firstEnv(names ...string) string {
	for _, name := range names {
		if value := strings.TrimSpace(os.Getenv(name)); value != "" {
			return value
		}
	}
	return ""
}
