# Part 033: The Anatomy of Channels

Day 17 · Week 3 · Synchronization vs. buffering.

## What this shows

- An unbuffered hand-off where the receiver arrives 50 ms late: comparing timestamps shows the send completed only after the receiver was ready. `main` waits for the sender, so slide p02's "Sent!" can't be lost.
- A buffered channel with capacity 2: two sends with no receiver, `len`/`cap`, and `select` with `default` to show that a third send would block, without deadlocking.
- Directional channel types: `producer(chan<- int)` with `defer close(ch)`, `consumer(<-chan int)` ranging until the close, and comma-ok on the closed channel giving the zero value and `false`.
- Broadcast: one `close(done)` wakes 100 goroutines started with `wg.Go`, and all of them exit.
- Nil channels: sending to or receiving from one blocks forever (shown with `select` and `default`), and closing one panics (recovered).
- Benchmarks moving one million ints one at a time, in batches of 1,000, and through a mutex-protected slice.

## Run

```sh
go run ./examples/033-anatomy-of-channels
go test ./examples/033-anatomy-of-channels
go test -run='^$' -bench=. -benchmem ./examples/033-anatomy-of-channels
```

## What to look for

Expected output of `go run` (only the duration in section 1 varies):

```text
== 1. Unbuffered: the send waits for the receiver (slide p02) ==
data
Sent!
send completed after the receiver arrived: true
sender was blocked for about 50ms

== 2. Buffered: sends succeed until the buffer is full (slides p03-p04) ==
len=2 cap=2: buffer is full, but the sender was not blocked
third send would block
message 1
len=1 cap=2

== 3. Directional channels, close and comma-ok (slides p05-p06) ==
Received: 0
Received: 1
Received: 2
Received: 3
Received: 4
Channel closed, cleaning up...
after close: val=0 ok=false

== 4. close(done) as a broadcast (slide p07) ==
100 of 100 goroutines exited after one close(done)

== 5. Nil channels (slide p06) ==
send to nil channel: would block forever
receive from nil channel: would block forever
close(nil channel) panicked: close of nil channel
```

Benchmark shape on one machine (numbers vary by machine):

```text
BenchmarkChannelOneByOne   ~50 ms/op     ~900 B/op      3 allocs/op
BenchmarkChannelBatched    ~1.9 ms/op   ~8.2 MB/op   1004 allocs/op
BenchmarkMutexSlice        ~4.3 ms/op     ~70 B/op      4 allocs/op
```

- The sender was blocked for the full 50 ms: with no buffer, a send can't complete until a receiver takes the value.
- `val=0 ok=false`: a closed, drained channel answers every receive immediately with the zero value. Without comma-ok you can't tell that from a real 0.
- Sending one int at a time is about 25 times slower than sending batches of 1,000, because every operation pays the channel's synchronisation and goroutine wake-ups. The batched version allocates a new slice per batch, since ownership passes to the receiver. The uncontended mutex sits in between.
- The directional types make a consumer that closes its input a compile error: `invalid operation: cannot close receive-only channel ch (variable of type <-chan int)`.

## Notes and corrections

- **Slide p02:** "Sent!" isn't guaranteed to print, because `main` may return right after its receive, before the goroutine reaches the `Println`. The hand-off claim itself is right: a receive on an unbuffered channel is synchronized before the send completes.
- **Slide p07:** a `chan struct{}` element takes 0 bytes, but the channel itself is a heap object of about 96 bytes (the runtime's `hchan`). A `select` with a `default` case inside a loop busy-polls, so it is only reasonable when `doWork()` does real, bounded work; a listener that only waits should block on `<-done`, as section 4 does. The text says 1,000 goroutines and the comment 100, while the code starts one.
- **Slide p08:** channels use the runtime's internal lock, not `sync.Mutex`, and some operations (such as a non-blocking send on a full channel) take a lock-free fast path. "Every send and receive takes a lock" is a fair simplification.
- **Slide p06:** closing a nil channel panics, and receiving from (or sending to) a nil channel blocks forever. Closing an already-closed channel, or sending on one, also panics.

## Slides

[033-anatomy-of-channels.pdf](../../slides/033-anatomy-of-channels.pdf)
