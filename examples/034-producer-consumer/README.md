# Part 034: The Producer/Consumer Pattern

Day 17 · Week 3 · Building resilient pipelines.

## What this shows

- `produce(ctx, items) <-chan string`: the producer creates the channel, fills it from its own goroutine, closes it with `defer close(out)`, and stops early through a ctx-aware `select`.
- A pool of three consumers started with `wg.Go`, each ranging over the same channel with 10 ms of simulated work per item.
- Backpressure: a producer that could send everything at once, two slow consumers and a buffer of 3. `len(ch)` sits at `cap` and the producer takes as long as the consumers do.
- Close drains, cancel stops: the same pipeline run twice. Closing the channel lets the consumers finish **every** queued item; cancelling the context stops them early.
- Graceful shutdown with `signal.NotifyContext(ctx, os.Interrupt)` plus a 300 ms `context.WithTimeout` fallback, so the demo ends without Ctrl+C. `-wait` turns the fallback off.

## Run

```sh
go run ./examples/034-producer-consumer
go run ./examples/034-producer-consumer -wait   # demo 5 runs until you press Ctrl+C
go test ./examples/034-producer-consumer
go run -race ./examples/034-producer-consumer   # needs cgo (a C compiler)
```

## What to look for

Output of one run. The timings and the number of jobs in section 5 vary; the counts in section 4 are what `main_test.go` asserts on a fake clock:

```text
== 1. The producer owns the channel (slides p03, p07) ==
received in order: a b c d e
range ended because the producer closed the channel

== 2. A pool of consumers (slides p04-p05) ==
3 workers processed 12 items (10 ms each) in about 40ms

== 3. Backpressure from a bounded buffer (slide p06) ==
len(ch) every 10 ms: [3 3 3 3 3 3 3 2] (cap 3)
the producer needed about 80ms to hand over 12 items it could have sent at once

== 4. Close drains, cancel stops (slide p08) ==
close:            processed 20 of 20
cancel at 25 ms:  processed 6 of 20

== 5. Graceful shutdown (slide p07) ==
stopped by 300 ms fallback timeout: 90 jobs processed, all 3 workers exited cleanly
```

- 12 items × 10 ms of work across 3 workers takes about 40 ms instead of 120 ms.
- In section 3 the buffer stays full until the producer runs out of items: each send waits for a consumer to free a slot, so the slow side sets the pace and memory stays bounded.
- `20 of 20` after a close: `range` (and `consume`'s `ok` check) only ends once the channel is closed **and** empty. `6 of 20` after a cancel: the two workers finished the items they had started at 20 ms, then saw `ctx.Done()` and left the rest of the buffer alone.
- In section 5, Ctrl+C (with `-wait`) and the fallback timeout go through the same path: the context is cancelled, every worker finishes its current job, and `wg.Wait()` returns.

## Notes and corrections

- **Slide p08:** closing a channel doesn't stop consumers early: `range` drains every buffered value first, so close means "no more data", not "stop now". Stopping early needs `ctx.Done()` (or a done channel) in the consumer's `select`. A poison-pill value stops only the one consumer that receives it; N workers need N pills.
- **Slides p02 vs. p06:** the producer's send returns immediately only while the buffer has room. Once it is full, the producer blocks, which is the backpressure slide p06 describes.
- **Slide p07:** use `signal.NotifyContext(context.Background(), os.Interrupt)` to turn Ctrl+C into a cancelled context. When both cases of a `select` are ready, Go picks one at random, so a producer may still send an item or two after the cancel; `consume` checks `ctx.Err()` again before starting new work.
- **Slide p05:** with `wg.Go` (Go 1.25), `consume` doesn't need a `*sync.WaitGroup` parameter, and capturing the loop variable is safe since Go 1.22.
- **Slide p09:** backpressure comes from any bounded channel, unbuffered included; a buffer only adds slack.

## Slides

[034-producer-consumer.pdf](../../slides/034-producer-consumer.pdf)
