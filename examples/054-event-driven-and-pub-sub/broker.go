package main

import (
	"bytes"
	"errors"
	"sync"
)

// ErrBrokerClosed is returned by Publish after Close.
var ErrBrokerClosed = errors.New("broker is closed")

// Event is the envelope every message travels in. ID identifies one logical
// event: a redelivery carries the same ID, which is what dedup keys on.
type Event struct {
	ID    string
	Topic string
	Type  string
	Data  []byte // JSON payload
}

// Broker is an in-process pub/sub broker (slide p03). Each subscription has
// its own queue and its own goroutine, so a slow subscriber never slows the
// publisher or the other subscribers down.
//
// Like RabbitMQ, it copies each message into every subscriber's queue.
// Log-based brokers such as Kafka or NATS JetStream instead keep one copy of
// the stream and a read offset per consumer.
type Broker struct {
	mu     sync.Mutex
	topics map[string][]*subscription
	closed bool
	pumps  sync.WaitGroup
}

// NewBroker returns an empty broker.
func NewBroker() *Broker {
	return &Broker{topics: make(map[string][]*subscription)}
}

// subscription is one subscriber's queue. The queue is an unbounded slice
// so Publish never blocks; a real broker would bound it and apply
// back-pressure or spill to disk.
type subscription struct {
	mu     sync.Mutex
	queue  []Event
	closed bool
	wake   chan struct{} // capacity 1: "the queue changed"
	out    chan Event
}

// Subscribe returns a channel that receives every event published to topic
// from now on. The channel is closed after Close, once the subscriber's
// queue has drained, so a consumer must keep reading until then.
func (b *Broker) Subscribe(topic string) <-chan Event {
	s := &subscription{wake: make(chan struct{}, 1), out: make(chan Event, 8)}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		close(s.out)
		return s.out
	}
	b.topics[topic] = append(b.topics[topic], s)
	b.pumps.Go(s.pump)
	return s.out
}

// Publish fans a copy of e out to every subscriber of e.Topic. It only
// appends to queues, so it returns immediately however slow they are.
func (b *Broker) Publish(e Event) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return ErrBrokerClosed
	}
	for _, s := range b.topics[e.Topic] {
		c := e
		c.Data = bytes.Clone(e.Data) // each subscriber gets its own copy
		s.enqueue(c)
	}
	return nil
}

// Close stops accepting events, lets every subscription deliver what is
// already queued, closes the subscriber channels and waits for that to
// finish.
func (b *Broker) Close() {
	b.mu.Lock()
	if !b.closed {
		b.closed = true
		for _, subs := range b.topics {
			for _, s := range subs {
				s.close()
			}
		}
	}
	b.mu.Unlock()
	b.pumps.Wait()
}

func (s *subscription) enqueue(e Event) {
	s.mu.Lock()
	s.queue = append(s.queue, e)
	s.mu.Unlock()
	s.signal()
}

func (s *subscription) close() {
	s.mu.Lock()
	s.closed = true
	s.mu.Unlock()
	s.signal()
}

func (s *subscription) signal() {
	select {
	case s.wake <- struct{}{}:
	default: // a wake-up is already pending
	}
}

// pump moves events from the queue to the subscriber's channel at whatever
// rate the subscriber reads them.
func (s *subscription) pump() {
	defer close(s.out)
	for {
		s.mu.Lock()
		for len(s.queue) == 0 {
			if s.closed {
				s.mu.Unlock()
				return
			}
			s.mu.Unlock()
			<-s.wake
			s.mu.Lock()
		}
		e := s.queue[0]
		s.queue[0] = Event{} // drop the reference for the GC
		s.queue = s.queue[1:]
		s.mu.Unlock()

		s.out <- e
	}
}
