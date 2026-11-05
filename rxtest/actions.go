package rxtest

import (
	"github.com/TroutSoftware/rx"
)

// Click on a button, or input of type submit.
//
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

// Fill enters v into an input-compatible element.
//
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

// Select from a list of values (typically in a drop-down)
func Select(v string, e Element) rx.Action {
	if e == notFound {
		panic("element not found")
	}

	switch e.rxNode.TagName {
	default:
		return rx.DoNothing
	case "select":
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
