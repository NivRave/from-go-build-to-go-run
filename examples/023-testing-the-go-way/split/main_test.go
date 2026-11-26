package split_test

import (
	"flag"
	"fmt"
	"maps"
	"testing"
)

// fixtureRecords stands in for an expensive shared resource (a database
// container, a mock server) that every test in the package reuses. TestMain
// creates it once.
var fixtureRecords map[string]string

// TestMain runs instead of the tests and decides when they run (slide p05).
//
// It does not call os.Exit(m.Run()). Since Go 1.15, when TestMain returns,
// the test binary exits with the code m.Run returned. os.Exit would also
// skip every deferred call, so a teardown written with defer would never run.
func TestMain(m *testing.M) {
	flag.Parse() // TestMain must parse flags itself before using testing.Verbose
	teardown := setup()
	defer teardown()

	m.Run()
}

func setup() (teardown func()) {
	fixtureRecords = map[string]string{
		"user:1": "Ada,Lovelace,1815",
		"user:2": "Alan,Turing,1912",
	}
	logf("TestMain: setup done, %d fixture records", len(fixtureRecords))
	return func() {
		clear(fixtureRecords)
		logf("TestMain: teardown done")
	}
}

// copyFixtures gives a test its own copy, so no test can leak changes into
// another through the shared fixture.
func copyFixtures() map[string]string {
	return maps.Clone(fixtureRecords)
}

func logf(format string, args ...any) {
	if testing.Verbose() {
		fmt.Printf(format+"\n", args...)
	}
}
