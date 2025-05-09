package rx

import (
	"reflect"
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

type keyedEntity struct {
	next *keyedEntity
	key  reflect.Type
	val  Entity
}

// NoAction is a marker context, which is going to prevent a render cycle from happening.
// This is only useful as a performance optimisation for reacting to events, preventing an otherwise useless re-rendering.
// The engine enforces this by randomly ignoring the optimisation.
var NoAction Context

// Keep stores an entity of type T in the context.
// The entity will be available during the next cycle by calling the [Reuse] function.
// This is only required for elements where identity matters (e.g. drag / drop / transition).
//
// Most of the elements should not use Keep.
func Keep[T any](ctx Context, nd *Node) {
	typ := reflect.TypeFor[T]()

	p := &ctx.ng.k0
	for (*p) != nil && (*p).key != typ {
		p = &(*p).next
	}
	if nd.Entity == 0 {
		nd.GiveKey(ctx)
	}

	*p = &keyedEntity{
		key: typ,
		val: nd.Entity,
	}
}

// Reuse returns a node of type kept during the previous rendering cycle.
// If no node is kept at T (or was kept more than one rendering cycle ago), nil is returned.
func Reuse[T any](ctx Context) *Node {
	typ := reflect.TypeFor[T]()

	for p := ctx.ng.k1; p != nil; p = p.next {
		if p.key == typ {
			nd := ReuseFrom(ctx, p.val)
			Keep[T](ctx, nd)
			return nd
		}
	}
	return nil
}

func DoNothing(ctx Context) Context { return NoAction }

// vctx is a lock-protected map.
type vctx struct {
	ml sync.Mutex
	kv map[reflect.Type]any
}

// WithValue adds a new value in the context, which should be passed down the building stack.
// Existing values of the same key are hidden, but not overwritten.
//
// # Concurrency note
//
// The happens-after relationship could look a bit counter-intuitive; without further synchronization, two goroutines G1 and G2 would be able to write their value, but read the value from the other goroutine.
// We believe this is an acceptable tradeoff as this is not a common case, and adding synchronization (e.g. through channels) is both trivial, and clearer anyway.
// We do ensure that the data structure remains valid from concurrent access.
func WithValue[T any](ctx Context, value T) Context {
	if ctx.vx == nil {
		ctx.vx = &vctx{kv: make(map[reflect.Type]any)}
	}

	ctx.vx.ml.Lock()
	ctx.vx.kv[reflect.TypeFor[T]()] = value
	ctx.vx.ml.Unlock()
	return ctx
}
func WithValues(ctx Context, v ...any) Context {
	if ctx.vx == nil {
		ctx.vx = &vctx{kv: make(map[reflect.Type]any)}
	}

	ctx.vx.ml.Lock()
	for _, v := range v {
		ctx.vx.kv[reflect.TypeOf(v)] = v
	}

	ctx.vx.ml.Unlock()
	return ctx
}

// ValueOf returns a value of type T at key.
// If the type of T is invalid, the function panics.
func ValueOf[T any](ctx Context) T {
	var z T

	vx := ctx.vx
	if vx == nil {
		return z
	}

	val, ok := vx.kv[reflect.TypeFor[T]()]
	if !ok {
		return z
	}
	return val.(T)
}

// Mutate executes all mutators (which must be functions taking exactly one pointer)
// by loading the value from the context, modifying it with the mutator and storign it.
// If the type is not yet registered in the context, the zero value is used instead
// It panics if the mutators are of the wrong type
func Mutate(mutators ...any) Action {
	return func(ctx Context) Context {
		for _, m := range mutators {
			tt := reflect.TypeOf(m)
			if tt.Kind() != reflect.Func || tt.NumIn() != 1 || tt.In(0).Kind() != reflect.Pointer || tt.NumOut() != 0 {
				panic("mutator must be functions of one pointer argument")
			}

			kt := tt.In(0).Elem()

			ctx.vx.ml.Lock()
			v := reflect.New(kt)
			if vv, ok := ctx.vx.kv[kt]; ok {
				v.Elem().Set(reflect.ValueOf(vv))
			}

			reflect.ValueOf(m).Call([]reflect.Value{v})
			ctx.vx.kv[kt] = v.Elem().Interface()
			ctx.vx.ml.Unlock()
		}
		return ctx
	}
}

// LoadContext loads all values in context (cf [New])
func LoadContext(values ...any) Action {
	return func(ctx Context) Context {
		return WithValues(ctx, values...)
	}
}
