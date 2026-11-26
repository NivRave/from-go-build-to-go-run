// The /v2 suffix is part of the module path (slide p02): v2 is a different
// module from v1, so both can be in one build. This repository keeps v2 in a
// v2/ subdirectory (the "major subdirectory" layout); a v2 branch whose
// go.mod says example.com/greeter/v2 works the same way.
module example.com/greeter/v2

go 1.26.0
