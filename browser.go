//go:build !js

package rx

import "io"

func RedirectTo(url string) Action                                     { return DoNothing }
func ReadInput(ctx Context) string                                     { return "" }
func WriteDataTransfer(data string, effect string, image *Node) Action { return DoNothing }
func ReadDataTransfer(ctx Context) string                              { return "" }
func DownloadFile(name string, content io.Reader) Action               { return DoNothing }
func ReadFile(dst io.Writer) Action                                    { return DoNothing }
