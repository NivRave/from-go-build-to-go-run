package main

import "testing"

// MockClient is a test double with a function field (slide p06): each test
// case decides what Get returns, with no mocking framework and no reflection.
type MockClient struct {
	GetFunc func() string
}

// Get calls GetFunc. Calling a nil GetFunc would panic, so it falls back to
// an empty response instead.
func (m MockClient) Get() string {
	if m.GetFunc == nil {
		return ""
	}
	return m.GetFunc()
}

func TestGreeting(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		client MockClient
		want   string
	}{
		{"name", MockClient{GetFunc: func() string { return "Ada" }}, "Hello, Ada!"},
		{"trims space", MockClient{GetFunc: func() string { return "  Ada \n" }}, "Hello, Ada!"},
		{"empty response", MockClient{GetFunc: func() string { return "" }}, "Hello, stranger!"},
		{"nil GetFunc", MockClient{}, "Hello, stranger!"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := Greeting(tt.client); got != tt.want {
				t.Errorf("Greeting() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGreetingCallsClientOnce(t *testing.T) {
	t.Parallel()
	calls := 0
	Greeting(MockClient{GetFunc: func() string { calls++; return "Ada" }})
	if calls != 1 {
		t.Errorf("Get called %d times, want 1", calls)
	}
}
