package rx

func ExampleKeep() {
	var ctx Context
	type btnKey struct{}

	if nd := Reuse[btnKey](ctx); nd != nil {
		// use nd directly
	}

	nd := Get(`<button>`)
	Keep[btnKey](ctx, nd)
	// return new nd
}
