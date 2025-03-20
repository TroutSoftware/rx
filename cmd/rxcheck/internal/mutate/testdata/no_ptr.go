package P

import (
	"fmt"

	"github.com/TroutSoftware/rx"
)

var _ = rx.Mutate(func(x string) { fmt.Println(x) })          // want "mutate function must be single pointers"
var _ = rx.Mutate(func(x, y *string) { fmt.Println(*x, *y) }) // want "mutate function must be single pointers"
var _ = rx.Mutate("help wanted")                              // want "mutate function must be single pointers"
