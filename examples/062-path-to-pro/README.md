# Part 062: The grand finale: The path to a pro Go engineer

Day 31 · Week 5 · From writing code to architecting systems.

## What this shows

- **API versioning** with Go 1.22 `ServeMux` patterns: `POST /v1/summarize`, `POST /v2/summarize` (adds a `provider` field), and `/v0/summarize` answering `410 Gone` with the standard `Deprecation` (RFC 9745) and `Sunset` (RFC 8594) headers plus a `successor-version` link.
- **Vendor neutrality:** a consumer-side `TextGenerator` interface with two fake providers, `echo` and `reverse`. `-provider` picks one, so swapping vendors is a configuration change.
- **Security middleware:** CSP `frame-ancestors 'none'` alongside `X-Frame-Options` and `X-Content-Type-Options`, Go 1.25's `http.NewCrossOriginProtection()` against CSRF, and a token-bucket rate limiter written with the standard library.
- **Server hardening:** `ReadHeaderTimeout` (and the other `http.Server` timeouts) against Slowloris, and `http.MaxBytesReader` against oversized bodies.
- **Concurrency where it pays:** summarising eight reports sequentially and with `wg.Go`, timed.
- Everything runs in-process through `httptest`; `-serve` listens on `localhost:8080`.

## Run

```sh
go run ./examples/062-path-to-pro
go run ./examples/062-path-to-pro -provider reverse
go test ./examples/062-path-to-pro
go run ./examples/062-path-to-pro -serve          # Ctrl+C to stop
```

With `-serve` running:

```sh
curl -i -X POST localhost:8080/v2/summarize -d '{"report": "a b c"}'
curl -i -X POST localhost:8080/v0/summarize
```

## What to look for

Expected output of `go run` (only the two timings in section 5 vary):

```text
== 1. Path versioning and a retired version (slide p06) ==
POST /v1/summarize -> 200 {"summary":"Revenue grew 12 percent while"}
POST /v2/summarize -> 200 {"summary":"Revenue grew 12 percent while","provider":"echo"}
POST /v0/summarize -> 410 v0 was removed; use /v2/summarize
  Deprecation: @1735689600
  Sunset: Thu, 01 Jan 2026 00:00:00 GMT
  Link: </v2/summarize>; rel="successor-version"
GET /v1/summarize -> 405 (the pattern says POST)

== 2. Swapping providers is a configuration change (slide p03) ==
-provider echo    -> "Revenue grew 12 percent while", err = <nil>
-provider reverse -> "third a by fell tickets support while percent 12 grew Revenue", err = <nil>
-provider openai  -> unknown provider "openai" (want echo or reverse)

== 3. Security middleware (slide p07) ==
Content-Security-Policy: default-src 'none'; frame-ancestors 'none'
X-Frame-Options: DENY
X-Content-Type-Options: nosniff
Referrer-Policy: no-referrer
cross-site POST -> 403 cross-origin request detected from Sec-Fetch-Site header
5 requests against burst 3 -> [200 200 200 429 429], Retry-After: 1

== 4. Server hardening (slide p07) ==
POST with a 64 KiB body -> 413 request body larger than 65536 bytes
slow client with unfinished headers -> server closed the connection: true

== 5. Sequential vs concurrent, measured (slides p05, p10) ==
sequential: 8 reports in 400ms
concurrent: 8 reports in 50ms
first summary: "Report 1: Revenue grew 12"
```

- The method is part of the route: `GET /v1/summarize` gets `405 Method Not Allowed` from the mux itself, with no code in the handler.
- `Deprecation: @1735689600` is 2025-01-01 as Unix seconds, the RFC 9745 format; `Sunset` is an HTTP date. Clients and gateways can act on both without parsing the error text.
- Every response carries the security headers, including the 403 from cross-origin protection and the 429 from the rate limiter, because `SecurityMiddleware` is the outermost layer. The 403 comes from the browser's `Sec-Fetch-Site: cross-site` header; `curl` and other non-browser clients send no such header and pass.
- The slow client sent half a request and stopped. With `ReadHeaderTimeout` set (200 ms for the demo, 2 s in `harden`), the server dropped the connection; without it, each such client would hold a connection open indefinitely.
- Eight 50 ms provider calls take 400 ms in a loop and about 50 ms concurrently, because the work is waiting on I/O. For CPU-bound work or a single call, the plain loop is simpler and just as fast (slide p05).

The `Example` functions in `server_test.go` assert sections 1-3, so this output can't go stale. The other tests cover both versions, the 410 and its headers, the security headers, cross-origin protection, the rate limit and its refill (with `testing/synctest`), bad and oversized bodies, and a fake generator plugged in through the interface.

## Notes and corrections

- **Slide p10:** "use your pprof skills (from Day 29)" refers to Part 057. The chart shows 2000 ms of AI latency while the text says 2.5 s; the point stands either way: 5 ms is noise next to a multi-second call.
- **Slide p06:** `Accept-Version` isn't a standard HTTP header. Use path versioning (as here) or media-type versioning (`Accept: application/vnd.example.v2+json`), and announce retirement with the standard `Deprecation` (RFC 9745) and `Sunset` (RFC 8594) headers.
- **Slide p07:** `X-Frame-Options` is superseded by CSP `frame-ancestors`; send both for older browsers. Since Go 1.25 the standard library has `http.CrossOriginProtection` for CSRF. Server-level settings (`ReadHeaderTimeout`, `MaxBytesReader`) protect more than any response header.
- **Slide p03:** `SummarizeReport` has no return values on the slide; here it returns `(string, error)`.
- **Slide p05:** when concurrency is warranted, `sync.WaitGroup.Go` (Go 1.25) or `errgroup` removes the `Add`/`Done` bookkeeping.

## Slides

[062-path-to-pro.pdf](../../slides/062-path-to-pro.pdf)
