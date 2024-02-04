// package rx is a work in progress to cleanly separate the code in rendering from the domain logic
// it is meant to be dot-imported.
package rx

import "fmt"

func assert(pred bool, msg string, args ...any) {
	if !pred {
		panic(fmt.Sprintf(msg, args...))
	}
}
