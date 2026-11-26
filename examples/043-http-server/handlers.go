package main

import (
	"fmt"
	"net/http"
	"time"
)

// UserStore is the dependency UserHandler needs. The slide injects a *sql.DB;
// an interface with a fake implementation keeps the example stdlib-only and
// shows why injection matters: tests hand in whatever store they like.
type UserStore interface {
	UserName(id string) (name string, ok bool)
}

// MemUserStore is an in-memory UserStore. It is read-only after construction,
// so concurrent handler goroutines can share it without a lock.
type MemUserStore map[string]string

func (s MemUserStore) UserName(id string) (string, bool) {
	name, ok := s[id]
	return name, ok
}

// UserHandler is a struct that implements http.Handler (slide p03). Its
// dependencies are fields, so they are visible and replaceable.
type UserHandler struct {
	Store UserStore
}

func (h *UserHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	name, ok := h.Store.UserName(id)
	if !ok {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprintf(w, "user %s: %s\n", id, name)
}

// homeHandler is an ordinary function turned into a Handler by the
// http.HandlerFunc adapter (slide p04). The slide also calls
// w.WriteHeader(http.StatusOK); that is redundant, because the first Write
// sends 200 implicitly.
func homeHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprintln(w, "Welcome to the API")
}

// healthHandler is the /health endpoint that slide p06 registers but never
// defines.
func healthHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "ok")
}

// newMux builds a local ServeMux (slide p05). Nothing here touches
// http.DefaultServeMux, so no imported package can register routes on it and
// every test can build its own.
func newMux(store UserStore) *http.ServeMux {
	mux := http.NewServeMux()
	mux.Handle("/users", &UserHandler{Store: store})
	mux.Handle("/", http.HandlerFunc(homeHandler))
	mux.HandleFunc("/health", healthHandler)
	return mux
}

// newServer returns an http.Server with every timeout set (slide p07).
//
//   - ReadHeaderTimeout bounds the time to read the request line and headers.
//     It is the field aimed at slowloris clients that connect and send
//     nothing (or trickle bytes). The slide leaves it out.
//   - ReadTimeout bounds reading the whole request, body included.
//   - WriteTimeout bounds the time from the end of the header read to the end
//     of the response write. It also cuts off long streaming or SSE
//     responses; a handler can relax it for itself with
//     http.NewResponseController(w).SetWriteDeadline (Go 1.20+).
//   - IdleTimeout bounds how long a keep-alive connection waits for the next
//     request.
//
// http.ListenAndServe(addr, h) builds a Server with all of these at zero,
// meaning no limit at all. There is no "DefaultServer" identifier; the
// danger is simply the zero values.
func newServer(handler http.Handler, readHeaderTimeout time.Duration) *http.Server {
	return &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
}

// demoStore is the fake store main wires in.
func demoStore() MemUserStore {
	return MemUserStore{"1": "Ada", "2": "Grace"}
}
