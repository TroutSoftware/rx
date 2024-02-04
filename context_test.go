package rx

import (
	"math/rand"
	"testing"
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

	for i := 0; i < b.N; i++ {

		orderkeys := rand.Perm(6)
		ctx := Context{vx: &vctx{kv: make(map[ContextKey]any)}}

		for i := 0; i < 30_000; i++ {
			rnd := rand.Uint32()
			va := values[rnd%16]

			switch {
			case rnd&(1<<key5) > 0:
				ctx = WithValue(ctx, ContextKey(key5+uint32(orderkeys[i%6])), va)
			case rnd&(1<<key4) > 0:
				ctx = WithValue(ctx, ContextKey(key4+uint32(orderkeys[i%6])), va)
			case rnd&(1<<key3) > 0:
				ctx = WithValue(ctx, ContextKey(key3+uint32(orderkeys[i%6])), va)
			case rnd&(1<<key2) > 0:
				ctx = WithValue(ctx, ContextKey(key2+uint32(orderkeys[i%6])), va)
			case rnd&key1 > 0:
				ctx = WithValue(ctx, ContextKey(key1+uint32(orderkeys[i%6])), va)
			}

			b.ReportMetric(float64(len(ctx.vx.kv)), "valuelength")
		}
	}
}
