// Package sys provides low-level primitives for interacting with Javascript
package sys

import (
	"fmt"
	"io"
	"runtime"
	"syscall/js"
)

func Pipe() (js.Value, io.WriteCloser) {
	buf := make(chan streampull, 1)
	readInto := js.FuncOf(func(this js.Value, args []js.Value) any {
		buf <- streampull{args[0], args[1]}
		return js.Null()
	})
	// second alloc, make sure readInto does not hold a ref to p
	// which would prevent the cleanup to happen
	p := &pipe{buf}
	runtime.AddCleanup(p, func(_ int) { readInto.Release() }, 0)

	r := js.Global().Call("trout_sftw_openPipe", readInto)
	return r, p
}

type pipe struct {
	buf chan streampull
}

type streampull struct{ buf, cb js.Value }

func (p *pipe) Write(dt []byte) (n int, err error) {
	n = 0
	for n < len(dt) {
		sp := <-p.buf
		sz := js.CopyBytesToJS(sp.buf, dt[n:])
		go sp.cb.Invoke(sz)
		n += sz
	}
	return n, nil
}

func (p *pipe) Close() error {
	const noMoreData = -1

	sp := <-p.buf
	sp.cb.Invoke(noMoreData)
	return nil
}
