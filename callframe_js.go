package rx

import (
	"syscall/js"

	"trout.software/rx/internal/sys"
)

var Pipe = sys.Pipe

type CallFrame struct {
	IntentType
	Entity

	Mouse     Coord
	Point     int
	Registers [4]js.Value
	Returns   [4]js.Value
	Modifiers struct {
		CTRL, SHIFT, ALT bool
	}

	Continuation chan CallFrame

	Gen int
}

func R1(ctx Context) string { return ctx.ng.Registers[0].String() }
func R2(ctx Context) string { return ctx.ng.Registers[1].String() }
func R3(ctx Context) string { return ctx.ng.Registers[2].String() }
func R4(ctx Context) string { return ctx.ng.Registers[3].String() }

// Continuation outputs
func S1(ctx Context, v any) { ctx.ng.Returns[0] = js.ValueOf(v) } //lint:ignore U1000 this is an API
func S2(ctx Context, v any) { ctx.ng.Returns[1] = js.ValueOf(v) } //lint:ignore U1000 this is an API
func S3(ctx Context, v any) { ctx.ng.Returns[2] = js.ValueOf(v) } //lint:ignore U1000 this is an API
func S4(ctx Context, v any) { ctx.ng.Returns[3] = js.ValueOf(v) } //lint:ignore U1000 this is an API
