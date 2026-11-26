// Package greeter is v2 of example.com/greeter. It keeps the package name, so
// importers usually alias one of the two versions when they use both.
package greeter

import "fmt"

var greetings = map[string]string{
	"en": "Hello",
	"es": "Hola",
	"he": "Shalom",
}

// Hello is the v2 API: it takes a language and can fail, a breaking change
// that required a new major version (slide p02).
func Hello(name, lang string) (string, error) {
	g, ok := greetings[lang]
	if !ok {
		return "", fmt.Errorf("greeter/v2: unsupported language %q", lang)
	}
	return g + ", " + name + "!", nil
}
