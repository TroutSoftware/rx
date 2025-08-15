//go:build js

package rx

import (
	"io"
	"syscall/js"

	"github.com/TroutSoftware/rx/internal/sys"
)

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

// DownloadFile triggers a browser file download
func DownloadFile(name string, content io.Reader) Action {
	return func(ctx Context) Context {
		S1(ctx, "trigger-download")
		S2(ctx, name)
		r, buf := sys.Pipe()
		go func() {
			io.Copy(buf, content)
			buf.Close()
		}()
		S3(ctx, r)
		return ctx
	}
}

// ReadFile starts reading file into buf.
// buf should be read in a dedicated goroutine.
func ReadFile(dst io.Writer) Action {
	return func(ctx Context) Context {
		S1(ctx, "read-file")
		chunk := js.FuncOf(func(this js.Value, args []js.Value) any {
			len := args[0].Get("length").Int()
			buf := make([]byte, len)
			js.CopyBytesToGo(buf, args[0])
			dst.Write(buf)
			return js.Null()
		})
		S2(ctx, chunk)
		return ctx
	}
}
