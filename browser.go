//go:build !js

package rx

import (
	"io"
)

type redirectKey struct {
	URL   string
	Blank bool
	Gen   int
}

type UserInput string

func RedirectTo(url string) Action                                     { return DoNothing }
func RedirectToNew(url string) Action                                  { return DoNothing }
func WriteDataTransfer(data string, effect string, image *Node) Action { return DoNothing }
func ReadDataTransfer(ctx Context) string                              { return "" }
func DownloadFile(name string, content io.Reader) Action               { return DoNothing }
func ReadFile(dst io.Writer) Action                                    { return DoNothing }
func CopyToClipboard(text string, flag string) Action                  { return DoNothing }
func UploadTo(id string, flag string) Action                           { return DoNothing }

func ReadInput(ctx Context) string { return string(ValueOf[UserInput](ctx)) }
