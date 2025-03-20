package rx

import "fmt"

func assert(pred bool, msg string, args ...any) {
	if !pred {
		panic(fmt.Sprintf(msg, args...))
	}
}
