module github.com/TroutSoftware/rx

go 1.25.0

require (
	github.com/google/go-cmp v0.6.0
	golang.org/x/net v0.57.0
	golang.org/x/tools v0.48.0
)

require (
	golang.org/x/mod v0.38.0 // indirect
	golang.org/x/sync v0.22.0 // indirect
)

tool (
	github.com/TroutSoftware/rx/cmd/rxabi
	golang.org/x/tools/cmd/stringer
)
