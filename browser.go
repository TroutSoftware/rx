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

// WriteDataTransfer attaches a [datatransfer] object to the current drag start.
//
// [datatransfer]: https://developer.mozilla.org/fr/docs/Web/API/DataTransfer
func WriteDataTransfer(data string, effect string, image *Node) Action {
	return func(ctx Context) Context {
		S1(ctx, data)
		S2(ctx, effect)
		if image != nil {
			S3(ctx, image.ElementID(ctx))
		}
		return ctx
	}
}

// ReadDataTransfer returns the data set in a call to [WriteDataTransfer] as a result of a drop action
func ReadDataTransfer(ctx Context) string { return R1(ctx) }
