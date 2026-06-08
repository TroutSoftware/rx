package rxtest

import "github.com/TroutSoftware/rx"

// https://playwright.dev/docs/input
func Click(ctx rx.Context, e Element) rx.Context {
	for {
		if act := rx.ActionFor(e.rxNode, rx.Click); act != nil {
			return act(ctx)
		}

		p := e.parent
		if p == nil {
			return ctx
		}
		e = *p
	}
}
