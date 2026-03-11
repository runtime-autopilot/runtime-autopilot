package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/runtime-autopilot/runtime-autopilot/internal/profile"
	"github.com/runtime-autopilot/runtime-autopilot/pkg/probe"
)

func main() {
	var (
		pretty = flag.Bool("pretty", false, "pretty-print JSON output")
		once   = flag.Bool("once", false, "detect once and exit (default behaviour outside --serve)")
		serve  = flag.String("serve", "", "start HTTP server on given address (e.g. :9000)")
	)

	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: runtime-autopilot [flags]")
		flag.PrintDefaults()
	}

	if err := flag.CommandLine.Parse(os.Args[1:]); err != nil {
		os.Exit(1)
	}

	if *serve != "" && !*once {
		runServer(*serve, *pretty)
		return
	}

	p := probe.Detect()
	if err := writeProfile(os.Stdout, p, *pretty); err != nil {
		log.Printf("encoding profile: %v", err)
		os.Exit(1)
	}
}

func writeProfile(w io.Writer, p profile.RuntimeProfile, pretty bool) error {
	enc := json.NewEncoder(w)
	if pretty {
		enc.SetIndent("", "  ")
	}
	return enc.Encode(p)
}

func runServer(addr string, pretty bool) {
	mux := http.NewServeMux()
	mux.HandleFunc("/profile", func(w http.ResponseWriter, _ *http.Request) {
		p := probe.Detect()
		w.Header().Set("Content-Type", "application/json")
		enc := json.NewEncoder(w)
		
		if pretty {
			enc.SetIndent("", "  ")
		}
		if err := enc.Encode(p); err != nil {
			log.Printf("encoding profile for HTTP response: %v", err)
		}
	})

	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	log.Printf("runtime-autopilot listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
