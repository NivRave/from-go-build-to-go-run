module example.com/app

go 1.26.0

require (
	example.com/greeter v1.0.0
	example.com/greeter/v2 v2.0.0
)

// Local development with replace (slide p03). Neither module is published,
// so these directives make the build use the sibling directories instead.
// replace only applies when this module is the main module: if another module
// depended on example.com/app, these lines would be ignored for it. For
// day-to-day multi-module work, go.work (see the README) avoids editing
// go.mod at all.
replace (
	example.com/greeter => ../greeter
	example.com/greeter/v2 => ../greeter/v2
)
