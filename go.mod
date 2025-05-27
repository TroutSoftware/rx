module github.com/TroutSoftware/rx

go 1.24.1

require (
	github.com/google/go-cmp v0.6.0
	golang.org/x/net v0.40.0
	golang.org/x/tools v0.33.0
)

require (
	golang.org/x/mod v0.24.0 // indirect
	golang.org/x/sync v0.14.0 // indirect
)

tool (
	github.com/TroutSoftware/rx/cmd/rxabi
	golang.org/x/tools/cmd/stringer
)
