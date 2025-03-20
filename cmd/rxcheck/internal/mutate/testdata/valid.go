package P

import (
	"github.com/TroutSoftware/rx"
)

var _ = rx.Mutate(func(x *int) { *x = 2 })
