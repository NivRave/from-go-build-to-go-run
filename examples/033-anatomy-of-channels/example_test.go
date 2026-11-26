package main

// Example tests run under `go test` and fail if stdout differs from the
// // Output: comment. Demo 1 has none because it prints a measured duration.

func Example_buffered() {
	buffered()
	// Output:
	// len=2 cap=2: buffer is full, but the sender was not blocked
	// third send would block
	// message 1
	// len=1 cap=2
}

func Example_directional() {
	directional()
	// Output:
	// Received: 0
	// Received: 1
	// Received: 2
	// Received: 3
	// Received: 4
	// Channel closed, cleaning up...
	// after close: val=0 ok=false
}

func Example_broadcast() {
	broadcast(100)
	// Output: 100 of 100 goroutines exited after one close(done)
}

func Example_nilChannels() {
	nilChannels()
	// Output:
	// send to nil channel: would block forever
	// receive from nil channel: would block forever
	// close(nil channel) panicked: close of nil channel
}
