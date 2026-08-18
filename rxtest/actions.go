package rxtest

import (
	"github.com/TroutSoftware/rx"
)

// https://playwright.dev/docs/input
func Click(e Element) rx.Action {
	if e == notFound {
		panic("element not found")
	}

	return findActionFor(e, func(e *rx.Node) rx.IntentType {
		switch {
		default:
			return rx.Click
		case e.TagName == "input" && e.GetAttr("type") == "submit":
			return rx.Submit
		}
	})
}

// https://playwright.dev/docs/input#text-input
func Fill(v string, e Element) rx.Action {
	if e == notFound {
		panic("element not found")
	}

	switch e.rxNode.TagName {
	default:
		return rx.DoNothing
	case "input":
		return func(ctx rx.Context) rx.Context {
			h := findActionFor(e, func(e *rx.Node) rx.IntentType {
				return rx.Change
			})

			return rx.WithValues(ctx, rx.UserInput(v), h)
		}
	}
}

func findActionFor(e Element, intent func(e *rx.Node) rx.IntentType) rx.Action {
	for {
		if act := rx.ActionFor(e.rxNode, intent(e.rxNode)); act != nil {
			return act
		}

		p := e.parent
		if p == nil {
			return rx.DoNothing
		}
		e = *p
	}
}
