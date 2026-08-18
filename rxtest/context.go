package rxtest

import (
	"testing"
	"testing/synctest"

	"github.com/TroutSoftware/rx"
)

// Simple engine for test
type Engine struct {
	actions chan rx.Action
	ctx     rx.Context
}

func NewTestEngine(t *testing.T) (*Engine, chan rx.Action) {
	actions := make(chan rx.Action)
	ng := &Engine{ctx: rx.NewContext(), actions: actions}

	t.Cleanup(func() { close(actions) })
	go func() {
		for a := range actions {
			ng.ctx = a(ng.ctx)
		}
	}()

	return ng, actions
}

func (ng *Engine) Context() rx.Context {
	synctest.Wait()
	return ng.ctx
}

func (ng *Engine) Do(actions ...rx.Action) {
	for _, a := range actions {
		ng.actions <- a
	}
}
