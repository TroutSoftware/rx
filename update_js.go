package rx

import (
	"syscall/js"
)

// JSUpdate is a callback to let the JS world trigger a new rendering cycle.
// Usually called from the JS shim via
//
//	js.Scope().Set("updateGo", ng.JSUpdate())
func (ngx *Engine) JSUpdate() js.Func {
	return js.FuncOf(func(this js.Value, args []js.Value) any {
		evt := args[0].Int()
		// terminate early
		if IntentType(evt) == Seppuku {
			close(ngx.XAS)
			return js.Null()
		}

		cf := CallFrame{
			Entity:     uint32(args[1].Int()),
			IntentType: IntentType(evt),
		}
		world := args[2]

		cf.Gen = world.Get("gen").Int()
		cf.Mouse = Coord{X: world.Get("mouse").Index(0).Int(), Y: world.Get("mouse").Index(1).Int()}
		// backward compatibility with current datacell implementation.
		// TODO replace this with, e.g. explicit register.
		if v := world.Get("point"); !v.IsNull() && !v.IsUndefined() {
			cf.Point = v.Int()
		}
		regs := world.Get("registers")
		for i := 0; i < 4; i++ {
			cf.Registers[i] = regs.Index(i)
		}
		cf.Modifiers = struct {
			CTRL  bool
			SHIFT bool
			ALT   bool
		}{
			CTRL:  world.Get("modifiers").Index(0).Bool(),
			SHIFT: world.Get("modifiers").Index(1).Bool(),
			ALT:   world.Get("modifiers").Index(2).Bool(),
		}

		if cont := world.Get("continuation"); !cont.IsUndefined() {
			cf.Continuation = make(chan CallFrame)
			go ngx.ReactToIntent(cf)
			cf := <-cf.Continuation
			args := js.Global().Get("Array").New()
			args.Call("push", cf.Returns[0])
			args.Call("push", cf.Returns[1])
			args.Call("push", cf.Returns[2])
			args.Call("push", cf.Returns[3])
			cont.Invoke(args)
		} else {
			go ngx.ReactToIntent(cf)
		}
		return js.Null()
	})
}

// StartLoop creates the infinite rendering loop, calling drawfn each time a view refresh is required.
// Run after all initialization is finished:
//
//	ng.DrawAndLoop(js.Scope().Get("redraw"))
func (ngx *Engine) DrawAndLoop(drawfn js.Value) {
	uintArr := js.Global().Get("Uint8Array")

	ngx.Actions <- func(c Context) Context { return WithValue(c, ngx.Root) }
	for vm := range ngx.XAS {
		prog := uintArr.New(len(vm))
		js.CopyBytesToJS(prog, vm)
		ngx.ReleaseXAS(vm)
		drawfn.Invoke(prog)
	}
}
