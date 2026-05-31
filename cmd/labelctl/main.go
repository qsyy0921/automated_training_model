package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
)

func main() {
	addr := flag.String("addr", "http://127.0.0.1:7870", "labelserver base URL")
	flag.Parse()
	args := flag.Args()
	if len(args) == 0 {
		usage()
		os.Exit(2)
	}
	switch args[0] {
	case "health":
		get(*addr + "/healthz")
	case "videos":
		get(*addr + "/api/videos")
	case "providers":
		get(*addr + "/api/providers")
	case "secrets":
		get(*addr + "/api/secrets")
	case "video":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: labelctl video <scene>")
			os.Exit(2)
		}
		get(*addr + "/api/video/" + args[1] + "/meta")
	default:
		usage()
		os.Exit(2)
	}
}

func get(url string) {
	resp, err := http.Get(url)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer resp.Body.Close()
	var value any
	if err := json.NewDecoder(resp.Body).Decode(&value); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(value)
}

func usage() {
	fmt.Println(`labelctl commands:
  health
  videos
  providers
  secrets
  video <scene>

options:
  -addr http://127.0.0.1:7870`)
}
