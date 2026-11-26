module example.com/greeter

go 1.26.0

// Illustrative retractions (slide p05). They have no effect here: the go
// command reads retract directives only from the go.mod of the latest version
// of a module, fetched through a proxy. To retract a published version you
// tag and publish a NEW version (say v1.0.6) whose go.mod contains these
// lines; it may even retract itself. The single version below is already
// covered by the range, which is redundant but harmless.
retract (
	v1.0.1 // Published with a critical race condition.
	[v1.0.0, v1.0.5] // Retract a range of versions.
)
