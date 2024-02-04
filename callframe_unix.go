//go:build !js

package rx

import "io"

type Type int

// from syscall/js.Value
// TODO(rdo) make this available for unit testing
type JSValue interface {
	Bool() bool
	Call(m string, args ...any) JSValue
	Delete(p string)
	Equal(w JSValue) bool
	Float() float64
	Get(p string) JSValue
	Index(i int) JSValue
	InstanceOf(t JSValue) bool
	Int() int
	Invoke(args ...any) JSValue
	IsNaN() bool
	IsNull() bool
	IsUndefined() bool
	Length() int
	New(args ...any) JSValue
	Set(p string, x any)
	SetIndex(i int, x any)
	String() string
	Truthy() bool
	Type() Type
}

// CallFrame mock implementation for Unix
type CallFrame struct {
	IntentType
	Entity

	Mouse     Coord
	Point     int
	Registers [4]JSValue
	Returns   [4]JSValue
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
func S1(ctx Context, v any) { ctx.ng.Returns[0] = ValueOf(v) } //lint:ignore U1000 this is an API
func S2(ctx Context, v any) { ctx.ng.Returns[1] = ValueOf(v) } //lint:ignore U1000 this is an API
func S3(ctx Context, v any) { ctx.ng.Returns[2] = ValueOf(v) } //lint:ignore U1000 this is an API
func S4(ctx Context, v any) { ctx.ng.Returns[3] = ValueOf(v) } //lint:ignore U1000 this is an API

func ValueOf(any) JSValue { panic("not implemented") }

func Pipe() (JSValue, io.WriteCloser) {
	panic("not implemented")
}
