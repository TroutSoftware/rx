//go:build !js

// empty signatures, makes the IDE happy

package rx

import "io"

func RedirectTo(url string) Action                                     { panic("not implemented") }
func ReadInput(ctx Context) string                                     { panic("not implemented") }
func WriteDataTransfer(data string, effect string, image *Node) Action { panic("not implemented") }
func ReadDataTransfer(ctx Context) string                              { panic("not implemented") }
func DownloadFile(name string, content io.Reader) Action               { panic("not implemented") }
func ReadFile(dst io.Writer) Action                                    { panic("not implemented") }
