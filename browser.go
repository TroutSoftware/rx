package rx

// RedirectTo sends browser to url
// It should be triggered by click event only to prevent anti-popup
func RedirectTo(url string) Action {
	return func(ctx Context) Context {
		S1(ctx, "redirect")
		S2(ctx, url)
		return ctx
	}
}

// ReadValue is available on Change and Blur intents
// It reads the value of the underlying element (e.g. input)
func ReadInput(ctx Context) string { return R1(ctx) }
