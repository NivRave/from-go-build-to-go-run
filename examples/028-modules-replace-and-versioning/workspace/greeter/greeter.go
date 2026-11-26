// Package greeter is v1 of example.com/greeter.
package greeter

// Hello returns an English greeting. Its signature is the v1 API: changing it
// would break every caller, which is why the new signature lives in
// example.com/greeter/v2 instead.
func Hello(name string) string {
	return "Hello, " + name + "!"
}
