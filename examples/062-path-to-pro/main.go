// Command path-to-pro puts the series' advice into one small, production-
// shaped service: path versioning with Go 1.22 patterns and a 410 for the
// retired version, a consumer-side TextGenerator interface with swappable
// providers, security middleware, server hardening, and concurrency only
// where it pays.
//
// Slides: slides/062-path-to-pro.pdf
//
// Run:
//
//	go run ./examples/062-path-to-pro
//	go run ./examples/062-path-to-pro -provider reverse
//	go run ./examples/062-path-to-pro -serve
package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

const report = "Revenue grew 12 percent while support tickets fell by a third"

func main() {
	provider := flag.String("provider", "echo", "AI provider: echo or reverse")
	serveFlag := flag.Bool("serve", false, "listen on localhost:8080 until Ctrl+C")
	flag.Parse()

	gen, err := NewGenerator(*provider, 0)
	if err != nil {
		log.Fatal(err)
	}
	cfg := Config{Provider: *provider, Gen: gen, RateLimit: 100, Burst: 100}

	if *serveFlag {
		if err := serve(cfg); err != nil {
			log.Fatal(err)
		}
		return
	}

	section("1. Path versioning and a retired version (slide p06)")
	versioning(cfg)

	section("2. Swapping providers is a configuration change (slide p03)")
	providerSwap()

	section("3. Security middleware (slide p07)")
	middleware(cfg)

	section("4. Server hardening (slide p07)")
	hardening(cfg)

	section("5. Sequential vs concurrent, measured (slides p05, p10)")
	concurrency()
}

// response is the part of an HTTP response the demos print.
type response struct {
	status int
	header http.Header
	body   string
}

func do(client *http.Client, method, url, body string, header map[string]string) response {
	req, err := http.NewRequestWithContext(context.Background(), method, url, strings.NewReader(body))
	if err != nil {
		return response{body: err.Error()}
	}
	for k, v := range header {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	if err != nil {
		return response{body: err.Error()}
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return response{status: resp.StatusCode, header: resp.Header, body: strings.TrimSpace(string(b))}
}

func versioning(cfg Config) {
	srv := httptest.NewServer(NewHandler(cfg))
	defer srv.Close()
	body := `{"report": "` + report + `"}`

	for _, path := range []string{"/v1/summarize", "/v2/summarize", "/v0/summarize"} {
		r := do(srv.Client(), http.MethodPost, srv.URL+path, body, nil)
		fmt.Printf("POST %s -> %d %s\n", path, r.status, r.body)
		if r.status == http.StatusGone {
			for _, h := range []string{"Deprecation", "Sunset", "Link"} {
				fmt.Printf("  %s: %s\n", h, r.header.Get(h))
			}
		}
	}
	r := do(srv.Client(), http.MethodGet, srv.URL+"/v1/summarize", "", nil)
	fmt.Printf("GET /v1/summarize -> %d (the pattern says POST)\n", r.status)
}

func providerSwap() {
	for _, name := range []string{"echo", "reverse"} {
		gen, err := NewGenerator(name, 0)
		if err != nil {
			fmt.Println(err)
			continue
		}
		summary, err := SummarizeReport(context.Background(), gen, report)
		fmt.Printf("-provider %-7s -> %q, err = %v\n", name, summary, err)
	}
	_, err := NewGenerator("openai", 0)
	fmt.Println("-provider openai  ->", err)
}

func middleware(cfg Config) {
	srv := httptest.NewServer(NewHandler(cfg))
	defer srv.Close()
	body := `{"report": "` + report + `"}`

	r := do(srv.Client(), http.MethodPost, srv.URL+"/v1/summarize", body, nil)
	for _, h := range []string{"Content-Security-Policy", "X-Frame-Options", "X-Content-Type-Options", "Referrer-Policy"} {
		fmt.Printf("%s: %s\n", h, r.header.Get(h))
	}

	// What a browser sends when a page on another site submits a form here.
	r = do(srv.Client(), http.MethodPost, srv.URL+"/v1/summarize", body, map[string]string{"Sec-Fetch-Site": "cross-site"})
	fmt.Printf("cross-site POST -> %d %s\n", r.status, r.body)

	// A separate server with a tiny bucket: 3 requests at once, then 1/s.
	limited := httptest.NewServer(NewHandler(Config{Provider: cfg.Provider, Gen: cfg.Gen, RateLimit: 1, Burst: 3}))
	defer limited.Close()
	var codes []int
	for range 5 {
		r = do(limited.Client(), http.MethodPost, limited.URL+"/v1/summarize", body, nil)
		codes = append(codes, r.status)
	}
	fmt.Printf("5 requests against burst 3 -> %v, Retry-After: %s\n", codes, r.header.Get("Retry-After"))
}

func hardening(cfg Config) {
	srv := httptest.NewUnstartedServer(NewHandler(cfg))
	harden(srv.Config)
	srv.Config.ReadHeaderTimeout = 200 * time.Millisecond // short, for the demo
	srv.Start()
	defer srv.Close()

	big := `{"report": "` + strings.Repeat("x", maxBodyBytes) + `"}`
	r := do(srv.Client(), http.MethodPost, srv.URL+"/v1/summarize", big, nil)
	fmt.Printf("POST with a %d KiB body -> %d %s\n", len(big)>>10, r.status, r.body)

	// Slowloris: send half a request and never finish the headers.
	conn, err := net.Dial("tcp", srv.Listener.Addr().String())
	if err != nil {
		fmt.Println(err)
		return
	}
	defer conn.Close()
	fmt.Fprint(conn, "POST /v1/summarize HTTP/1.1\r\nHost: example\r\n")
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second)) // safety net for the demo
	_, err = bufio.NewReader(conn).ReadByte()
	fmt.Printf("slow client with unfinished headers -> server closed the connection: %v\n", errors.Is(err, io.EOF))
}

func concurrency() {
	// Each call takes 50 ms of simulated provider latency.
	gen, _ := NewGenerator("echo", 50*time.Millisecond)
	reports := make([]string, 8)
	for i := range reports {
		reports[i] = fmt.Sprintf("Report %d: %s", i+1, report)
	}
	ctx := context.Background()

	start := time.Now()
	for _, r := range reports {
		if _, err := SummarizeReport(ctx, gen, r); err != nil {
			fmt.Println(err)
		}
	}
	sequential := time.Since(start)

	// Concurrency pays here because the work is waiting on I/O. wg.Go
	// (Go 1.25) replaces the Add/Done bookkeeping. Each goroutine writes its
	// own slice element, so the results need no lock.
	start = time.Now()
	summaries := make([]string, len(reports))
	errs := make([]error, len(reports))
	var wg sync.WaitGroup
	for i, r := range reports {
		wg.Go(func() { summaries[i], errs[i] = SummarizeReport(ctx, gen, r) })
	}
	wg.Wait()
	concurrent := time.Since(start)
	if err := errors.Join(errs...); err != nil {
		fmt.Println(err)
	}

	fmt.Printf("sequential: %d reports in %v\n", len(reports), sequential.Round(10*time.Millisecond))
	fmt.Printf("concurrent: %d reports in %v\n", len(reports), concurrent.Round(10*time.Millisecond))
	fmt.Printf("first summary: %q\n", summaries[0])
}

func serve(cfg Config) error {
	srv := &http.Server{Addr: "localhost:8080", Handler: NewHandler(cfg)}
	harden(srv)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	errc := make(chan error, 1)
	go func() { errc <- srv.ListenAndServe() }()
	log.Printf("listening on http://%s (provider %s)", srv.Addr, cfg.Provider)

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}

func section(title string) {
	fmt.Printf("\n== %s ==\n", title)
}
