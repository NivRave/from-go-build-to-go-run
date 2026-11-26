// Command middleware-and-routing demonstrates net/http middleware (logging
// with a status recorder, panic recovery, auth via the request context), a
// Chain helper that applies middleware in the declared order, and Go 1.22
// routing patterns with a sub-mux.
//
// Slides: slides/044-middleware-and-routing.pdf
//
// Run:
//
//	go run ./examples/044-middleware-and-routing
package main

import (
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
)

func main() {
	section("1. Logging with a status recorder (slide p03)")
	loggingDemo()

	section("2. Recovery and Auth through the full stack (slides p07-p08)")
	recoveryAndAuth()

	section("3. Chain order (slide p04)")
	chainOrder()

	section("4. Routes: method patterns and a sub-mux (slides p05-p06)")
	routes()
}

// demoTokens maps bearer tokens to user IDs.
var demoTokens = map[string]string{"secret-token": "user-42"}

// newLogger writes text logs to stdout. It drops the time and duration
// attributes so the demo output is the same on every run.
func newLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey || a.Key == "duration" {
				return slog.Attr{}
			}
			return a
		},
	}))
}

// newApp builds the routes and wraps them in the middleware stack.
func newApp(logger *slog.Logger) http.Handler {
	mux := http.NewServeMux()

	// Method + wildcard patterns need Go 1.22+ and a go.mod saying so.
	mux.HandleFunc("GET /users/{id}", func(w http.ResponseWriter, r *http.Request) {
		user, _ := UserFrom(r.Context())
		fmt.Fprintf(w, "user %s (requested by %s)", r.PathValue("id"), user)
	})

	// The sub-mux sees paths with "/api/v1" stripped, so it registers
	// "/users/{id}", and PathValue comes from the pattern it matched.
	v1 := http.NewServeMux()
	v1.HandleFunc("GET /users/{id}", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "v1 user %s", r.PathValue("id"))
	})
	mux.Handle("/api/v1/", http.StripPrefix("/api/v1", v1))

	mux.HandleFunc("/panic", func(w http.ResponseWriter, r *http.Request) {
		panic("database handle is nil") // stands in for any bug that panics
	})
	mux.HandleFunc("/abort", func(w http.ResponseWriter, r *http.Request) {
		panic(http.ErrAbortHandler) // "drop this connection", handled silently by net/http
	})

	// Logging is outermost so it also records the 500s that Recovery
	// produces; Recovery sits above everything that might panic.
	return Chain(mux, Logging(logger), Recovery(logger), Auth(demoTokens))
}

// newServer starts the app on an httptest server. Its ErrorLog is silenced
// because /abort deliberately makes net/http drop a connection.
func newServer() *httptest.Server {
	srv := httptest.NewUnstartedServer(newApp(newLogger()))
	srv.Config.ErrorLog = log.New(io.Discard, "", 0)
	srv.Start()
	return srv
}

// get performs one request and prints the result on a single line.
func get(client *http.Client, url, token string) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		fmt.Println("bad request:", err)
		return
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := client.Do(req)
	if err != nil {
		// The text differs by OS (EOF vs connection reset), so print a fact.
		fmt.Printf("-> %s: connection aborted, no response\n", req.URL.Path)
		return
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("read body:", err)
		return
	}
	fmt.Printf("-> %s: %d %q\n", req.URL.Path, resp.StatusCode, strings.TrimSpace(string(body)))
}

func loggingDemo() {
	logger := newLogger()
	teapot := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		// Flush goes through statusRecorder.Unwrap to the real writer.
		// Without Unwrap this would return http.ErrNotSupported.
		err := http.NewResponseController(w).Flush()
		fmt.Printf("handler: Flush through the wrapper, err = %v\n", err)
	})
	h := Logging(logger)(teapot)
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/coffee", nil))
}

func recoveryAndAuth() {
	srv := newServer()
	defer srv.Close()
	client := srv.Client()

	get(client, srv.URL+"/users/7", "")           // no token: Auth stops it
	get(client, srv.URL+"/users/7", "wrong")      // unknown token
	get(client, srv.URL+"/panic", "secret-token") // Recovery answers 500
	get(client, srv.URL+"/abort", "secret-token") // re-panicked, connection dropped
}

func chainOrder() {
	var trace []string
	tracer := func(name string) Middleware {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				trace = append(trace, name+"-before")
				next.ServeHTTP(w, r)
				trace = append(trace, name+"-after")
			})
		}
	}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		trace = append(trace, "handler")
	})

	run := func(label string, h http.Handler) {
		trace = nil
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
		fmt.Printf("%-22s %s\n", label, strings.Join(trace, " "))
	}
	run("Chain(h, A, B):", Chain(handler, tracer("A"), tracer("B")))
	run("SlideChain(h, A, B):", SlideChain(handler, tracer("A"), tracer("B")))
}

func routes() {
	srv := newServer()
	defer srv.Close()
	client := srv.Client()

	get(client, srv.URL+"/users/7", "secret-token")
	get(client, srv.URL+"/api/v1/users/9", "secret-token")
	get(client, srv.URL+"/nope", "secret-token")
}

func section(title string) {
	fmt.Printf("\n== %s ==\n", title)
}
