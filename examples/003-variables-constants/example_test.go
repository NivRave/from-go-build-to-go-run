package main

func Example_trinity() {
	trinity()
	// Output:
	// const maxRetries = 10         int      10
	// var name string               string   ""
	// pi := 3.14                    float64  3.14
	// user := "niv_rave"            string   "niv_rave"
	// count, active := 10, true     int      10
	//   (and active)                bool     true
}

func Example_untypedConstants() {
	untypedConstants()
	// Output:
	// int64:   int64 1099511627776
	// float64: float64 1.099511627776e+12
	// huge >> 98 = 4
}

func Example_packageVars() {
	packageVars()
	// Output:
	// databaseURL = ""
	// timeout     = 0s
	// retries     = 0
}

func Example_shadowing() {
	shadowing()
	// Output:
	// 6  Inside block
	// 10 Outside block
}

func Example_shadowedErr() {
	shadowedErr()
	// Output:
	//   (inside the if, timeout = 5s)
	// buggy("5s") = 30s, <nil>
	// buggy("five") = 30s, <nil>
	// fixed("5s") = 5s, <nil>
	// fixed("five") = 0s, timeout "five": time: invalid duration "five"
}

func Example_staticTyping() {
	staticTyping()
	// Output:
	// tmp := 6  -> int 6
	// tmp = 7   -> int 7
}
