package main

import "strings"

// Client is the dependency Greeting needs (slide p06). In production it
// might call an HTTP API; tests pass a MockClient (mock_test.go).
type Client interface {
	Get() string
}

// Greeting is the consumer under test.
func Greeting(c Client) string {
	name := strings.TrimSpace(c.Get())
	if name == "" {
		return "Hello, stranger!"
	}
	return "Hello, " + name + "!"
}

// StaticClient is a trivial real Client, used by main.
type StaticClient struct{ Name string }

func (s StaticClient) Get() string { return s.Name }
