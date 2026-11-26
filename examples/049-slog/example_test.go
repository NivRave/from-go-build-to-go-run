package main

// The demos drop the time attribute with ReplaceAttr, so their JSON output
// is the same on every run and can be checked here.

func Example_jsonBasics() {
	jsonBasics()
	// Output:
	// {"level":"INFO","msg":"incoming_request","method":"GET","path":"/users","status":200}
	// {"level":"INFO","msg":"upload","size":1024,"user":"niv"}
}

func Example_groupsAndScopes() {
	groupsAndScopes()
	// Output:
	// {"level":"INFO","msg":"user_update","user":{"id":42,"email":"user@example.com"}}
	// {"level":"INFO","msg":"charged","service":"billing","amount_cents":999}
	// {"level":"INFO","msg":"request done","http":{"method":"GET","status":200}}
}

func Example_contextHandler() {
	contextHandler()
	// Output:
	// {"level":"INFO","msg":"fetching user","user_id":42,"request_id":"req-7f3a"}
	// {"level":"INFO","msg":"query ok","component":"db","rows":1,"request_id":"req-7f3a"}
	// {"level":"INFO","msg":"no context, no request_id"}
}

func Example_dynamicLevel() {
	dynamicLevel()
	// Output:
	// {"level":"INFO","msg":"serving","min_level":"INFO"}
	// {"level":"DEBUG","msg":"cache miss","key":"user:42"}
	// {"level":"INFO","msg":"serving","min_level":"DEBUG"}
}

func Example_setDefault() {
	setDefault()
	// Output:
	// {"level":"INFO","msg":"via slog.Info","who":"new code"}
	// {"level":"INFO","msg":"via log.Printf from legacy code"}
}

func Example_multiHandler() {
	multiHandler()
	// Output:
	// level=INFO msg=fan-out sinks=2
	// {"level":"INFO","msg":"fan-out","sinks":2}
}
