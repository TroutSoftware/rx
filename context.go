package rx

import (
	"sync"
)

// Context carries a set of values down the rendering tree.
// This is used by UI elements to pass values between rendering passes.
// A build context can safely be shared between goroutines, and so can the children.
// The zero build context is valid, but only marginally useful, as it cannot be used to link nodes to widgets.
// Do not confuse it with the standard library’s [context.Context], which does allow to pass values, but also a lot more.
//
// For a good introduction and uses, the Dart [InheritedWidget] class is a good start.
//
// [InheritedWidget]: https://api.flutter.dev/flutter/widgets/InheritedWidget-class.html
type Context struct {
	ng *Engine
	vx *vctx
}

// NoAction is a marker context, which is going to prevent a render cycle from happening.
// This is only useful as a performance optimisation for reacting to events, preventing an otherwise useless re-rendering.
// The engine enforces this by randomly ignoring the optimisation.
var NoAction Context

func DoNothing(ctx Context) Context { return NoAction }

// vctx is a lock-protected map.
// TODO should not be protected, context is not thread-safe
// TODO this should be a structurally shareable data structure
type vctx struct {
	ml sync.Mutex
	kv map[ContextKey]any
}

// ContextKey is a unique key to identify a value in the context.
// To ensure uniqueness, users of the library should start their own keys using [LastRXKey]:
//
//	 const (
//	 	RESTProviderKey = rx.LastRXKey + iota
//	 	DataStoreKey
//			...
//	 )
type ContextKey uint16

const (
	NoKey ContextKey = iota
	ErrorKey
	RootKey
	LastRXKey // use as starting point

)

// WithValue adds a new value in the context, which should be passed down the building stack.
// Existing values of the same key are hidden, but not overwritten.
//
// # Concurrency note
//
// The happens-after relationship could look a bit counter-intuitive; without further synchronization, two goroutines G1 and G2 would be able to write their value, but read the value from the other goroutine.
// We believe this is an acceptable tradeoff as this is not a common case, and adding synchronization (e.g. through channels) is both trivial, and clearer anyway.
// We do ensure that the data structure remains valid from concurrent access.
func WithValue(ctx Context, key ContextKey, value any) Context {
	if ctx.vx == nil {
		ctx.vx = &vctx{kv: make(map[ContextKey]any)}
	}

	ctx.vx.ml.Lock()
	ctx.vx.kv[key] = value
	ctx.vx.ml.Unlock()
	return ctx
}
func WithValues(ctx Context, v ...any) Context {
	if ctx.vx == nil {
		ctx.vx = &vctx{kv: make(map[ContextKey]any)}
	}

	ctx.vx.ml.Lock()
	for i := 0; i < len(v); i += 2 {
		ctx.vx.kv[v[i].(ContextKey)] = v[i+1]
	}

	ctx.vx.ml.Unlock()
	return ctx
}

// Value retrieves the value matching the corresponding key.
// If no such value exists, nil is returned.
//
// Value is deprecated, [ValueOf] should be used instead
func Value(ctx Context, key ContextKey) any {
	vx := ctx.vx
	if vx == nil {
		return nil
	}

	vx.ml.Lock()
	val := vx.kv[key]
	vx.ml.Unlock()
	return val
}

// ValueOf returns a value of type T at key.
// If the type of T is invalid, the function panics.
func ValueOf[T any](ctx Context, key ContextKey) T {
	var z T

	vx := ctx.vx
	if vx == nil {
		return z
	}

	val, ok := vx.kv[key]
	if !ok {
		return z
	}
	return val.(T)

}

// CleanValue deletes all values corresponding for a given key.
func CleanValue(ctx Context, key ContextKey) {
	vx := ctx.vx
	if vx == nil {
		return
	}

	vx.ml.Lock()
	delete(vx.kv, key)
	vx.ml.Unlock()
}
