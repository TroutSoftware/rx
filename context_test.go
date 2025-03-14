package rx

import (
	"math/rand"
	"net/netip"
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func BenchmarkContextValues(b *testing.B) {
	const (
		key1 uint32 = 1 + iota
		key2
		key3
		key4
		key5
		key6
	)
	values := []any{
		"I", "am", "a", "string",
		42, 24, 125, 122,
		[]string{"hello", "world"}, []string{"john", "doe"},
		[]string{"can", "you"}, []string{"talk", "english"},
		[]int{1, 2}, []int{3, 4},
		[]int{4, 5}, []int{6, 7},
	}
	if len(values) != 16 {
		b.Fatal("change the division below to make sure the context key also rotate")
	}
	// simulation note: pattern acces to values is usually skewed towards recent values
	// to reproduce this, we use the order in switch, making sure that, statistically, key5 is modified twice each time key4 is modified, and so on…

	for b.Loop() {
		ctx := Context{vx: &vctx{kv: make(map[reflect.Type]any)}}

		for range 30_000 {
			rnd := rand.Uint32()
			va := values[rnd%16]

			switch {
			case rnd&(1<<key5) > 0:
				ctx = WithValue(ctx, va)
			case rnd&(1<<key4) > 0:
				ctx = WithValue(ctx, va)
			case rnd&(1<<key3) > 0:
				ctx = WithValue(ctx, va)
			case rnd&(1<<key2) > 0:
				ctx = WithValue(ctx, va)
			case rnd&key1 > 0:
				ctx = WithValue(ctx, va)
			}

			b.ReportMetric(float64(len(ctx.vx.kv)), "valuelength")
		}
	}
}

func TestMutator(t *testing.T) {
	var ctx Context

	type User struct{ name string }
	type Endpoint int
	type Dst netip.Addr

	ctx = WithValues(ctx, User{name: "Doe"}, Endpoint(10))

	Mutate(
		func(u *User) { u.name = "Bond" },
		func(e *Endpoint) { *e = 10 },
		func(d *Dst) { *d = Dst(netip.MustParseAddr("192.0.2.10")) },
	)(ctx)

	want := User{name: "Bond"}
	if gu := ValueOf[User](ctx); gu != want {
		t.Errorf("different user %s", cmp.Diff(want, gu))
	}
}
