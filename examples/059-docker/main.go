// Command docker is a tiny service built to run in a minimal container image:
// by default it prints one JSON object describing the environment it finds
// (Go version, CPU limits, time zone data, CA certificates) and exits.
//
// Slides: slides/059-docker.pdf
//
// Run:
//
//	go run ./examples/059-docker
//	go run ./examples/059-docker -serve
//	go run ./examples/059-docker -https https://go.dev
//	docker build -f examples/059-docker/Dockerfile -t gosvc:distroless .
//	docker run --rm --cpus=2 gosvc:distroless
package main

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"runtime/debug"
	"syscall"
	"time"
)

// info is what the default mode prints and what GET /info returns.
type info struct {
	GoVersion  string `json:"go_version"`
	GOMAXPROCS int    `json:"gomaxprocs"`
	NumCPU     int    `json:"num_cpu"`
	// GOMEMLIMIT is "unset" unless the GOMEMLIMIT environment variable (or
	// debug.SetMemoryLimit) sets one. Unlike GOMAXPROCS, Go does not derive
	// it from the container's memory limit.
	GOMEMLIMIT string `json:"gomemlimit"`
	// TimeZone reports whether time.LoadLocation found time zone data:
	// distroless ships /usr/share/zoneinfo, scratch has nothing unless the
	// binary embeds it with -tags timetzdata (or import _ "time/tzdata").
	TimeZone string `json:"tz_europe_berlin"`
	// SystemCACerts reports whether the system certificate pool has any
	// roots. Without them, every outbound HTTPS call fails verification.
	SystemCACerts bool `json:"system_ca_certs"`
}

func collectInfo() info {
	inf := info{
		GoVersion:  runtime.Version(),
		GOMAXPROCS: runtime.GOMAXPROCS(0),
		NumCPU:     runtime.NumCPU(),
		GOMEMLIMIT: "unset",
	}
	// SetMemoryLimit with a negative value only reads the current limit.
	if limit := debug.SetMemoryLimit(-1); limit != math.MaxInt64 {
		inf.GOMEMLIMIT = fmt.Sprintf("%d MiB", limit>>20)
	}

	if loc, err := time.LoadLocation("Europe/Berlin"); err != nil {
		inf.TimeZone = "error: " + err.Error()
	} else {
		inf.TimeZone = "ok: " + loc.String()
	}

	// CertPool has no exported length, and Subjects is deprecated for system
	// pools, so compare with an empty pool instead.
	if pool, err := x509.SystemCertPool(); err == nil && !pool.Equal(x509.NewCertPool()) {
		inf.SystemCACerts = true
	}
	return inf
}

func main() {
	serveFlag := flag.Bool("serve", false, "serve GET /healthz and GET /info on "+addr+" until SIGTERM or Ctrl+C")
	httpsURL := flag.String("https", "", "make one outbound GET to this https:// URL to prove CA certificates work")
	flag.Parse()

	switch {
	case *serveFlag:
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		if err := serve(ctx); err != nil {
			log.Fatal(err)
		}
	case *httpsURL != "":
		if err := fetch(*httpsURL); err != nil {
			fmt.Println("GET failed:", err)
			os.Exit(1)
		}
	default:
		// One JSON object on one line, so CI and `docker run` logs stay
		// machine-readable.
		if err := json.NewEncoder(os.Stdout).Encode(collectInfo()); err != nil {
			log.Fatal(err)
		}
	}
}

// addr listens on every interface on purpose: inside a container,
// "localhost" is the container's own loopback, which `docker run -p` can't
// reach. Publish it to the host's loopback instead:
//
//	docker run --rm -p 127.0.0.1:8080:8080 gosvc:distroless -serve
const addr = ":8080"

func newMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		// There is no shell or curl in the image for a Docker HEALTHCHECK;
		// an orchestrator probes this endpoint instead.
		fmt.Fprintln(w, "ok")
	})
	mux.HandleFunc("GET /info", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(collectInfo())
	})
	return mux
}

// serve runs until ctx is cancelled, then gives in-flight requests up to five
// seconds to finish. `docker stop` sends SIGTERM to PID 1, which is this
// binary: there is no shell in between to swallow the signal.
func serve(ctx context.Context) error {
	srv := &http.Server{Addr: addr, Handler: newMux(), ReadHeaderTimeout: 5 * time.Second}
	errc := make(chan error, 1)
	go func() { errc <- srv.ListenAndServe() }()
	log.Printf("listening on %s", addr)

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	log.Print("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return err
	}
	if err := <-errc; !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// fetch makes one GET request. On a bare scratch image without a CA bundle
// it fails with "x509: certificate signed by unknown authority".
func fetch(url string) error {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	n, err := io.Copy(io.Discard, resp.Body)
	if err != nil {
		return err
	}
	fmt.Printf("GET %s: %s (%d bytes); TLS verification succeeded\n", url, resp.Status, n)
	return nil
}
