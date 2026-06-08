package rxtest

import (
	"testing"

	"github.com/TroutSoftware/rx"
)

func TestDoAction(t *testing.T) {
	var ctx rx.Context

	tn := rx.Get(`<div name="test">`).OnIntent(rx.Click, func(ctx rx.Context) rx.Context {
		return rx.WithValue(ctx, "hello")
	})
	nc := Click(ctx, Element{rxNode: tn})
	if got := rx.ValueOf[string](nc); got != "hello" {
		t.Error("invalid state", got)
	}
}
